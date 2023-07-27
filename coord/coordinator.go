package coord

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/kaderr"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/util"
)

// A Coordinator coordinates the state machines that comprise a Kademlia DHT
// Currently this is only queries but will expand to include other state machines such as routing table refresh,
// and reproviding.
type Coordinator[K kad.Key[K], A kad.Address[A]] struct {
	// self is the node id of the system the coordinator is running on
	self kad.NodeID[K]

	// cfg is a copy of the optional configuration supplied to the coordinator
	cfg Config

	qp *query.Pool[K, A]

	// rt is the routing table used to look up nodes by distance
	rt kad.RoutingTable[K]

	// ep is the message endpoint used to send requests
	ep endpoint.Endpoint[K, A]

	peerstoreTTL time.Duration

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

// Validate checks the configuration options and returns an error if any have invalid values.
func (cfg *Config) Validate() error {
	if cfg.Clock == nil {
		return &kaderr.ConfigurationError{
			Component: "CoordinatorConfig",
			Err:       fmt.Errorf("clock must not be nil"),
		}
	}

	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Clock:        clock.New(), // use standard time
		PeerstoreTTL: 10 * time.Minute,
	}
}

func NewCoordinator[K kad.Key[K], A kad.Address[A]](self kad.NodeID[K], ep endpoint.Endpoint[K, A], rt kad.RoutingTable[K], cfg *Config) (*Coordinator[K, A], error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	qpCfg := query.DefaultPoolConfig()
	qpCfg.Clock = cfg.Clock

	qp, err := query.NewPool[K, A](qpCfg)
	if err != nil {
		return nil, fmt.Errorf("query pool: %w", err)
	}
	return &Coordinator[K, A]{
		self:           self,
		cfg:            *cfg,
		ep:             ep,
		rt:             rt,
		qp:             qp,
		outboundEvents: make(chan KademliaEvent, 20),
		inboundEvents:  make(chan coordinatorInternalEvent, 20),
	}, nil
}

func (c *Coordinator[K, A]) Start(ctx context.Context) <-chan KademliaEvent {
	ctx, span := util.StartSpan(ctx, "Coordinator.Start")
	defer span.End()
	// ensure there is only ever one mainloop
	c.startOnce.Do(func() {
		go c.mainloop(ctx)
		go c.heartbeat(ctx)
	})
	return c.outboundEvents
}

func (c *Coordinator[K, A]) mainloop(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "Coordinator.mainloop")
	defer span.End()

	// once the main loop exits no further events will be sent so clients waiting
	// on the event channel should be notified
	defer close(c.outboundEvents)

	for {
		// wait for inbound events to trigger state changes
		select {
		case <-ctx.Done():
			return
		case ev := <-c.inboundEvents:
			switch tev := ev.(type) {
			case *eventUnroutablePeer[K]:
				c.dispatchQueryPoolEvent(ctx, nil)

			case *eventMessageFailed[K]:
				c.dispatchQueryPoolEvent(ctx, nil)

			case *eventMessageResponse[K, A]:
				c.onMessageSuccess(ctx, tev.QueryID, tev.NodeID, tev.Response)
				qev := &query.EventPoolMessageResponse[K, A]{
					QueryID:  tev.QueryID,
					NodeID:   tev.NodeID,
					Response: tev.Response,
				}
				c.dispatchQueryPoolEvent(ctx, qev)
			case *eventAddQuery[K, A]:
				qev := &query.EventPoolAddQuery[K, A]{
					QueryID:           tev.QueryID,
					Target:            tev.Target,
					ProtocolID:        tev.ProtocolID,
					Message:           tev.Message,
					KnownClosestPeers: tev.KnownClosestPeers,
				}
				c.dispatchQueryPoolEvent(ctx, qev)
			case *eventStopQuery[K]:
				qev := &query.EventPoolStopQuery{
					QueryID: tev.QueryID,
				}
				c.dispatchQueryPoolEvent(ctx, qev)
			case *eventPoll:
				c.dispatchQueryPoolEvent(ctx, nil)
			default:
				panic(fmt.Sprintf("unexpected event: %T", tev))
			}
		}
	}
}

func (c *Coordinator[K, A]) heartbeat(ctx context.Context) {
	ticker := c.cfg.Clock.Ticker(5 * time.Millisecond)

	for {
		// wait for inbound events to trigger state changes
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.inboundEvents <- &eventPoll{}
		}
	}
}

