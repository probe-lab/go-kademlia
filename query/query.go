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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type QueryID string

const InvalidQueryID QueryID = ""

type QueryStats struct {
	Start    time.Time
	End      time.Time
	Requests int
	Success  int
	Failure  int
}

type QueryState interface {
	queryState()
}

// StateQueryFinished indicates that the Query has finished.
type StateQueryFinished struct {
	QueryID QueryID
	Stats   QueryStats
}

// StateQueryWaitingMessage indicates that the Query is waiting to send a message to a node.
type StateQueryWaitingMessage[K kad.Key[K], A kad.Address[A]] struct {
	QueryID    QueryID
	Stats      QueryStats
	NodeID     kad.NodeID[K]
	ProtocolID address.ProtocolID
	Message    kad.Request[K, A]
}

// StateQueryWaiting indicates that the Query is waiting for results from one or more nodes.
type StateQueryWaiting struct {
	QueryID QueryID
	Stats   QueryStats
}

// StateQueryWaitingAtCapacity indicates that the Query is waiting for results and is at capacity.
type StateQueryWaitingAtCapacity struct {
	QueryID QueryID
	Stats   QueryStats
}

// StateQueryWaitingWithCapacity indicates that the Query is waiting for results but has no further nodes to contact.
type StateQueryWaitingWithCapacity struct {
	QueryID QueryID
	Stats   QueryStats
}

// queryState() ensures that only Query states can be assigned to a QueryState.
func (*StateQueryFinished) queryState()             {}
func (*StateQueryWaitingMessage[K, A]) queryState() {}
func (*StateQueryWaiting) queryState()              {}
func (*StateQueryWaitingAtCapacity) queryState()    {}
func (*StateQueryWaitingWithCapacity) queryState()  {}

type QueryEvent interface {
	queryEvent()
}

type EventQueryCancel struct{}

type EventQueryMessageResponse[K kad.Key[K], A kad.Address[A]] struct {
	NodeID   kad.NodeID[K]
	Response kad.Response[K, A]
}

type EventQueryMessageFailure[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

// queryEvent() ensures that only Query events can be assigned to a QueryEvent.
func (*EventQueryCancel) queryEvent()                {}
func (*EventQueryMessageResponse[K, A]) queryEvent() {}
func (*EventQueryMessageFailure[K]) queryEvent()     {}

// QueryConfig specifies optional configuration for a Query
type QueryConfig struct {
	Clock clock.Clock // a clock that may replaced by a mock when testing
}

// Validate checks the configuration options and returns an error if any have invalid values.
func (cfg *QueryConfig) Validate() error {
	if cfg.Clock == nil {
		return &kaderr.ConfigurationError{
			Component: "QueryConfig",
			Err:       fmt.Errorf("Clock must not be nil"),
		}
	}
	return nil
}

// DefaultQueryConfig returns the default configuration options for a Query.
// Options may be overridden before passing to NewQuery
func DefaultQueryConfig() *QueryConfig {
	return &QueryConfig{
		Clock: clock.New(), // use standard time
	}
}

type Query[K kad.Key[K], A kad.Address[A]] struct {
	id QueryID

	// cfg is a copy of the optional configuration supplied to the query
	cfg QueryConfig

	iter       NodeIter[K]
	protocolID address.ProtocolID
	msg        kad.Request[K, A]
	stats      QueryStats

	// finished indicates that that the query has completed its work or has been stopped.
	finished bool
}

func NewQuery[K kad.Key[K], A kad.Address[A]](id QueryID, protocolID address.ProtocolID, msg kad.Request[K, A], iter NodeIter[K], cfg *QueryConfig) (*Query[K, A], error) {
	if cfg == nil {
		cfg = DefaultQueryConfig()
	} else if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Query[K, A]{
		id:         id,
		cfg:        *cfg,
		iter:       iter,
		protocolID: protocolID,
		msg:        msg,
	}, nil
}

// Advance advances the state of the query by attempting to advance its iterator
func (q *Query[K, A]) Advance(ctx context.Context, ev QueryEvent) QueryState {
	ctx, span := util.StartSpan(ctx, "Query.Advance", trace.WithAttributes(
		attribute.String("QueryID", string(q.id))))
	defer span.End()

	if q.finished {
		return &StateQueryFinished{
			QueryID: q.id,
			Stats:   q.stats,
		}
	}

	var nev NodeIterEvent

	switch tev := ev.(type) {
	case *EventQueryCancel:
		nev = &EventNodeIterCancel{}
	case *EventQueryMessageResponse[K, A]:

		var nodes []kad.NodeID[K]

		if tev.Response != nil {
			nodes = make([]kad.NodeID[K], len(tev.Response.CloserNodes()))
			for i, a := range tev.Response.CloserNodes() {
				nodes[i] = a.ID()
			}
		}

		nev = &EventNodeIterNodeContacted[K]{
			NodeID:      tev.NodeID,
			CloserNodes: nodes,
		}
	case *EventQueryMessageFailure[K]:
		nev = &EventNodeIterNodeNotContacted[K]{
			NodeID: tev.NodeID,
		}
	case nil:
		// TEMPORARY: no event to process
	default:
		panic(fmt.Sprintf("unexpected event: %T", tev))
	}

	state := q.iter.Advance(ctx, nev)
	switch st := state.(type) {

	case *StateNodeIterFinished:
		q.finished = true
		q.stats.Success = st.Successes
		if q.stats.End.IsZero() {
			q.stats.End = q.cfg.Clock.Now()
		}
		return &StateQueryFinished{
			QueryID: q.id,
			Stats:   q.stats,
		}

	case *StateNodeIterWaitingContact[K]:
		q.stats.Requests++
		return &StateQueryWaitingMessage[K, A]{
			QueryID:    q.id,
			Stats:      q.stats,
			NodeID:     st.NodeID,
			ProtocolID: q.protocolID,
			Message:    q.msg,
		}

	case *StateNodeIterWaiting:
		return &StateQueryWaiting{
			QueryID: q.id,
			Stats:   q.stats,
		}
	case *StateNodeIterWaitingAtCapacity:
		return &StateQueryWaitingAtCapacity{
			QueryID: q.id,
			Stats:   q.stats,
		}
	case *StateNodeIterWaitingWithCapacity:
		return &StateQueryWaitingWithCapacity{
			QueryID: q.id,
			Stats:   q.stats,
		}
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}
}
