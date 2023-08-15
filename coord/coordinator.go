package coord

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/event"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/kaderr"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/routing"
	"github.com/plprobelab/go-kademlia/util"
)

// A StateMachine progresses through a set of states in response to transition events.
type StateMachine[S any, E any] interface {
	// Advance advances the state of the state machine.
	Advance(context.Context, E) S
}

// eventQueue is a bounded, typed queue for events
// NOTE: this type is incompatible with the semantics of event.Queue which blocks on Dequeue
type eventQueue[E any] struct {
	events chan E
}

func newEventQueue[E any](capacity int) *eventQueue[E] {
	return &eventQueue[E]{
		events: make(chan E, capacity),
	}
}

// Enqueue adds an event to the queue. It blocks if the queue is at capacity.
func (q *eventQueue[E]) Enqueue(ctx context.Context, e E) {
	q.events <- e
}

// Dequeue reads an event from the queue. It returns the event and a true value
// if an event was read or the zero value of the event type and false if no event
// was read. This method is non-blocking.
func (q *eventQueue[E]) Dequeue(ctx context.Context) (E, bool) {
	select {
	case e := <-q.events:
		return e, true
	default:
		var v E
		return v, false
	}
}

// FindNodeRequestFunc is a function that creates a request to find the supplied node id
// TODO: consider this being a first class method of the Endpoint
type FindNodeRequestFunc[K kad.Key[K], A kad.Address[A]] func(kad.NodeID[K]) (address.ProtocolID, kad.Request[K, A])

// A Coordinator coordinates the state machines that comprise a Kademlia DHT
// Currently this is only queries and bootstrapping but will expand to include other state machines such as
// routing table refresh, and reproviding.
type Coordinator[K kad.Key[K], A kad.Address[A]] struct {
	// self is the node id of the system the coordinator is running on
	self kad.NodeID[K]

	// cfg is a copy of the optional configuration supplied to the coordinator
	cfg Config

	// pool is the query pool state machine, responsible for running user-submitted queries
	pool StateMachine[query.PoolState, query.PoolEvent]

	// poolEvents is a fifo queue of events that are to be processed by the pool state machine
	poolEvents *eventQueue[query.PoolEvent]

	// bootstrap is the bootstrap state machine, responsible for bootstrapping the routing table
	bootstrap StateMachine[routing.BootstrapState, routing.BootstrapEvent]

	// bootstrapEvents is a fifo queue of events that are to be processed by the bootstrap state machine
	bootstrapEvents *eventQueue[routing.BootstrapEvent]

	// include is the include state machine, responsible for including candidate nodes into the routing table
	include StateMachine[routing.IncludeState, routing.IncludeEvent]

	// includeEvents is a fifo queue of events that are to be processed by the include state machine
	includeEvents *eventQueue[routing.IncludeEvent]

	// rt is the routing table used to look up nodes by distance
	rt kad.RoutingTable[K, kad.NodeID[K]]

	// ep is the message endpoint used to send requests
	ep endpoint.Endpoint[K, A]

	// findNodeFn is a function that creates a find node request that may be understod by the endpoint
	// TODO: thiis should be a function of the endpoint
	findNodeFn FindNodeRequestFunc[K, A]

	// queue not used
	queue event.EventQueue

	// planner not used
	planner event.AwareActionPlanner

	outboundEvents chan KademliaEvent
}

const DefaultChanqueueCapacity = 1024

type Config struct {
	PeerstoreTTL time.Duration // duration for which a peer is kept in the peerstore

	Clock clock.Clock // a clock that may replaced by a mock when testing

	QueryConcurrency int           // the maximum number of queries that may be waiting for message responses at any one time
	QueryTimeout     time.Duration // the time to wait before terminating a query that is not making progress

	RequestConcurrency int           // the maximum number of concurrent requests that each query may have in flight
	RequestTimeout     time.Duration // the timeout queries should use for contacting a single node
}