func (c *Coordinator[K, A]) dispatchQueryPoolEvent(ctx context.Context, ev query.PoolEvent) {
	ctx, span := util.StartSpan(ctx, "Coordinator.dispatchQueryPoolEvent")
	defer span.End()
	// attempt to advance the query state machine
	state := c.qp.Advance(ctx, ev)
	switch st := state.(type) {
	case *query.StatePoolWaiting:
		// TODO
	case *query.StatePoolQueryMessage[K, A]:
		c.attemptSendMessage(ctx, st.ProtocolID, st.NodeID, st.Message, st.QueryID)
	case *query.StatePoolWaitingAtCapacity:
		// TODO
	case *query.StatePoolWaitingWithCapacity:
		// TODO
	case *query.StatePoolQueryFinished:
		// TODO
	case *query.StatePoolQueryTimeout:
		// TODO
	case *query.StatePoolIdle:
		// TODO
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}
}

func (c *Coordinator[K, A]) attemptSendMessage(ctx context.Context, protoID address.ProtocolID, to kad.NodeID[K], msg kad.Request[K, A], queryID query.QueryID) {
	ctx, span := util.StartSpan(ctx, "Coordinator.attemptSendMessage")
	defer span.End()
	go func() {
		resp, err := c.ep.SendMessage(ctx, protoID, to, msg)
		if err != nil {
			if errors.Is(err, endpoint.ErrCannotConnect) {
				// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
				ev := &eventUnroutablePeer[K]{NodeID: to}
				// c.queue.Enqueue(ctx, ev)
				c.inboundEvents <- ev
				return
			}
			ev := &eventMessageFailed[K]{NodeID: to, QueryID: queryID}
			// c.queue.Enqueue(ctx, ev)
			c.inboundEvents <- ev
			return
		}

		ev := &eventMessageResponse[K, A]{NodeID: to, QueryID: queryID, Response: resp}
		// c.queue.Enqueue(ctx, ev)
		c.inboundEvents <- ev
	}()
}

func (c *Coordinator[K, A]) onMessageSuccess(ctx context.Context, queryID query.QueryID, node kad.NodeID[K], resp kad.Response[K, A]) {
	ctx, span := util.StartSpan(ctx, "Coordinator.onMessageSuccess")
	defer span.End()
	// HACK: add closer nodes to peer store
	// TODO: make this an inbound event
	for _, addr := range resp.CloserNodes() {
		c.rt.AddNode(addr.ID())
		c.ep.MaybeAddToPeerstore(ctx, addr, c.peerstoreTTL)
	}

	// notify caller so they have chance to stop query
	c.outboundEvents <- &KademliaOutboundQueryProgressedEvent[K, A]{
		NodeID:   node,
		QueryID:  queryID,
		Response: resp,
	}
}

func (c *Coordinator[K, A]) StartQuery(ctx context.Context, queryID query.QueryID, protocolID address.ProtocolID, msg kad.Request[K, A]) error {
	knownClosestPeers := c.rt.NearestNodes(msg.Target(), 20)

	ev := &eventAddQuery[K, A]{
		QueryID:           queryID,
		Target:            msg.Target(),
		ProtocolID:        protocolID,
		Message:           msg,
		KnownClosestPeers: knownClosestPeers,
	}

	// c.queue.Enqueue(ctx, ev)
	c.inboundEvents <- ev

	return nil
}

func (c *Coordinator[K, A]) StopQuery(ctx context.Context, queryID query.QueryID) error {
	ev := &eventStopQuery[K]{
		QueryID: queryID,
	}

	// c.queue.Enqueue(ctx, ev)

	c.inboundEvents <- ev
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

type eventUnroutablePeer[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

type eventMessageFailed[K kad.Key[K]] struct {
	NodeID  kad.NodeID[K]
	QueryID query.QueryID
}

type eventMessageResponse[K kad.Key[K], A kad.Address[A]] struct {
	NodeID   kad.NodeID[K]
	QueryID  query.QueryID
	Response kad.Response[K, A]
}

type eventAddQuery[K kad.Key[K], A kad.Address[A]] struct {
	QueryID           query.QueryID
	Target            K
	ProtocolID        address.ProtocolID
	Message           kad.Request[K, A]
	KnownClosestPeers []kad.NodeID[K]
}

type eventStopQuery[K kad.Key[K]] struct {
	QueryID query.QueryID
}

type eventPoll struct{}

// coordinatorInternalEvent() ensures that only an internal coordinator event can be assigned to the coordinatorInternalEvent interface.
func (*eventUnroutablePeer[K]) coordinatorInternalEvent()     {}
func (*eventMessageFailed[K]) coordinatorInternalEvent()      {}
func (*eventMessageResponse[K, A]) coordinatorInternalEvent() {}
func (*eventAddQuery[K, A]) coordinatorInternalEvent()        {}
func (*eventStopQuery[K]) coordinatorInternalEvent()          {}
func (*eventPoll) coordinatorInternalEvent()                  {}
