package query

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/kaderr"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/util"
)

type Pool[K kad.Key[K], A kad.Address[A]] struct {
	clk         clock.Clock
	timeout     time.Duration
	concurrency int // the 'α' parameter defined by Kademlia
	replication int // the 'k' parameter defined by Kademlia
	queries     map[QueryID]*Query[K, A]
}

// PoolConfig specifies optional configuration for a Pool
type PoolConfig struct {
	Concurrency int         // the 'α' parameter defined by Kademlia, the maximum number of requests that may be in flight for each query
	Replication int         // the 'k' parameter defined by Kademlia
	Clock       clock.Clock // a clock that may replaced by a mock when testing
}

// Validate checks the configuration options and returns an error if any have invalid values.
func (cfg *PoolConfig) Validate() error {
	if cfg.Clock == nil {
		return &kaderr.ConfigurationError{
			Component: "QueryPoolConfig",
			Err:       fmt.Errorf("clock must not be nil"),
		}
	}
	if cfg.Concurrency < 1 {
		return &kaderr.ConfigurationError{
			Component: "QueryPoolConfig",
			Err:       fmt.Errorf("concurrency must be greater than zero"),
		}
	}
	if cfg.Replication < 1 {
		return &kaderr.ConfigurationError{
			Component: "QueryPoolConfig",
			Err:       fmt.Errorf("replication must be greater than zero"),
		}
	}
	return nil
}

// DefaultPoolConfig returns the default configuration options for a Pool.
// Options may be overridden before passing to NewPool
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Clock:       clock.New(), // use standard time
		Concurrency: 3,
		Replication: 20,
	}
}

