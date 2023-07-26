package coord

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/util"
)

// A Coordinator coordinates the state machines that comprise a Kademlia DHT
// Currently this is only queries but will expand to include other state machines such as routing table refresh,
// and reproviding.
type Coordinator[K kad.Key[K], A kad.Address[A]] struct {
	clk clock.Clock

	qp *query.QueryPool[K, A]

	// rt is the routing table used to look up nodes by distance
	rt kad.RoutingTable[K]

	// ep is the message endpoint used to send requests
	ep endpoint.Endpoint[K, A]

	peerstoreTTL time.Duration

	notify         chan struct{} // channel to notify there is potentially work to do
	outboundEvents chan KademliaEvent
	inboundEvents  chan coordinatorInternalEvent
	startOnce      sync.Once
}

const DefaultChanqueueCapacity = 1024

type Config struct {
	// TODO: review if this is needed here
	PeerstoreTTL time.Duration // duration for which a peer is kept in the peerstore

	Clock clock.Clock // a clock that may replaced by a mock when testing
}

func DefaultCoordinatorConfig() *Config {
	return &Config{
		Clock:        clock.New(), // use standard time
		PeerstoreTTL: 10 * time.Minute,
	}
}

func NewCoordinator[K kad.Key[K], A kad.Address[A]](ep endpoint.Endpoint[K, A], rt kad.RoutingTable[K], cfg *Config) *Coordinator[K, A] {
	if cfg == nil {
		cfg = DefaultCoordinatorConfig()
	}

	qpCfg := query.DefaultQueryPoolConfig()
	qpCfg.Clock = cfg.Clock

	qp := query.NewQueryPool[K, A](qpCfg)
	return &Coordinator[K, A]{
		clk:            cfg.Clock,
		ep:             ep,
		rt:             rt,
		qp:             qp,
		notify:         make(chan struct{}, 20),
		outboundEvents: make(chan KademliaEvent, 20),
		inboundEvents:  make(chan coordinatorInternalEvent, 20),
	}
}

func (k *Coordinator[K, A]) Start(ctx context.Context) <-chan KademliaEvent {
	ctx, span := util.StartSpan(ctx, "Coordinator.Start")
	defer span.End()
	// ensure there is only ever one mainloop
	k.startOnce.Do(func() {
		go k.mainloop(ctx)
	})
	return k.outboundEvents
}

func (k *Coordinator[K, A]) mainloop(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "Coordinator.mainloop")
	defer span.End()
	for {
		// wait for inbound events to trigger state changes
		select {
		case <-ctx.Done():
			return
		case ev := <-k.inboundEvents:
			switch tev := ev.(type) {
			case *unroutablePeerEvent[K]:
				k.dispatchQueryPoolEvent(ctx, nil)

			case *messageFailedEvent[K]:
				k.dispatchQueryPoolEvent(ctx, nil)

			case *messageResponseEvent[K, A]:
				k.onMessageSuccess(ctx, tev.QueryID, tev.NodeID, tev.Response)
				qev := &query.QueryPoolEventMessageResponse[K, A]{
					QueryID:  tev.QueryID,
					NodeID:   tev.NodeID,
					Response: tev.Response,
				}
				k.dispatchQueryPoolEvent(ctx, qev)
			case *addQueryEvent[K, A]:
				qev := &query.QueryPoolEventAdd[K, A]{
					QueryID:           tev.QueryID,
					Target:            tev.Target,
					ProtocolID:        tev.ProtocolID,
					Message:           tev.Message,
					KnownClosestPeers: tev.KnownClosestPeers,
				}
				k.dispatchQueryPoolEvent(ctx, qev)
			case *stopQueryEvent[K]:
				qev := &query.QueryPoolEventStop[K]{
					QueryID: tev.QueryID,
				}
				k.dispatchQueryPoolEvent(ctx, qev)
			default:
				panic(fmt.Sprintf("unexpected event: %T", tev))
			}
		case <-k.notify:
			// got a hint that there is work to do
			// TODO: decide if this is a hack that can be removed
			k.dispatchQueryPoolEvent(ctx, nil)
		}
	}
}

func (k *Coordinator[K, A]) dispatchQueryPoolEvent(ctx context.Context, ev query.QueryPoolEvent) {
	ctx, span := util.StartSpan(ctx, "Coordinator.dispatchQueryPoolEvent")
	defer span.End()
	// attempt to advance the query state machine
	state := k.qp.Advance(ctx, ev)
	switch st := state.(type) {
	case *query.QueryPoolWaiting:
		// TODO
	case *query.QueryPoolWaitingMessage[K, A]:
		k.attemptSendMessage(ctx, st.ProtocolID, st.NodeID, st.Message, st.QueryID)
	case *query.QueryPoolWaitingWithCapacity:
		// TODO
	case *query.QueryPoolFinished:
		// TODO
	case *query.QueryPoolTimeout:
		// TODO
	case *query.QueryPoolIdle:
		// TODO
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}
}

