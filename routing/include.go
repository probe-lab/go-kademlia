package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/event"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/kaderr"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/util"
)

type Include[K kad.Key[K], A kad.Address[A]] struct {
	// self is the node id of the system the bootstrap is running on
	self kad.NodeID[K]

	// qry is the query used by the bootstrap process
	qry *query.Query[K, A]

	queue event.EventQueue

	// cfg is a copy of the optional configuration supplied to the Include
	cfg IncludeConfig[K, A]
}

// IncludeConfig specifies optional configuration for an Include
type IncludeConfig[K kad.Key[K], A kad.Address[A]] struct {
	Timeout            time.Duration // the time to wait before terminating a query that is not making progress
	RequestConcurrency int           // the maximum number of concurrent requests that each query may have in flight
	RequestTimeout     time.Duration // the timeout queries should use for contacting a single node
	QueueCapacity      int           // the size of the event queue
	Clock              clock.Clock   // a clock that may replaced by a mock when testing
}

// Validate checks the configuration options and returns an error if any have invalid values.
func (cfg *IncludeConfig[K, A]) Validate() error {
	if cfg.Clock == nil {
		return &kaderr.ConfigurationError{
			Component: "IncludeConfig",
			Err:       fmt.Errorf("clock must not be nil"),
		}
	}

	if cfg.Timeout < 1 {
		return &kaderr.ConfigurationError{
			Component: "IncludeConfig",
			Err:       fmt.Errorf("timeout must be greater than zero"),
		}
	}

	if cfg.RequestConcurrency < 1 {
		return &kaderr.ConfigurationError{
			Component: "IncludeConfig",
			Err:       fmt.Errorf("request concurrency must be greater than zero"),
		}
	}

	if cfg.RequestTimeout < 1 {
		return &kaderr.ConfigurationError{
			Component: "IncludeConfig",
			Err:       fmt.Errorf("request timeout must be greater than zero"),
		}
	}

	if cfg.QueueCapacity < 1 {
		return &kaderr.ConfigurationError{
			Component: "PoolConfig",
			Err:       fmt.Errorf("queue capacity must be greater than zero"),
		}
	}

	return nil
}

// DefaultIncludeConfig returns the default configuration options for an Include.
// Options may be overridden before passing to NewInclude
func DefaultIncludeConfig[K kad.Key[K], A kad.Address[A]]() *IncludeConfig[K, A] {
	return &IncludeConfig[K, A]{
		Clock:              clock.New(), // use standard time
		Timeout:            5 * time.Minute,
		RequestConcurrency: 3,
		RequestTimeout:     time.Minute,
		QueueCapacity:      128,
	}
}

