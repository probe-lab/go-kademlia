package simserver

import (
	"context"
	"time"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routing"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type SimServer[K kad.Key[K]] struct {
	rt       routing.Table[K]
	endpoint endpoint.Endpoint[K]

	peerstoreTTL              time.Duration
	numberOfCloserPeersToSend int
}

func NewSimServer[K kad.Key[K]](rt routing.Table[K], endpoint endpoint.Endpoint[K], cfg *Config) *SimServer[K] {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &SimServer[K]{
		rt:                        rt,
		endpoint:                  endpoint,
		peerstoreTTL:              cfg.PeerstoreTTL,
		numberOfCloserPeersToSend: cfg.NumberUsefulCloserPeers,
	}
}

func (s *SimServer[K]) HandleRequest(ctx context.Context, rpeer address.NodeID[K],
	msg message.MinKadMessage,
) (message.MinKadMessage, error) {
	switch msg := msg.(type) {
	case *simmessage.SimMessage[K]:
		return s.HandleFindNodeRequest(ctx, rpeer, msg)
	default:
		return nil, ErrUnknownMessageFormat
	}
}

func (s *SimServer[K]) HandleFindNodeRequest(ctx context.Context,
	rpeer address.NodeID[K], msg message.MinKadMessage,
) (message.MinKadMessage, error) {
	var target K

	switch msg := msg.(type) {
	case *simmessage.SimMessage[K]:
		target = msg.Target()
	default:
		// invalid request, don't reply
		return nil, ErrUnknownMessageFormat
	}

	_, span := util.StartSpan(ctx, "SimServer.HandleFindNodeRequest", trace.WithAttributes(
		attribute.Stringer("Requester", rpeer),
		attribute.String("Target", key.HexString(target))))
	defer span.End()

	peers, err := s.rt.NearestPeers(ctx, target, s.numberOfCloserPeersToSend)
	if err != nil {
		span.RecordError(err)
		// invalid request, don't reply
		return nil, err
	}
	span.AddEvent("Nearest peers", trace.WithAttributes(
		attribute.Int("count", len(peers)),
	))

	var resp message.MinKadMessage
	switch msg.(type) {
	case *simmessage.SimMessage[K]:
		peerAddrs := make([]address.NodeAddr[K], len(peers))
		var index int
		for _, p := range peers {
			na, err := s.endpoint.NetworkAddress(p)
			if err != nil {
				span.RecordError(err)
				continue
			}
			peerAddrs[index] = na
			index++
		}
		resp = simmessage.NewSimResponse(peerAddrs[:index])
	}

	return resp, nil
}

type Config struct {
	PeerstoreTTL            time.Duration
	NumberUsefulCloserPeers int
}

func DefaultConfig() *Config {
	return &Config{
		PeerstoreTTL:            time.Second,
		NumberUsefulCloserPeers: 4,
	}
}