func NewPool[K kad.Key[K], A kad.Address[A]](cfg *PoolConfig) (*Pool[K, A], error) {
	if cfg == nil {
		cfg = DefaultPoolConfig()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Pool[K, A]{
		timeout:     time.Minute,
		clk:         cfg.Clock,
		concurrency: cfg.Concurrency,
		replication: cfg.Replication,
		queries:     make(map[QueryID]*Query[K, A]),
	}, nil
}

// Advance advances the state of the pool by attempting to advance one of its queries
func (qp *Pool[K, A]) Advance(ctx context.Context, ev QueryPoolEvent) PoolState {
	ctx, span := util.StartSpan(ctx, "Pool.Advance")
	defer span.End()
	switch tev := ev.(type) {
	case *EventQueryPoolAdd[K, A]:
		qp.addQuery(ctx, tev.QueryID, tev.Target, tev.ProtocolID, tev.Message, tev.KnownClosestPeers)
		// TODO: return error as state
	case *EventQueryPoolStop[K]:
		if qry, ok := qp.queries[tev.QueryID]; ok {
			state, terminal := qp.advanceQuery(ctx, qry, &EventQueryCancel{})
			if terminal {
				return state
			}
		}
	case *EventQueryPoolEventResponse[K, A]:
		if qry, ok := qp.queries[tev.QueryID]; ok {
			state, terminal := qp.advanceQuery(ctx, qry, &EventQueryMessageResponse[K, A]{
				NodeID:   tev.NodeID,
				Response: tev.Response,
			})
			if terminal {
				return state
			}
		}
	case nil:
		// TEMPORARY: no event to process
	default:
		panic(fmt.Sprintf("unexpected event: %T", tev))
	}

	if len(qp.queries) == 0 {
		return &StatePoolIdle{}
	}

	// Attempt to advance another query
	for _, qry := range qp.queries {
		state, terminal := qp.advanceQuery(ctx, qry, nil)
		if terminal {
			return state
		}
	}

	return &StatePoolIdle{}
}

func (qp *Pool[K, A]) advanceQuery(ctx context.Context, qry *Query[K, A], qev QueryEvent) (PoolState, bool) {
	state := qry.Advance(ctx, qev)
	switch st := state.(type) {
	case *StateQueryWaiting:
		return &StatePoolWaiting{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}, true
	case *StateQueryWaitingMessage[K, A]:
		return &StatePoolQueryMessage[K, A]{
			QueryID:    st.QueryID,
			Stats:      st.Stats,
			NodeID:     st.NodeID,
			ProtocolID: st.ProtocolID,
			Message:    st.Message,
		}, true
	case *StateQueryFinished:
		delete(qp.queries, qry.id)
		return &StatePoolQueryFinished{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}, true
	case *StateQueryWaitingAtCapacity:
		elapsed := qp.clk.Since(qry.stats.Start)
		if elapsed > qp.timeout {
			delete(qp.queries, qry.id)
			return &StatePoolQueryTimeout{
				QueryID: st.QueryID,
				Stats:   st.Stats,
			}, true
		}
	case *StateQueryWaitingWithCapacity:
		return &StatePoolWaitingWithCapacity{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}, true
	}
	return nil, false
}

// addQuery adds a query to the pool, returning the new query id
// TODO: remove target argument and use msg.Target
func (qp *Pool[K, A]) addQuery(ctx context.Context, queryID QueryID, target K, protocolID address.ProtocolID, msg kad.Request[K, A], knownClosestPeers []kad.NodeID[K]) error {
	// TODO: return an error if queryID already in use
	iter := NewClosestNodesIter(target)

	qryCfg := DefaultQueryConfig()
	qryCfg.Clock = qp.clk

	var err error
	qp.queries[queryID], err = NewQuery[K, A](queryID, protocolID, msg, iter, knownClosestPeers, qryCfg)
	if err != nil {
		return fmt.Errorf("new query: %w", err)
	}
	return nil
}

// States

type PoolState interface {
	poolState()
}

// StatePoolIdle indicates that the pool is idle, i.e. there are no queries to process.
type StatePoolIdle struct{}

// StatePoolWaiting indicates that at least one query is waiting for results.
type StatePoolWaiting struct {
	QueryID QueryID
	Stats   QueryStats
}

// StatePoolQueryMessage indicates that at a query is waiting to message a peer.
type StatePoolQueryMessage[K kad.Key[K], A kad.Address[A]] struct {
	QueryID    QueryID
	NodeID     kad.NodeID[K]
	ProtocolID address.ProtocolID
	Message    kad.Request[K, A]
	Stats      QueryStats
}

// StatePoolWaitingWithCapacity indicates that at least one query is waiting for results but it is not at capacity.
type StatePoolWaitingWithCapacity struct {
	QueryID QueryID
	Stats   QueryStats
}

// StatePoolQueryFinished indicates that a query has finished.
type StatePoolQueryFinished struct {
	QueryID QueryID
	Stats   QueryStats
}

// StatePoolQueryTimeout indicates that at a query has timed out.
type StatePoolQueryTimeout struct {
	QueryID QueryID
	Stats   QueryStats
}

// poolState() ensures that only Pool states can be assigned to the PoolState interface.
func (*StatePoolIdle) poolState()                {}
func (*StatePoolWaiting) poolState()             {}
func (*StatePoolQueryMessage[K, A]) poolState()  {}
func (*StatePoolWaitingWithCapacity) poolState() {}
func (*StatePoolQueryFinished) poolState()       {}
func (*StatePoolQueryTimeout) poolState()        {}

// QueryPoolEvent is an event intended to advance the state of a query pool.
type QueryPoolEvent interface {
	queryPoolEvent()
}

// EventQueryPoolAdd is an event that attempts to add a new query
type EventQueryPoolAdd[K kad.Key[K], A kad.Address[A]] struct {
	QueryID           QueryID
	Target            K
	ProtocolID        address.ProtocolID
	Message           kad.Request[K, A]
	KnownClosestPeers []kad.NodeID[K]
}

// EventQueryPoolStop is an event that attempts to add a new query
type EventQueryPoolStop[K kad.Key[K]] struct {
	QueryID QueryID
}

// EventQueryPoolEventResponse is an event that notifies a query that a sent message has had a response.
type EventQueryPoolEventResponse[K kad.Key[K], A kad.Address[A]] struct {
	QueryID  QueryID
	NodeID   kad.NodeID[K]
	Response kad.Response[K, A]
}

type EventQueryPoolMessageFailure[K kad.Key[K]] struct {
	QueryID QueryID
	NodeID  kad.NodeID[K]
}

// queryPoolEvent() ensures that only QueryPool events can be assigned to a QueryPoolEvent.
func (*EventQueryPoolAdd[K, A]) queryPoolEvent()           {}
func (*EventQueryPoolStop[K]) queryPoolEvent()             {}
func (*EventQueryPoolEventResponse[K, A]) queryPoolEvent() {}
func (*EventQueryPoolMessageFailure[K]) queryPoolEvent()   {}