// Validate checks the configuration options and returns an error if any have invalid values.
func (cfg *Config) Validate() error {
	if cfg.Clock == nil {
		return &kaderr.ConfigurationError{
			Component: "CoordinatorConfig",
			Err:       fmt.Errorf("clock must not be nil"),
		}
	}

	if cfg.QueryConcurrency < 1 {
		return &kaderr.ConfigurationError{
			Component: "CoordinatorConfig",
			Err:       fmt.Errorf("query concurrency must be greater than zero"),
		}
	}
	if cfg.QueryTimeout < 1 {
		return &kaderr.ConfigurationError{
			Component: "CoordinatorConfig",
			Err:       fmt.Errorf("query timeout must be greater than zero"),
		}
	}

	if cfg.RequestConcurrency < 1 {
		return &kaderr.ConfigurationError{
			Component: "CoordinatorConfig",
			Err:       fmt.Errorf("request concurrency must be greater than zero"),
		}
	}

	if cfg.RequestTimeout < 1 {
		return &kaderr.ConfigurationError{
			Component: "CoordinatorConfig",
			Err:       fmt.Errorf("request timeout must be greater than zero"),
		}
	}
	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Clock:              clock.New(), // use standard time
		PeerstoreTTL:       10 * time.Minute,
		QueryConcurrency:   3,
		QueryTimeout:       5 * time.Minute,
		RequestConcurrency: 3,
		RequestTimeout:     time.Minute,
	}
}

func NewCoordinator[K kad.Key[K], A kad.Address[A]](self kad.NodeID[K], ep endpoint.Endpoint[K, A], fn FindNodeRequestFunc[K, A], rt kad.RoutingTable[K, kad.NodeID[K]], cfg *Config) (*Coordinator[K, A], error) {
	if cfg == nil {
		cfg = DefaultConfig()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	qpCfg := query.DefaultPoolConfig()
	qpCfg.Clock = cfg.Clock
	qpCfg.Concurrency = cfg.QueryConcurrency
	qpCfg.Timeout = cfg.QueryTimeout
	qpCfg.QueryConcurrency = cfg.RequestConcurrency
	qpCfg.RequestTimeout = cfg.RequestTimeout

	qp, err := query.NewPool[K, A](self, qpCfg)
	if err != nil {
		return nil, fmt.Errorf("query pool: %w", err)
	}

	bootstrapCfg := routing.DefaultBootstrapConfig[K, A]()
	bootstrapCfg.Clock = cfg.Clock
	bootstrapCfg.Timeout = cfg.QueryTimeout
	bootstrapCfg.RequestConcurrency = cfg.RequestConcurrency
	bootstrapCfg.RequestTimeout = cfg.RequestTimeout

	bootstrap, err := routing.NewBootstrap(self, bootstrapCfg)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: %w", err)
	}

	includeCfg := routing.DefaultIncludeConfig()
	includeCfg.Clock = cfg.Clock
	includeCfg.Timeout = cfg.QueryTimeout

	// TODO: expose config
	// includeCfg.QueueCapacity = cfg.IncludeQueueCapacity
	// includeCfg.Concurrency = cfg.IncludeConcurrency
	// includeCfg.Timeout = cfg.IncludeTimeout

	include, err := routing.NewInclude[K, A](rt, includeCfg)
	if err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	return &Coordinator[K, A]{
		self:            self,
		cfg:             *cfg,
		ep:              ep,
		findNodeFn:      fn,
		rt:              rt,
		pool:            qp,
		poolEvents:      newEventQueue[query.PoolEvent](20), // 20 is abitrary, move to config
		bootstrap:       bootstrap,
		bootstrapEvents: newEventQueue[routing.BootstrapEvent](20), // 20 is abitrary, move to config
		include:         include,
		includeEvents:   newEventQueue[routing.IncludeEvent](20), // 20 is abitrary, move to config
		outboundEvents:  make(chan KademliaEvent, 20),
		queue:           event.NewChanQueue(DefaultChanqueueCapacity),
		planner:         event.NewSimplePlanner(cfg.Clock),
	}, nil
}

func (c *Coordinator[K, A]) Events() <-chan KademliaEvent {
	return c.outboundEvents
}

func (c *Coordinator[K, A]) RunOne(ctx context.Context) bool {
	ctx, span := util.StartSpan(ctx, "Coordinator.RunOne")
	defer span.End()

	// Process state machines in priority order

	// Give the bootstrap state machine priority
	if c.advanceBootstrap(ctx) {
		return true
	}

	// Attempt to advance the include state machine so candidate nodes
	// are added to the routing table
	if c.advanceInclude(ctx) {
		return true
	}

	// Attempt to advance an outbound query
	if c.advancePool(ctx) {
		return true
	}

	return false
}

