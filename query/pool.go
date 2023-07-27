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

type QueryPool[K kad.Key[K], A kad.Address[A]] struct {
	clk         clock.Clock
	timeout     time.Duration
	concurrency int // the 'α' parameter defined by Kademlia
	replication int // the 'k' parameter defined by Kademlia
	queries     map[QueryID]*Query[K, A]
}

// QueryPoolConfig specifies optional configuration for a QueryPool
type QueryPoolConfig struct {
	Concurrency int         // the 'α' parameter defined by Kademlia, the maximum number of requests that may be in flight for each query
	Replication int         // the 'k' parameter defined by Kademlia
	Clock       clock.Clock // a clock that may replaced by a mock when testing
}

// Validate checks the configuration options and returns an error if any have invalid values.
func (cfg *QueryPoolConfig) Validate() error {
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

// DefaultQueryPoolConfig returns the default configuration options for a QueryPool.
// Options may be overridden before passing to NewQueryPool
func DefaultQueryPoolConfig() *QueryPoolConfig {
	return &QueryPoolConfig{
		Clock:       clock.New(), // use standard time
		Concurrency: 3,
		Replication: 20,
	}
}

func NewQueryPool[K kad.Key[K], A kad.Address[A]](cfg *QueryPoolConfig) (*QueryPool[K, A], error) {
	if cfg == nil {
		cfg = DefaultQueryPoolConfig()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &QueryPool[K, A]{
		timeout:     time.Minute,
		clk:         cfg.Clock,
		concurrency: cfg.Concurrency,
		replication: cfg.Replication,
		queries:     make(map[QueryID]*Query[K, A]),
	}, nil
}

// Advance advances the state of the query pool by attempting to advance one of its queries
func (qp *QueryPool[K, A]) Advance(ctx context.Context, ev QueryPoolEvent) QueryPoolState {
	ctx, span := util.StartSpan(ctx, "QueryPool.Advance")
	defer span.End()
	switch tev := ev.(type) {
	case *QueryPoolEventAdd[K, A]:
		qp.addQuery(ctx, tev.QueryID, tev.Target, tev.ProtocolID, tev.Message, tev.KnownClosestPeers)
		// TODO: return error as state
	case *QueryPoolEventStop[K]:
		if qry, ok := qp.queries[tev.QueryID]; ok {
			state, terminal := qp.advanceQuery(ctx, qry, &EventQueryCancel{})
			if terminal {
				return state
			}
		}
	case *QueryPoolEventMessageResponse[K, A]:
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
		return &QueryPoolIdle{}
	}

	// Attempt to advance another query
	for _, qry := range qp.queries {
		state, terminal := qp.advanceQuery(ctx, qry, nil)
		if terminal {
			return state
		}
	}

	return &QueryPoolIdle{}
}

func (qp *QueryPool[K, A]) advanceQuery(ctx context.Context, qry *Query[K, A], qev QueryEvent) (QueryPoolState, bool) {
	state := qry.Advance(ctx, qev)
	switch st := state.(type) {
	case *StateQueryWaiting:
		return &QueryPoolWaiting{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}, true
	case *StateQueryWaitingMessage[K, A]:
		return &QueryPoolWaitingMessage[K, A]{
			QueryID:    st.QueryID,
			Stats:      st.Stats,
			NodeID:     st.NodeID,
			ProtocolID: st.ProtocolID,
			Message:    st.Message,
		}, true
	case *StateQueryFinished:
		delete(qp.queries, qry.id)
		return &QueryPoolFinished{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}, true
	case *StateQueryWaitingAtCapacity:
		elapsed := qp.clk.Since(qry.stats.Start)
		if elapsed > qp.timeout {
			delete(qp.queries, qry.id)
			return &QueryPoolTimeout{
				QueryID: st.QueryID,
				Stats:   st.Stats,
			}, true
		}
	case *StateQueryWaitingWithCapacity:
		return &QueryPoolWaitingWithCapacity{
			QueryID: st.QueryID,
			Stats:   st.Stats,
		}, true
	}
	return nil, false
}

// addQuery adds a query to the pool, returning the new query id
// TODO: remove target argument and use msg.Target
func (qp *QueryPool[K, A]) addQuery(ctx context.Context, queryID QueryID, target K, protocolID address.ProtocolID, msg kad.Request[K, A], knownClosestPeers []kad.NodeID[K]) error {
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

type QueryPoolState interface {
	queryPoolState()
}

// QueryPoolIdle indicates that the pool is idle, i.e. there are no queries to process.
type QueryPoolIdle struct{}

// QueryPoolWaiting indicates that at least one query is waiting for results.
type QueryPoolWaiting struct {
	QueryID QueryID
	Stats   QueryStats
}

// QueryPoolWaitingMessage indicates that at a query is waiting to message a peer.
type QueryPoolWaitingMessage[K kad.Key[K], A kad.Address[A]] struct {
	QueryID    QueryID
	NodeID     kad.NodeID[K]
	ProtocolID address.ProtocolID
	Message    kad.Request[K, A]
	Stats      QueryStats
}

// QueryPoolWaitingWithCapacity indicates that at least one query is waiting for results but it is not at capacity.
type QueryPoolWaitingWithCapacity struct {
	QueryID QueryID
	Stats   QueryStats
}

// QueryPoolFinished indicates that a query has finished.
type QueryPoolFinished struct {
	QueryID QueryID
	Stats   QueryStats
}

// QueryPoolTimeout indicates that at a query has timed out.
type QueryPoolTimeout struct {
	QueryID QueryID
	Stats   QueryStats
}

// queryPoolState() ensures that only QueryPool states can be assigned to a QueryPoolState.
func (*QueryPoolIdle) queryPoolState()                 {}
func (*QueryPoolWaiting) queryPoolState()              {}
func (*QueryPoolWaitingMessage[K, A]) queryPoolState() {}
func (*QueryPoolWaitingWithCapacity) queryPoolState()  {}
func (*QueryPoolFinished) queryPoolState()             {}
func (*QueryPoolTimeout) queryPoolState()              {}

// QueryPoolEvent is an event intended to advance the state of a query pool.
type QueryPoolEvent interface {
	queryPoolEvent()
}

// QueryPoolEventAdd is an event that attempts to add a new query
type QueryPoolEventAdd[K kad.Key[K], A kad.Address[A]] struct {
	QueryID           QueryID
	Target            K
	ProtocolID        address.ProtocolID
	Message           kad.Request[K, A]
	KnownClosestPeers []kad.NodeID[K]
}

// QueryPoolEventStop is an event that attempts to add a new query
type QueryPoolEventStop[K kad.Key[K]] struct {
	QueryID QueryID
}

// QueryPoolEventMessageResponse is an event that notifies a query that a sent message has had a response.
type QueryPoolEventMessageResponse[K kad.Key[K], A kad.Address[A]] struct {
	QueryID  QueryID
	NodeID   kad.NodeID[K]
	Response kad.Response[K, A]
}

type QueryPoolEventMessageFailure[K kad.Key[K]] struct {
	QueryID QueryID
	NodeID  kad.NodeID[K]
}

// queryPoolEvent() ensures that only QueryPool events can be assigned to a QueryPoolEvent.
func (*QueryPoolEventAdd[K, A]) queryPoolEvent()             {}
func (*QueryPoolEventStop[K]) queryPoolEvent()               {}
func (*QueryPoolEventMessageResponse[K, A]) queryPoolEvent() {}
func (*QueryPoolEventMessageFailure[K]) queryPoolEvent()     {}
