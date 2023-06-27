package basicserver

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/peerid"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p-kad-dht/network/message/ipfsv1"
	"github.com/libp2p/go-libp2p-kad-dht/network/message/simmessage"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable"
	"github.com/libp2p/go-libp2p-kad-dht/server"
	"github.com/libp2p/go-libp2p-kad-dht/util"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type BasicServer struct {
	rt       routingtable.RoutingTable
	endpoint endpoint.Endpoint

	peerstoreTTL              time.Duration
	numberOfCloserPeersToSend int
}

var _ server.Server = (*BasicServer)(nil)

func NewBasicServer(rt routingtable.RoutingTable, endpoint endpoint.Endpoint,
	options ...Option) *BasicServer {
	var cfg Config
	if err := cfg.Apply(append([]Option{DefaultConfig}, options...)...); err != nil {
		return nil
	}

	return &BasicServer{
		rt:                        rt,
		endpoint:                  endpoint,
		peerstoreTTL:              cfg.PeerstoreTTL,
		numberOfCloserPeersToSend: cfg.NumberOfCloserPeersToSend,
	}
}

func (s *BasicServer) HandleRequest(ctx context.Context, rpeer address.NodeID,
	msg message.MinKadMessage) (message.MinKadMessage, error) {

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
	rpeer address.NodeID, msg message.MinKadMessage) (message.MinKadMessage, error) {

	var target key.KadKey

	switch msg := msg.(type) {
	case *simmessage.SimMessage:
		t := msg.Target()
		if t == nil {
			// invalid request (nil target), don't reply
			return nil, ErrSimMessageNilTarget
		}
		target = *t
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

	s.endpoint.MaybeAddToPeerstore(ctx, rpeer, s.peerstoreTTL)

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
		resp = simmessage.NewSimResponse(peers)
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