func (c *Coordinator[K, A]) advanceBootstrap(ctx context.Context) bool {
	bev, ok := c.bootstrapEvents.Dequeue(ctx)
	if !ok {
		bev = &routing.EventBootstrapPoll{}
	}

	bstate := c.bootstrap.Advance(ctx, bev)
	switch st := bstate.(type) {
	case *routing.StateBootstrapMessage[K, A]:
		c.sendBootstrapFindNode(ctx, st.NodeID, st.QueryID, st.Stats)
		return true

	case *routing.StateBootstrapWaiting:
		// bootstrap waiting for a message response, proceed with other state machines
		return false

	case *routing.StateBootstrapFinished:
		c.outboundEvents <- &KademliaBootstrapFinishedEvent{
			Stats: st.Stats,
		}
		return true

	case *routing.StateBootstrapIdle:
		// bootstrap not running, can proceed to other state machines
		return false
	default:
		panic(fmt.Sprintf("unexpected bootstrap state: %T", st))
	}
}

func (c *Coordinator[K, A]) advanceInclude(ctx context.Context) bool {
	// Attempt to advance the include state machine so candidate nodes
	// are added to the routing table
	iev, ok := c.includeEvents.Dequeue(ctx)
	if !ok {
		iev = &routing.EventIncludePoll{}
	}
	istate := c.include.Advance(ctx, iev)
	switch st := istate.(type) {
	case *routing.StateIncludeFindNodeMessage[K, A]:
		// include wants to send a find node message to a node
		c.sendIncludeFindNode(ctx, st.NodeInfo)
		return true
	case *routing.StateIncludeRoutingUpdated[K, A]:
		// a node has been included in the routing table
		c.outboundEvents <- &KademliaRoutingUpdatedEvent[K, A]{
			NodeInfo: st.NodeInfo,
		}
		return true
	case *routing.StateIncludeWaitingAtCapacity:
		// nothing to do except wait for message response or timeout
		return false
	case *routing.StateIncludeWaitingWithCapacity:
		// nothing to do except wait for message response or timeout
		return false
	case *routing.StateIncludeWaitingFull:
		// nothing to do except wait for message response or timeout
		return false
	case *routing.StateIncludeIdle:
		// nothing to do except wait for message response or timeout
		return false
	default:
		panic(fmt.Sprintf("unexpected include state: %T", st))
	}
}

func (c *Coordinator[K, A]) advancePool(ctx context.Context) bool {
	pev, ok := c.poolEvents.Dequeue(ctx)
	if !ok {
		pev = &query.EventPoolPoll{}
	}

	state := c.pool.Advance(ctx, pev)
	switch st := state.(type) {
	case *query.StatePoolQueryMessage[K, A]:
		c.sendQueryMessage(ctx, st.ProtocolID, st.NodeID, st.Message, st.QueryID, st.Stats)
		return true
	case *query.StatePoolWaitingAtCapacity:
		// nothing to do except wait for message response or timeout
		return false
	case *query.StatePoolWaitingWithCapacity:
		// nothing to do except wait for message response or timeout
		return false
	case *query.StatePoolQueryFinished:
		c.outboundEvents <- &KademliaOutboundQueryFinishedEvent{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}
		return true
	case *query.StatePoolQueryTimeout:
		// TODO
		return false
	case *query.StatePoolIdle:
		// nothing to do
		return false
	default:
		panic(fmt.Sprintf("unexpected pool state: %T", st))
	}
}

func (c *Coordinator[K, A]) sendQueryMessage(ctx context.Context, protoID address.ProtocolID, to kad.NodeID[K], msg kad.Request[K, A], queryID query.QueryID, stats query.QueryStats) {
	ctx, span := util.StartSpan(ctx, "Coordinator.sendQueryMessage")
	defer span.End()

	onSendError := func(ctx context.Context, err error) {
		if errors.Is(err, endpoint.ErrCannotConnect) {
			// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
			// TODO: remove from routing table
			return
		}

		qev := &query.EventPoolMessageFailure[K]{
			NodeID:  to,
			QueryID: queryID,
			Error:   err,
		}
		c.poolEvents.Enqueue(ctx, qev)
	}

	onMessageResponse := func(ctx context.Context, resp kad.Response[K, A], err error) {
		if err != nil {
			onSendError(ctx, err)
			return
		}

		if resp != nil {
			candidates := resp.CloserNodes()
			if len(candidates) > 0 {
				// ignore error here
				c.AddNodes(ctx, candidates)
			}
		}

		// notify caller so they have chance to stop query
		c.outboundEvents <- &KademliaOutboundQueryProgressedEvent[K, A]{
			NodeID:   to,
			QueryID:  queryID,
			Response: resp,
			Stats:    stats,
		}

		qev := &query.EventPoolMessageResponse[K, A]{
			NodeID:   to,
			QueryID:  queryID,
			Response: resp,
		}
		c.poolEvents.Enqueue(ctx, qev)
	}

	err := c.ep.SendRequestHandleResponse(ctx, protoID, to, msg, msg.EmptyResponse(), 0, onMessageResponse)
	if err != nil {
		onSendError(ctx, err)
	}
}

