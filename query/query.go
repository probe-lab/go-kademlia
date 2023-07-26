package query

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type QueryID string

const InvalidQueryID QueryID = ""

type Query[K kad.Key[K], A kad.Address[A]] struct {
	id         QueryID
	clk        clock.Clock
	iter       NodeIter[K]
	protocolID address.ProtocolID
	msg        kad.Request[K, A]
	stats      QueryStats
}

// Advance advances the state of the query by attempting to advance its iterator
func (q *Query[K, A]) Advance(ctx context.Context, ev QueryEvent) QueryState {
	ctx, span := util.StartSpan(ctx, "Query.Advance", trace.WithAttributes(
		attribute.String("QueryID", string(q.id))))
	defer span.End()

	var nev NodeIterEvent

	switch tev := ev.(type) {
	case *QueryEventCancel:
		nev = &EventNodeIterCancel{}
	case *QueryEventMessageResponse[K, A]:
		q.stats.Success++

		nodes := make([]kad.NodeID[K], len(tev.Response.CloserNodes()))
		for i, a := range tev.Response.CloserNodes() {
			nodes[i] = a.ID()
		}

		nev = &EventNodeIterNodeContacted[K]{
			NodeID:      tev.NodeID,
			CloserNodes: nodes,
		}
	case nil:
		// TEMPORARY: no event to process
	default:
		panic(fmt.Sprintf("unexpected event: %T", tev))
	}

	state := q.iter.Advance(ctx, nev)
	switch st := state.(type) {

	case *StateNodeIterFinished[K]:
		if q.stats.End.IsZero() {
			q.stats.End = q.clk.Now()
		}
		return &QueryStateFinished[K]{
			QueryID: q.id,
			Stats:   q.stats,
		}

	case *StateNodeIterWaitingContact[K]:
		q.stats.Requests++
		return &QueryStateWaitingMessage[K, A]{
			QueryID:    q.id,
			Stats:      q.stats,
			NodeID:     st.NodeID,
			ProtocolID: q.protocolID,
			Message:    q.msg,
		}

	case *StateNodeIterWaiting:
		return &QueryStateWaiting{
			QueryID: q.id,
			Stats:   q.stats,
		}
	case *StateNodeIterWaitingAtCapacity:
		return &QueryStateWaitingAtCapacity{
			QueryID: q.id,
			Stats:   q.stats,
		}
	case *StateNodeIterWaitingWithCapacity:
		return &QueryStateWaitingWithCapacity{
			QueryID: q.id,
			Stats:   q.stats,
		}
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}
}

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

// QueryStateFinished indicates that the Query has finished.
type QueryStateFinished[K kad.Key[K]] struct {
	QueryID QueryID
	Stats   QueryStats
}

// QueryStateWaitingMessage indicates that the Query is waiting to send a message to a node.
type QueryStateWaitingMessage[K kad.Key[K], A kad.Address[A]] struct {
	QueryID    QueryID
	Stats      QueryStats
	NodeID     kad.NodeID[K]
	ProtocolID address.ProtocolID
	Message    kad.Request[K, A]
}

// QueryStateWaiting indicates that the Query is waiting for results from one or more nodes.
type QueryStateWaiting struct {
	QueryID QueryID
	Stats   QueryStats
}

// QueryStateWaitingAtCapacity indicates that the Query is waiting for results and is at capacity.
type QueryStateWaitingAtCapacity struct {
	QueryID QueryID
	Stats   QueryStats
}

// QueryStateWaitingWithCapacity indicates that the Query is waiting for results but has no further nodes to contact.
type QueryStateWaitingWithCapacity struct {
	QueryID QueryID
	Stats   QueryStats
}

// queryState() ensures that only Query states can be assigned to a QueryState.
func (*QueryStateFinished[K]) queryState()          {}
func (*QueryStateWaitingMessage[K, A]) queryState() {}
func (*QueryStateWaiting) queryState()              {}
func (*QueryStateWaitingAtCapacity) queryState()    {}
func (*QueryStateWaitingWithCapacity) queryState()  {}

type QueryEvent interface {
	queryEvent()
}

type QueryEventCancel struct{}

type QueryEventMessageResponse[K kad.Key[K], A kad.Address[A]] struct {
	NodeID   kad.NodeID[K]
	Response kad.Response[K, A]
}

// queryEvent() ensures that only Query events can be assigned to a QueryEvent.
func (*QueryEventCancel) queryEvent()                {}
func (*QueryEventMessageResponse[K, A]) queryEvent() {}
