package basicserver

import (
	"context"
	"time"

	"github.com/plprobelab/go-kademlia/kad"

	"github.com/plprobelab/go-kademlia/libp2p"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/sim"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type BasicServer[A kad.Address[A]] struct {
	rt       kad.RoutingTable[key.Key256]
	endpoint endpoint.Endpoint[key.Key256, A]

	peerstoreTTL              time.Duration
	numberOfCloserPeersToSend int
}

// var _ server.Server = (*BasicServer)(nil)

func NewBasicServer[A kad.Address[A]](rt kad.RoutingTable[key.Key256], endpoint endpoint.Endpoint[key.Key256, A],
	options ...Option,
) *BasicServer[A] {
	var cfg Config
	if err := cfg.Apply(append([]Option{DefaultConfig}, options...)...); err != nil {
		return nil
	}

	return &BasicServer[A]{
		rt:                        rt,
		endpoint:                  endpoint,
		peerstoreTTL:              cfg.PeerstoreTTL,
		numberOfCloserPeersToSend: cfg.NumberUsefulCloserPeers,
	}
}

func (s *BasicServer[A]) HandleRequest(ctx context.Context, rpeer kad.NodeID[key.Key256],
	msg message.MinKadMessage,
) (message.MinKadMessage, error) {
	switch msg := msg.(type) {
	case *sim.Message[key.Key256, A]:
		return s.HandleFindNodeRequest(ctx, rpeer, msg)
	case *libp2p.Message:
		switch msg.GetType() {
		case libp2p.Message_FIND_NODE:
			return s.HandleFindNodeRequest(ctx, rpeer, msg)
		default:
			return nil, ErrIpfsV1InvalidRequest
		}
	default:
		return nil, ErrUnknownMessageFormat
	}
}

func (s *BasicServer[A]) HandleFindNodeRequest(ctx context.Context,
	rpeer kad.NodeID[key.Key256], msg message.MinKadMessage,
) (message.MinKadMessage, error) {
	var target key.Key256

	switch msg := msg.(type) {
	case *sim.Message[key.Key256, A]:
		target = msg.Target()
	case *libp2p.Message:
		p := peer.ID("")
		if p.UnmarshalBinary(msg.GetKey()) != nil {
			// invalid requested key (not a peer.ID)
			return nil, ErrIpfsV1InvalidPeerID
		}
		t := libp2p.NewPeerID(p)
		target = t.Key()
	default:
		// invalid request, don't reply
		return nil, ErrUnknownMessageFormat
	}

	_, span := util.StartSpan(ctx, "SimServer.HandleFindNodeRequest", trace.WithAttributes(
		attribute.Stringer("Requester", rpeer),
		attribute.String("Target", key.HexString(target))))
	defer span.End()

	peers := s.rt.NearestNodes(target, s.numberOfCloserPeersToSend)

	span.AddEvent("Nearest peers", trace.WithAttributes(
		attribute.Int("count", len(peers)),
	))

	var resp message.MinKadMessage
	switch msg.(type) {
	case *sim.Message[key.Key256, A]:
		peerAddrs := make([]kad.NodeInfo[key.Key256, A], len(peers))
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
		resp = sim.NewResponse(peerAddrs[:index])
	case *libp2p.Message:
		nEndpoint, ok := s.endpoint.(endpoint.NetworkedEndpoint[key.Key256, A])
		if !ok {
			span.RecordError(ErrNotNetworkedEndpoint)
			return nil, ErrNotNetworkedEndpoint
		}
		resp = libp2p.FindPeerResponse(peers, nEndpoint)
	}

	return resp, nil
}