func (c *Coordinator[K, A]) sendBootstrapFindNode(ctx context.Context, to kad.NodeID[K], queryID query.QueryID, stats query.QueryStats) {
	ctx, span := util.StartSpan(ctx, "Coordinator.sendBootstrapFindNode")
	defer span.End()

	onSendError := func(ctx context.Context, err error) {
		if errors.Is(err, endpoint.ErrCannotConnect) {
			// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
			// TODO: remove from routing table
			return
		}

		bev := &routing.EventBootstrapMessageFailure[K]{
			NodeID: to,
			Error:  err,
		}
		c.bootstrapEvents.Enqueue(ctx, bev)
	}

	onMessageResponse := func(ctx context.Context, resp kad.Response[K, A], err error) {
		if err != nil {
			onSendError(ctx, err)
			return
		}

		if resp != nil {
			candidates := resp.CloserNodes()
			if len(candidates) > 0 {
				// ignore error here
				c.AddNodes(ctx, candidates)
			}
		}

		// notify caller so they have chance to stop query
		c.outboundEvents <- &KademliaOutboundQueryProgressedEvent[K, A]{
			NodeID:   to,
			QueryID:  queryID,
			Response: resp,
			Stats:    stats,
		}

		bev := &routing.EventBootstrapMessageResponse[K, A]{
			NodeID:   to,
			Response: resp,
		}
		c.bootstrapEvents.Enqueue(ctx, bev)
	}

	protoID, msg := c.findNodeFn(c.self)
	err := c.ep.SendRequestHandleResponse(ctx, protoID, to, msg, msg.EmptyResponse(), 0, onMessageResponse)
	if err != nil {
		onSendError(ctx, err)
	}
}

func (c *Coordinator[K, A]) sendIncludeFindNode(ctx context.Context, to kad.NodeInfo[K, A]) {
	ctx, span := util.StartSpan(ctx, "Coordinator.sendIncludeFindNode")
	defer span.End()

	onSendError := func(ctx context.Context, err error) {
		if errors.Is(err, endpoint.ErrCannotConnect) {
			// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
			// TODO: remove from routing table
			return
		}

		iev := &routing.EventIncludeMessageFailure[K, A]{
			NodeInfo: to,
			Error:    err,
		}
		c.includeEvents.Enqueue(ctx, iev)
	}

	onMessageResponse := func(ctx context.Context, resp kad.Response[K, A], err error) {
		if err != nil {
			onSendError(ctx, err)
			return
		}

		iev := &routing.EventIncludeMessageResponse[K, A]{
			NodeInfo: to,
			Response: resp,
		}
		c.includeEvents.Enqueue(ctx, iev)

		if resp != nil {
			candidates := resp.CloserNodes()
			if len(candidates) > 0 {
				// ignore error here
				c.AddNodes(ctx, candidates)
			}
		}
	}

	// this might be new node addressing info
	c.ep.MaybeAddToPeerstore(ctx, to, c.cfg.PeerstoreTTL)

	protoID, msg := c.findNodeFn(c.self)
	err := c.ep.SendRequestHandleResponse(ctx, protoID, to.ID(), msg, msg.EmptyResponse(), 0, onMessageResponse)
	if err != nil {
		onSendError(ctx, err)
	}
}

func (c *Coordinator[K, A]) StartQuery(ctx context.Context, queryID query.QueryID, protocolID address.ProtocolID, msg kad.Request[K, A]) error {
	ctx, span := util.StartSpan(ctx, "Coordinator.StartQuery")
	defer span.End()
	knownClosestPeers := c.rt.NearestNodes(msg.Target(), 20)

	qev := &query.EventPoolAddQuery[K, A]{
		QueryID:           queryID,
		Target:            msg.Target(),
		ProtocolID:        protocolID,
		Message:           msg,
		KnownClosestNodes: knownClosestPeers,
	}
	c.poolEvents.Enqueue(ctx, qev)
	return nil
}

func (c *Coordinator[K, A]) StopQuery(ctx context.Context, queryID query.QueryID) error {
	ctx, span := util.StartSpan(ctx, "Coordinator.StopQuery")
	defer span.End()
	qev := &query.EventPoolStopQuery{
		QueryID: queryID,
	}
	c.poolEvents.Enqueue(ctx, qev)
	return nil
}

