package basicserver

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/ipfsv1"
	"github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routing"
	"github.com/plprobelab/go-kademlia/server"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type BasicServer struct {
	rt       routing.Table
	endpoint endpoint.Endpoint

	peerstoreTTL              time.Duration
	numberOfCloserPeersToSend int
}

var _ server.Server = (*BasicServer)(nil)

func NewBasicServer(rt routing.Table, endpoint endpoint.Endpoint,
	options ...Option,
) *BasicServer {
	var cfg Config
	if err := cfg.Apply(append([]Option{DefaultConfig}, options...)...); err != nil {
		return nil
	}

	return &BasicServer{
		rt:                        rt,
		endpoint:                  endpoint,
		peerstoreTTL:              cfg.PeerstoreTTL,
		numberOfCloserPeersToSend: cfg.NumberUsefulCloserPeers,
	}
}

func (s *BasicServer) HandleRequest(ctx context.Context, rpeer address.NodeID,
	msg message.MinKadMessage,
) (message.MinKadMessage, error) {
	switch msg := msg.(type) {
	case *simmessage.SimMessage:
		return s.HandleFindNodeRequest(ctx, rpeer, msg)
	case *ipfsv1.Message:
		switch msg.GetType() {
		case ipfsv1.Message_FIND_NODE:
			return s.HandleFindNodeRequest(ctx, rpeer, msg)
		default:
			return nil, ErrIpfsV1InvalidRequest
		}
	default:
		return nil, ErrUnknownMessageFormat
	}
}

func (s *BasicServer) HandleFindNodeRequest(ctx context.Context,
	rpeer address.NodeID, msg message.MinKadMessage,
) (message.MinKadMessage, error) {
	var target key.KadKey

	switch msg := msg.(type) {
	case *simmessage.SimMessage:
		target = msg.Target()
		if target == nil {
			// invalid request (nil target), don't reply
			return nil, ErrSimMessageNilTarget
		}
	case *ipfsv1.Message:
		p := peer.ID("")
		if p.UnmarshalBinary(msg.GetKey()) != nil {
			// invalid requested key (not a peer.ID)
			return nil, ErrIpfsV1InvalidPeerID
		}
		t := peerid.NewPeerID(p)
		target = t.Key()
	default:
		// invalid request, don't reply
		return nil, ErrUnknownMessageFormat
	}

	_, span := util.StartSpan(ctx, "SimServer.HandleFindNodeRequest", trace.WithAttributes(
		attribute.Stringer("Requester", rpeer),
		attribute.Stringer("Target", target)))
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
	case *simmessage.SimMessage:
		peerAddrs := make([]address.NodeAddr, len(peers))
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
	case *ipfsv1.Message:
		nEndpoint, ok := s.endpoint.(endpoint.NetworkedEndpoint)
		if !ok {
			span.RecordError(ErrNotNetworkedEndpoint)
			return nil, ErrNotNetworkedEndpoint
		}
		resp = ipfsv1.FindPeerResponse(peers, nEndpoint)
	}

	return resp, nil
}