func NewInclude[K kad.Key[K], A kad.Address[A]](self kad.NodeID[K], cfg *IncludeConfig[K, A]) (*Include[K, A], error) {
	if cfg == nil {
		cfg = DefaultIncludeConfig[K, A]()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Include[K, A]{
		self:  self,
		cfg:   *cfg,
		queue: event.NewChanQueue(cfg.QueueCapacity),
	}, nil
}

// Enqueue enqueues an event to be processed by the state machine.
func (b *Include[K, A]) Enqueue(ctx context.Context, ev IncludeEvent) {
	ctx, span := util.StartSpan(ctx, "Include.Enqueue")
	defer span.End()
	b.queue.Enqueue(ctx, ev)
}

// Advance advances the state of the bootstrap by attempting to advance its query if running.
func (b *Include[K, A]) Advance(ctx context.Context) IncludeState {
	ctx, span := util.StartSpan(ctx, "Include.Advance")
	defer span.End()

	ev := b.queue.Dequeue(ctx)

	switch tev := ev.(type) {
	case *EventIncludeStart[K, A]:
		// TODO: ignore start event if query is already in progress
		iter := query.NewClosestNodesIter(b.self.Key())

		qryCfg := query.DefaultQueryConfig[K]()
		qryCfg.Clock = b.cfg.Clock
		qryCfg.Concurrency = b.cfg.RequestConcurrency
		qryCfg.RequestTimeout = b.cfg.RequestTimeout

		queryID := query.QueryID("bootstrap")

		qry, err := query.NewQuery[K](b.self, queryID, tev.ProtocolID, tev.Message, iter, tev.KnownClosestNodes, qryCfg)
		if err != nil {
			// TODO: don't panic
			panic(err)
		}
		b.qry = qry
		return b.advanceQuery(ctx, nil)

	case *EventIncludeMessageResponse[K, A]:
		return b.advanceQuery(ctx, &query.EventQueryMessageResponse[K, A]{
			NodeID:   tev.NodeID,
			Response: tev.Response,
		})
	case *EventIncludeMessageFailure[K]:
		return b.advanceQuery(ctx, &query.EventQueryMessageFailure[K]{
			NodeID: tev.NodeID,
			Error:  tev.Error,
		})

	case *EventIncludePoll:
	case nil:
	// ignore, nothing to do
	default:
		panic(fmt.Sprintf("unexpected event: %T", tev))
	}

	if b.qry != nil {
		return b.advanceQuery(ctx, nil)
	}

	return &StateIncludeIdle{}
}

func (b *Include[K, A]) advanceQuery(ctx context.Context, qev query.QueryEvent) IncludeState {
	state := b.qry.Advance(ctx, qev)
	switch st := state.(type) {
	case *query.StateQueryWaitingMessage[K, A]:
		return &StateIncludeMessage[K, A]{
			QueryID:    st.QueryID,
			Stats:      st.Stats,
			NodeID:     st.NodeID,
			ProtocolID: st.ProtocolID,
			Message:    st.Message,
		}
	case *query.StateQueryFinished:
		return &StateIncludeFinished{
			Stats: st.Stats,
		}
	case *query.StateQueryWaitingAtCapacity:
		elapsed := b.cfg.Clock.Since(st.Stats.Start)
		if elapsed > b.cfg.Timeout {
			return &StateIncludeTimeout{
				Stats: st.Stats,
			}
		}
		return &StateIncludeWaiting{
			Stats: st.Stats,
		}
	case *query.StateQueryWaitingWithCapacity:
		elapsed := b.cfg.Clock.Since(st.Stats.Start)
		if elapsed > b.cfg.Timeout {
			return &StateIncludeTimeout{
				Stats: st.Stats,
			}
		}
		return &StateIncludeWaiting{
			Stats: st.Stats,
		}
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}
}

// IncludeState is the state of a bootstrap.
type IncludeState interface {
	bootstrapState()
}

// StateIncludeMessage indicates that the bootstrap query is waiting to message a node.
type StateIncludeMessage[K kad.Key[K], A kad.Address[A]] struct {
	QueryID    query.QueryID
	NodeID     kad.NodeID[K]
	ProtocolID address.ProtocolID
	Message    kad.Request[K, A]
	Stats      query.QueryStats
}

// StateIncludeIdle indicates that the bootstrap is not running its query.
type StateIncludeIdle struct{}

// StateIncludeFinished indicates that the bootstrap has finished.
type StateIncludeFinished struct {
	Stats query.QueryStats
}

// StateIncludeTimeout indicates that the bootstrap query has timed out.
type StateIncludeTimeout struct {
	Stats query.QueryStats
}

// StateIncludeWaiting indicates that the bootstrap query is waiting for a response.
type StateIncludeWaiting struct {
	Stats query.QueryStats
}

// bootstrapState() ensures that only Include states can be assigned to an IncludeState.
func (*StateIncludeMessage[K, A]) bootstrapState() {}
func (*StateIncludeIdle) bootstrapState()          {}
func (*StateIncludeFinished) bootstrapState()      {}
func (*StateIncludeTimeout) bootstrapState()       {}
func (*StateIncludeWaiting) bootstrapState()       {}

// IncludeEvent is an event intended to advance the state of a bootstrap.
type IncludeEvent interface {
	bootstrapEvent()
	Run(context.Context)
}

type EventIncludePoll struct{}

// EventIncludeStart is an event that attempts to start a new bootstrap
type EventIncludeStart[K kad.Key[K], A kad.Address[A]] struct {
	ProtocolID        address.ProtocolID
	Message           kad.Request[K, A]
	KnownClosestNodes []kad.NodeID[K]
}

// EventIncludeMessageResponse notifies a bootstrap that a sent message has received a successful response.
type EventIncludeMessageResponse[K kad.Key[K], A kad.Address[A]] struct {
	NodeID   kad.NodeID[K]      // the node the message was sent to
	Response kad.Response[K, A] // the message response sent by the node
}

// EventIncludeMessageFailure notifiesa bootstrap that an attempt to send a message has failed.
type EventIncludeMessageFailure[K kad.Key[K]] struct {
	NodeID kad.NodeID[K] // the node the message was sent to
	Error  error         // the error that caused the failure, if any
}

// bootstrapEvent() ensures that only Include events can be assigned to the IncludeEvent interface.
func (*EventIncludePoll) bootstrapEvent()                  {}
func (*EventIncludeStart[K, A]) bootstrapEvent()           {}
func (*EventIncludeMessageResponse[K, A]) bootstrapEvent() {}
func (*EventIncludeMessageFailure[K]) bootstrapEvent()     {}

// Run(context.Context) ensures that Include events can be assigned to the action.Action interface.
func (*EventIncludePoll) Run(context.Context)                  {}
func (*EventIncludeStart[K, A]) Run(context.Context)           {}
func (*EventIncludeMessageResponse[K, A]) Run(context.Context) {}
func (*EventIncludeMessageFailure[K]) Run(context.Context)     {}
