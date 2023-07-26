package sim

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/util"
)

type Server[K kad.Key[K], A kad.Address[A]] struct {
	rt       kad.RoutingTable[K]
	endpoint endpoint.Endpoint[K, A]

	peerstoreTTL              time.Duration
	numberOfCloserPeersToSend int
}

func NewServer[K kad.Key[K], A kad.Address[A]](rt kad.RoutingTable[K], endpoint endpoint.Endpoint[K, A], cfg *ServerConfig) *Server[K, A] {
	if cfg == nil {
		cfg = DefaultServerConfig()
	}
	return &Server[K, A]{
		rt:                        rt,
		endpoint:                  endpoint,
		peerstoreTTL:              cfg.PeerstoreTTL,
		numberOfCloserPeersToSend: cfg.NumberUsefulCloserPeers,
	}
}

func (s *Server[K, A]) HandleRequest(ctx context.Context, rpeer kad.NodeID[K],
	msg kad.Message,
) (kad.Message, error) {
	switch msg := msg.(type) {
	case *Message[K, A]:
		return s.HandleFindNodeRequest(ctx, rpeer, msg)
	default:
		return nil, ErrUnknownMessageFormat
	}
}

func (s *Server[K, A]) HandleFindNodeRequest(ctx context.Context,
	rpeer kad.NodeID[K], msg kad.Message,
) (kad.Message, error) {
	var target K

	switch msg := msg.(type) {
	case *Message[K, A]:
		target = msg.Target()
	default:
		// invalid request, don't reply
		return nil, ErrUnknownMessageFormat
	}

	_, span := util.StartSpan(ctx, "Server.HandleFindNodeRequest", trace.WithAttributes(
		attribute.Stringer("Requester", rpeer),
		attribute.String("Target", key.HexString(target))))
	defer span.End()

	nodes := s.rt.NearestNodes(target, s.numberOfCloserPeersToSend)
	span.AddEvent("Nearest nodes", trace.WithAttributes(
		attribute.Int("count", len(nodes)),
	))

	var resp kad.Message
	switch msg.(type) {
	case *Message[K, A]:
		peerAddrs := make([]kad.NodeInfo[K, A], len(nodes))
		var index int
		for _, p := range nodes {
			na, err := s.endpoint.NetworkAddress(p)
			if err != nil {
				span.RecordError(err)
				continue
			}
			peerAddrs[index] = na
			index++
		}
		resp = NewResponse(peerAddrs[:index])
	}

	return resp, nil
}

type ServerConfig struct {
	PeerstoreTTL            time.Duration
	NumberUsefulCloserPeers int
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		PeerstoreTTL:            time.Second,
		NumberUsefulCloserPeers: 4,
	}
}