func (h *Coordinator[K, A]) attemptSendMessage(ctx context.Context, protoID address.ProtocolID, to kad.NodeID[K], msg kad.Request[K, A], queryID query.QueryID) {
	ctx, span := util.StartSpan(ctx, "Coordinator.attemptSendMessage")
	defer span.End()
	go func() {
		resp, err := h.ep.SendMessage(ctx, protoID, to, msg)
		if err != nil {
			if errors.Is(err, endpoint.ErrCannotConnect) {
				// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
				h.inboundEvents <- &unroutablePeerEvent[K]{NodeID: to}
			}
			h.inboundEvents <- &messageFailedEvent[K]{NodeID: to, QueryID: queryID}
			return
		}

		h.inboundEvents <- &messageResponseEvent[K, A]{NodeID: to, QueryID: queryID, Response: resp}
	}()
}

func (k *Coordinator[K, A]) onMessageSuccess(ctx context.Context, queryID query.QueryID, node kad.NodeID[K], resp kad.Response[K, A]) {
	ctx, span := util.StartSpan(ctx, "Coordinator.onMessageSuccess")
	defer span.End()
	// HACK: add closer nodes to peer store
	// TODO: make this an inbound event
	for _, addr := range resp.CloserNodes() {
		k.rt.AddNode(addr.ID())
		k.ep.MaybeAddToPeerstore(ctx, addr, k.peerstoreTTL)
	}

	// notify caller so they have chance to stop query
	k.outboundEvents <- &KademliaOutboundQueryProgressedEvent[K, A]{
		NodeID:   node,
		QueryID:  queryID,
		Response: resp,
	}
}

func (k *Coordinator[K, A]) StartQuery(ctx context.Context, queryID query.QueryID, protocolID address.ProtocolID, msg kad.Request[K, A]) error {
	knownClosestPeers := k.rt.NearestNodes(msg.Target(), 20)

	k.inboundEvents <- &addQueryEvent[K, A]{
		QueryID:           queryID,
		Target:            msg.Target(),
		ProtocolID:        protocolID,
		Message:           msg,
		KnownClosestPeers: knownClosestPeers,
	}

	return nil
}

func (k *Coordinator[K, A]) StopQuery(ctx context.Context, queryID query.QueryID) error {
	k.inboundEvents <- &stopQueryEvent[K]{
		QueryID: queryID,
	}
	return nil
}

// Kademlia events emitted by the Coordinator, intended for consumption by clients of the package

type KademliaEvent interface {
	kademliaEvent()
}

type KademliaRoutingUpdatedEvent[K kad.Key[K]] struct{}

type KademliaOutboundQueryProgressedEvent[K kad.Key[K], A kad.Address[A]] struct {
	NodeID   kad.NodeID[K]
	QueryID  query.QueryID
	Response kad.Response[K, A]
}

type KademliaUnroutablePeerEvent[K kad.Key[K]] struct{}

type KademliaRoutablePeerEvent[K kad.Key[K]] struct{}

// kademliaEvent() ensures that only Kademlia events can be assigned to a KademliaEvent.
func (*KademliaRoutingUpdatedEvent[K]) kademliaEvent()             {}
func (*KademliaOutboundQueryProgressedEvent[K, A]) kademliaEvent() {}
func (*KademliaUnroutablePeerEvent[K]) kademliaEvent()             {}
func (*KademliaRoutablePeerEvent[K]) kademliaEvent()               {}

// Internal events for the Coordiinator

type coordinatorInternalEvent interface {
	coordinatorInternalEvent()
}

// TODO: unexport name and make consistent with other internal events
type unroutablePeerEvent[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

// TODO: unexport name and make consistent with other internal events
type messageFailedEvent[K kad.Key[K]] struct {
	NodeID  kad.NodeID[K]
	QueryID query.QueryID
}

// TODO: unexport name and make consistent with other internal events
type messageResponseEvent[K kad.Key[K], A kad.Address[A]] struct {
	NodeID   kad.NodeID[K]
	QueryID  query.QueryID
	Response kad.Response[K, A]
}

// TODO: unexport name and make consistent with other internal events
type addQueryEvent[K kad.Key[K], A kad.Address[A]] struct {
	QueryID           query.QueryID
	Target            K
	ProtocolID        address.ProtocolID
	Message           kad.Request[K, A]
	KnownClosestPeers []kad.NodeID[K]
}

// TODO: unexport name and make consistent with other internal events
type stopQueryEvent[K kad.Key[K]] struct {
	QueryID query.QueryID
}

// coordinatorInternalEvent() ensures that only an internal coordinator event can be assigned to a coordinatorInternalEvent.
func (*unroutablePeerEvent[K]) coordinatorInternalEvent()     {}
func (*messageFailedEvent[K]) coordinatorInternalEvent()      {}
func (*messageResponseEvent[K, A]) coordinatorInternalEvent() {}
func (*addQueryEvent[K, A]) coordinatorInternalEvent()        {}
func (*stopQueryEvent[K]) coordinatorInternalEvent()          {}