// AddNodes suggests new DHT nodes and their associated addresses to be added to the routing table.
// If the routing table is updated as a result of this operation a KademliaRoutingUpdatedEvent event is emitted.
func (c *Coordinator[K, A]) AddNodes(ctx context.Context, infos []kad.NodeInfo[K, A]) error {
	ctx, span := util.StartSpan(ctx, "Coordinator.AddNodes")
	defer span.End()
	for _, info := range infos {
		if key.Equal(info.ID().Key(), c.self.Key()) {
			// skip self
			continue
		}
		// inject a new node into the coordinator's includeEvents queue
		iev := &routing.EventIncludeAddCandidate[K, A]{
			NodeInfo: info,
		}
		c.includeEvents.Enqueue(ctx, iev)
	}

	return nil
}

// Bootstrap instructs the coordinator to begin bootstrapping the routing table.
// While bootstrap is in progress, no other queries will make progress.
func (c *Coordinator[K, A]) Bootstrap(ctx context.Context, seeds []kad.NodeID[K]) error {
	protoID, msg := c.findNodeFn(c.self)

	bev := &routing.EventBootstrapStart[K, A]{
		ProtocolID:        protoID,
		Message:           msg,
		KnownClosestNodes: seeds,
	}

	c.bootstrapEvents.Enqueue(ctx, bev)

	return nil
}

// Kademlia events emitted by the Coordinator, intended for consumption by clients of the package

type KademliaEvent interface {
	kademliaEvent()
}

// KademliaOutboundQueryProgressedEvent is emitted by the coordinator when a query has received a
// response from a node.
type KademliaOutboundQueryProgressedEvent[K kad.Key[K], A kad.Address[A]] struct {
	QueryID  query.QueryID
	NodeID   kad.NodeID[K]
	Response kad.Response[K, A]
	Stats    query.QueryStats
}

// KademliaOutboundQueryFinishedEvent is emitted by the coordinator when a query has finished, either through
// running to completion or by being canceled.
type KademliaOutboundQueryFinishedEvent struct {
	QueryID query.QueryID
	Stats   query.QueryStats
}

// KademliaRoutingUpdatedEvent is emitted by the coordinator when a new node has been added to the routing table.
type KademliaRoutingUpdatedEvent[K kad.Key[K], A kad.Address[A]] struct {
	NodeInfo kad.NodeInfo[K, A]
}

type KademliaUnroutablePeerEvent[K kad.Key[K]] struct{}

type KademliaRoutablePeerEvent[K kad.Key[K]] struct{}

// KademliaBootstrapFinishedEvent is emitted by the coordinator when a bootstrap has finished, either through
// running to completion or by being canceled.
type KademliaBootstrapFinishedEvent struct {
	Stats query.QueryStats
}

// kademliaEvent() ensures that only Kademlia events can be assigned to a KademliaEvent.
func (*KademliaRoutingUpdatedEvent[K, A]) kademliaEvent()          {}
func (*KademliaOutboundQueryProgressedEvent[K, A]) kademliaEvent() {}
func (*KademliaUnroutablePeerEvent[K]) kademliaEvent()             {}
func (*KademliaRoutablePeerEvent[K]) kademliaEvent()               {}
func (*KademliaOutboundQueryFinishedEvent) kademliaEvent()         {}
func (*KademliaBootstrapFinishedEvent) kademliaEvent()             {}

// var _ scheduler.Scheduler = (*Coordinator[key.Key8])(nil)
func (c *Coordinator[K, A]) Clock() clock.Clock {
	return c.cfg.Clock
}

func (c *Coordinator[K, A]) EnqueueAction(ctx context.Context, a event.Action) {
	c.queue.Enqueue(ctx, a)
}

func (c *Coordinator[K, A]) ScheduleAction(ctx context.Context, t time.Time, a event.Action) event.PlannedAction {
	if c.cfg.Clock.Now().After(t) {
		c.EnqueueAction(ctx, a)
		return nil
	}
	return c.planner.ScheduleAction(ctx, t, a)
}

func (c *Coordinator[K, A]) RemovePlannedAction(ctx context.Context, a event.PlannedAction) bool {
	return c.planner.RemoveAction(ctx, a)
}

// NextActionTime returns the time of the next action to run, or the current
// time if there are actions to be run in the queue, or util.MaxTime if there
// are no scheduled to run.
func (c *Coordinator[K, A]) NextActionTime(ctx context.Context) time.Time {
	return c.cfg.Clock.Now()
}
