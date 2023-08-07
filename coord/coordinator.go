package coord

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/kaderr"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/util"
)

// A Coordinator coordinates the state machines that comprise a Kademlia DHT
// Currently this is only queries but will expand to include other state machines such as routing table refresh,
// and reproviding.
type Coordinator[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R kad.Record] struct {
	// self is the node id of the system the coordinator is running on
	self N

	// cfg is a copy of the optional configuration supplied to the coordinator
	cfg Config

	qp *query.Pool[K, N, A, R]

	// rt is the routing table used to look up nodes by distance
	rt kad.RoutingTable[K]

	queryCounter atomic.Uint64
	querySubs    map[query.QueryID]chan<- KademliaEvent

	// ndp is the node discovery protocol
	ndp kad.Protocol[K, N, A, R]

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

func NewCoordinator[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R kad.Record](self N, ndp kad.Protocol[K, N, A, R], rt kad.RoutingTable[K], cfg *Config) (*Coordinator[K, N, A, R], error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	qpCfg := query.DefaultPoolConfig()
	qpCfg.Clock = cfg.Clock

	qp, err := query.NewPool[K, N, A, R](self, qpCfg)
	if err != nil {
		return nil, fmt.Errorf("query pool: %w", err)
	}
	return &Coordinator[K, N, A, R]{
		self:          self,
		cfg:           *cfg,
		rt:            rt,
		qp:            qp,
		ndp:           ndp,
		querySubs:     map[query.QueryID]chan<- KademliaEvent{},
		inboundEvents: make(chan coordinatorInternalEvent, 20),
	}, nil
}

func (c *Coordinator[K, N, A, R]) Start(ctx context.Context) <-chan KademliaEvent {
	ctx, span := util.StartSpan(ctx, "Coordinator.Start")
	defer span.End()
	// ensure there is only ever one mainloop
	c.startOnce.Do(func() {
		go c.mainloop(ctx)
		go c.heartbeat(ctx)
	})
	return c.outboundEvents
}

func (c *Coordinator[K, N, A, R]) mainloop(ctx context.Context) {
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
			case *eventAddQuery[K, N, A, R]:
				qev := &query.EventPoolAddQuery[K, N, A, R]{
					QueryID:  tev.QueryID,
					Target:   tev.Target,
					Protocol: tev.Protocol,
					Seed:     tev.Seed,
				}
				c.querySubs[tev.QueryID] = tev.Out
				c.dispatchQueryPoolEvent(ctx, qev)
			case *eventUnroutablePeer[K]:
				// TODO: remove from routing table
				c.dispatchQueryPoolEvent(ctx, nil)

			case *eventMessageFailed[K]:
				qev := &query.EventPoolMessageFailure[K]{
					QueryID: tev.QueryID,
					NodeID:  tev.NodeID,
					Error:   tev.Error,
				}

				c.dispatchQueryPoolEvent(ctx, qev)

			case *eventMessageResponse[K, N, A, R]:
				if tev.Response != nil {
					for _, info := range tev.Response.CloserNodes() {
						if key.Equal(info.ID().Key(), c.self.Key()) {
							continue
						}
						isNew := c.rt.AddNode(info.ID())
						// c.ep.MaybeAddToPeerstore(ctx, info, c.peerstoreTTL)

						if isNew {
							c.querySubs[tev.QueryID] <- &KademliaRoutingUpdatedEvent[K, N, A]{
								NodeInfo: info,
							}
						}
					}
				}

				// notify caller
				c.querySubs[tev.QueryID] <- &KademliaOutboundQueryProgressedEvent[K, N, A, R]{
					NodeID:   tev.NodeID,
					QueryID:  tev.QueryID,
					Response: tev.Response,
					Stats:    tev.Stats,
				}

				qev := &query.EventPoolMessageResponse[K, N, A, R]{
					QueryID:  tev.QueryID,
					NodeID:   tev.NodeID,
					Response: tev.Response,
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

func (c *Coordinator[K, N, A, R]) heartbeat(ctx context.Context) {
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

func (c *Coordinator[K, N, A, R]) dispatchQueryPoolEvent(ctx context.Context, ev query.PoolEvent) {
	ctx, span := util.StartSpan(ctx, "Coordinator.dispatchQueryPoolEvent")
	defer span.End()
	// attempt to advance the query state machine
	state := c.qp.Advance(ctx, ev)
	switch st := state.(type) {
	case *query.StatePoolQueryMessage[K, N, A, R]:
		c.attemptSendMessage(ctx, st.Protocol, st.NodeID, st.Target, st.QueryID, st.Stats)
	case *query.StatePoolWaitingAtCapacity:
		// TODO
	case *query.StatePoolWaitingWithCapacity:
		// TODO
	case *query.StatePoolQueryFinished[K, N, A, R]:
		c.querySubs[st.QueryID] <- &KademliaOutboundQueryFinishedEvent[K, N, A, R]{
			QueryID: st.QueryID,
			Stats:   st.Stats,
			Records: st.Records,
		}
		delete(c.querySubs, st.QueryID)
		// TODO
	case *query.StatePoolQueryTimeout:
		// TODO
	case *query.StatePoolIdle:
		// TODO
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}
}

func (c *Coordinator[K, N, A, R]) attemptSendMessage(ctx context.Context, protocol kad.Protocol[K, N, A, R], to N, target K, queryID query.QueryID, stats query.QueryStats) {
	ctx, span := util.StartSpan(ctx, "Coordinator.attemptSendMessage")
	defer span.End()
	go func() {
		resp, err := protocol.Get(ctx, to, target)
		if err != nil {
			if errors.Is(err, endpoint.ErrCannotConnect) {
				// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
				c.inboundEvents <- &eventUnroutablePeer[K]{
					NodeID: to,
				}
				return
			}
			c.inboundEvents <- &eventMessageFailed[K]{
				NodeID:  to,
				QueryID: queryID,
				Stats:   stats,
				Error:   err,
			}
			return
		}

		c.inboundEvents <- &eventMessageResponse[K, N, A, R]{
			NodeID:   to,
			QueryID:  queryID,
			Response: resp,
			Stats:    stats,
		}
	}()
}

func (c *Coordinator[K, N, A, R]) StopQuery(ctx context.Context, queryID query.QueryID) error {
	ev := &eventStopQuery[K]{
		QueryID: queryID,
	}

	c.inboundEvents <- ev
	return nil
}

// Kademlia events emitted by the Coordinator, intended for consumption by clients of the package

type KademliaEvent interface {
	kademliaEvent()
}

// KademliaOutboundQueryProgressedEvent is emitted by the coordinator when a query has received a
// response from a node.
type KademliaOutboundQueryProgressedEvent[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R kad.Record] struct {
	QueryID  query.QueryID
	NodeID   kad.NodeID[K]
	Response kad.Response[K, N, A, R]
	Stats    query.QueryStats
}

// KademliaOutboundQueryFinishedEvent is emitted by the coordinator when a query has finished, either through
// running to completion or by being canceled.
type KademliaOutboundQueryFinishedEvent[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R kad.Record] struct {
	QueryID query.QueryID
	Stats   query.QueryStats
	Records []R
}

// KademliaRoutingUpdatedEvent is emitted by the coordinator when a new node has been added to the routing table.
type KademliaRoutingUpdatedEvent[K kad.Key[K], N kad.NodeID[K], A kad.Address[A]] struct {
	NodeInfo kad.NodeInfo[K, N, A]
}

type KademliaUnroutablePeerEvent[K kad.Key[K]] struct{}

type KademliaRoutablePeerEvent[K kad.Key[K]] struct{}

// kademliaEvent() ensures that only Kademlia events can be assigned to a KademliaEvent.
func (*KademliaRoutingUpdatedEvent[K, N, A]) kademliaEvent()             {}
func (*KademliaOutboundQueryProgressedEvent[K, N, A, R]) kademliaEvent() {}
func (*KademliaUnroutablePeerEvent[K]) kademliaEvent()                   {}
func (*KademliaRoutablePeerEvent[K]) kademliaEvent()                     {}
func (*KademliaOutboundQueryFinishedEvent[K, N, A, R]) kademliaEvent()   {}

// Internal events for the Coordiinator

type coordinatorInternalEvent interface {
	coordinatorInternalEvent()
}

type eventUnroutablePeer[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

type eventMessageFailed[K kad.Key[K]] struct {
	NodeID  kad.NodeID[K]    // the node the message was sent to
	QueryID query.QueryID    // the id of the query that sent the message
	Stats   query.QueryStats // stats for the query sending the message
	Error   error            // the error that caused the failure, if any
}

type eventMessageResponse[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R kad.Record] struct {
	NodeID   kad.NodeID[K]            // the node the message was sent to
	QueryID  query.QueryID            // the id of the query that sent the message
	Response kad.Response[K, N, A, R] // the message response sent by the node
	Stats    query.QueryStats         // stats for the query sending the message
}

type eventAddQuery[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R kad.Record] struct {
	QueryID  query.QueryID
	Target   K
	Protocol kad.Protocol[K, N, A, R]
	Seed     []N
	Out      chan<- KademliaEvent
}

type eventStopQuery[K kad.Key[K]] struct {
	QueryID query.QueryID
}

type eventPoll struct{}

// coordinatorInternalEvent() ensures that only an internal coordinator event can be assigned to the coordinatorInternalEvent interface.
func (*eventUnroutablePeer[K]) coordinatorInternalEvent()           {}
func (*eventMessageFailed[K]) coordinatorInternalEvent()            {}
func (*eventMessageResponse[K, N, A, R]) coordinatorInternalEvent() {}
func (*eventAddQuery[K, N, A, R]) coordinatorInternalEvent()        {}
func (*eventStopQuery[K]) coordinatorInternalEvent()                {}
func (*eventPoll) coordinatorInternalEvent()                        {}

func (c *Coordinator[K, N, A, R]) FindNode(ctx context.Context, node N) (*R, error) {
	evts := make(chan KademliaEvent)

	var seed []N
	for _, nn := range c.rt.NearestNodes(node.Key(), 20) {
		seed = append(seed, nn.(N)) // TODO: bad
	}

	ev := &eventAddQuery[K, N, A, R]{
		QueryID:  query.QueryID(c.queryCounter.Add(1)),
		Target:   node.Key(),
		Protocol: c.ndp,
		Seed:     seed,
		Out:      evts,
	}

	// c.queue.Enqueue(ctx, ev)
	c.inboundEvents <- ev

	for {
		select {
		case <-ctx.Done():
			c.StopQuery(ctx, ev.QueryID)
			return nil, ctx.Err()
		case evt, ok := <-evts:
			if !ok {
				return nil, fmt.Errorf("query was stopped unexpectedly")
			}
			switch evt := evt.(type) {
			case *KademliaOutboundQueryProgressedEvent[K, N, A, R]:
				// query progressed
			case *KademliaOutboundQueryFinishedEvent[K, N, A, R]:
				if len(evt.Records) > 0 {
					return &evt.Records[0], nil
				}
				return nil, fmt.Errorf("no result")
			}
		}
	}
}

type RecordCoordinator[K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R1 kad.Record, R2 kad.Record] struct {
	*Coordinator[K, N, A, R1]
	protocol kad.Protocol[K, N, A, R2]
}

func Specialize[R2 kad.Record, K kad.Key[K], N kad.NodeID[K], A kad.Address[A], R1 kad.Record](c *Coordinator[K, N, A, R1], p kad.Protocol[K, N, A, R2]) RecordCoordinator[K, N, A, R1, R2] {
	return RecordCoordinator[K, N, A, R1, R2]{
		Coordinator: c,
		protocol:    p,
	}
}

func (r RecordCoordinator[K, N, A, R1, R2]) GetRecords(ctx context.Context, key K) ([]R2, error) {
	evts := make(chan KademliaEvent)

	var seed []N
	for _, nn := range r.rt.NearestNodes(key, 20) {
		seed = append(seed, nn.(N)) // TODO: bad
	}

	ev := &eventAddQuery[K, N, A, R2]{
		QueryID:  query.QueryID(r.queryCounter.Add(1)),
		Target:   key,
		Protocol: r.protocol,
		Seed:     seed,
		Out:      evts,
	}

	// c.queue.Enqueue(ctx, ev)
	r.inboundEvents <- ev

	for {
		select {
		case <-ctx.Done():
			r.StopQuery(ctx, ev.QueryID)
			return nil, ctx.Err()
		case evt, ok := <-evts:
			if !ok {
				return nil, fmt.Errorf("query was stopped unexpectedly")
			}
			switch evt := evt.(type) {
			case *KademliaOutboundQueryProgressedEvent[K, N, A, R2]:
				// query progressed
			case *KademliaOutboundQueryFinishedEvent[K, N, A, R2]:
				return evt.Records, nil
			}
		}
	}
	return nil, nil
}

func (r RecordCoordinator[K, N, A, R1, R2]) PutRecord(ctx context.Context, to N, record R2) error {
	// uses r.Coordinator to find closest nodes to record key
	return nil
}
