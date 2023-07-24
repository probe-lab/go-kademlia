package main

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

// var _ Task[QueryPoolState] = (*QueryPool)(nil)

type QueryPool[K kad.Key[K]] struct {
	self        *FakeNode[K]
	mr          *MessageRouter[K]
	timeout     time.Duration
	concurrency int // the 'α' parameter defined by Kademlia
	replication int // the 'k' parameter defined by Kademlia
	queries     map[QueryID]*Query[K]
	nextID      uint64
}

func NewQueryPool[K kad.Key[K]](self *FakeNode[K], mr *MessageRouter[K]) *QueryPool[K] {
	return &QueryPool[K]{
		self:        self,
		mr:          mr,
		timeout:     time.Minute,
		concurrency: 3,
		replication: 20,
		queries:     make(map[QueryID]*Query[K]),
		nextID:      1,
	}
}

// Advance advances the state of the query pool by attempting to advance one of its queries
func (p *QueryPool[K]) Advance(ctx context.Context) (rstate QueryPoolState) {
	trace("QueryPool.Advance")
	defer func() {
		traceReturnState("QueryPool.Advance", rstate)
	}()

	if len(p.queries) == 0 {
		return &QueryPoolIdle{}
	}

	// Attempt to advance a query
	for qid, query := range p.queries {
		state := query.Advance(ctx)
		switch st := state.(type) {
		case *PeerIterStateWaiting:
			return &QueryPoolWaiting{
				QueryID: qid,
				Stats:   query.stats,
			}
		case *PeerIterStateWaitingMessage[K]:
			return &QueryPoolWaitingMessage[K]{
				QueryID: qid,
				NodeID:  st.NodeID,
				Message: query.msg,
				Stats:   query.stats,
			}
		case *PeerIterStateFinished:
			delete(p.queries, qid)
			return &QueryPoolFinished{
				QueryID: qid,
				Stats:   query.stats,
			}
		case *PeerIterStateWaitingAtCapacity:
			elapsed := time.Since(query.stats.Start)
			if elapsed > p.timeout {
				delete(p.queries, qid)
				return &QueryPoolTimeout{
					QueryID: qid,
					Stats:   query.stats,
				}
			}
		case *PeerIterStateWaitingWithCapacity:
			return &QueryPoolWaitingWithCapacity{
				QueryID: qid,
				Stats:   query.stats,
			}
			// ignore
		default:
			panic(fmt.Sprintf("unexpected state: %T", st))
		}
	}

	return &QueryPoolIdle{}
}

// AddQuery adds a query to the pool, returning the new query id
func (qp *QueryPool[K]) AddQuery(ctx context.Context, target K, msg message.MinKadRequestMessage[K]) (QueryID, error) {
	trace("QueryPool.AddQuery")
	knownClosestPeers, err := qp.self.Closest(ctx, target, qp.replication)
	if err != nil {
		return 0, nil
	}

	iter := NewClosestPeersIter(target, qp.mr, knownClosestPeers, qp.replication, qp.concurrency, qp.timeout)
	id := qp.nextQueryID()
	// TODO: lock queries
	qp.queries[id] = &Query[K]{
		id:   id,
		iter: iter,
		msg:  msg,
	}
	return id, nil
}

func (qp *QueryPool[K]) StopQuery(ctx context.Context, queryID QueryID) error {
	trace("QueryPool.StopQuery")
	// TODO: lock queries
	query, ok := qp.queries[queryID]
	if !ok {
		return fmt.Errorf("unknown query")
	}
	query.Cancel(ctx)
	return nil
}

func (qp *QueryPool[K]) onMessageSuccess(ctx context.Context, queryID QueryID, node address.NodeID[K], resp message.MinKadResponseMessage[K]) {
	// TODO: lock queries
	query, ok := qp.queries[queryID]
	if !ok {
		return // unknown query
	}
	query.onMessageSuccess(ctx, node, resp)
}

func (q *QueryPool[K]) nextQueryID() QueryID {
	id := q.nextID
	q.nextID++
	return QueryID(id)
}

func (q *QueryPool[K]) Cancel(context.Context) {
	panic("not implemented")
}

// var _ Task[QueryState] = (*Query)(nil)

type Query[K kad.Key[K]] struct {
	id    QueryID
	iter  PeerIter[K]
	msg   message.MinKadRequestMessage[K]
	stats QueryStats
}

type QueryState interface {
	// TODO: decide whether to introduce specific QueryStates
	PeerIterState
}

// Advance advances the state of the query by attempting to advance its iterator
func (q *Query[K]) Advance(ctx context.Context) QueryState {
	trace("Query.Advance")
	state := q.iter.Advance(ctx)
	if _, ok := state.(*PeerIterStateWaiting); ok {
		q.stats.Requests++
	}
	return state
}

func (q *Query[K]) Cancel(ctx context.Context) {
	trace("Query.Cancel")
	q.iter.Cancel(ctx)
}

func (q *Query[K]) onMessageSuccess(ctx context.Context, node address.NodeID[K], resp message.MinKadResponseMessage[K]) {
	q.iter.OnMessageSuccess(ctx, node, resp)
}

type QueryStats struct {
	Start    time.Time
	End      time.Time
	Requests int
	Success  int
	Failure  int
}

// States

type QueryPoolState interface {
	State
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
type QueryPoolWaitingMessage[K kad.Key[K]] struct {
	QueryID QueryID
	NodeID  address.NodeID[K]
	Message message.MinKadRequestMessage[K]
	Stats   QueryStats
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
func (*QueryPoolIdle) queryPoolState()                {}
func (*QueryPoolWaiting) queryPoolState()             {}
func (*QueryPoolWaitingMessage[K]) queryPoolState()   {}
func (*QueryPoolWaitingWithCapacity) queryPoolState() {}
func (*QueryPoolFinished) queryPoolState()            {}
func (*QueryPoolTimeout) queryPoolState()             {}

// General Peer Iterator states
type PeerIterState interface {
	State
	peerIterState()
}

// PeerIterStateFinished indicates that the PeerIter has finished.
type PeerIterStateFinished struct{}

// PeerIterStateWaitingMessage indicates that the PeerIter is waiting to send a message to a peer.
type PeerIterStateWaitingMessage[K kad.Key[K]] struct {
	NodeID  address.NodeID[K]
	Message message.MinKadRequestMessage[K]
}

// PeerIterStateWaiting indicates that the PeerIter is waiting for results from one or more peers.
type PeerIterStateWaiting struct{}

// PeerIterStateWaiting indicates that the PeerIter is waiting for results and is at capacity.
type PeerIterStateWaitingAtCapacity struct{}

// PeerIterStateWaiting indicates that the PeerIter is waiting for results but has no further peers to contact.
type PeerIterStateWaitingWithCapacity struct{}

// peerIterState() ensures that only PeerIter states can be assigned to a PeerIterState.
func (*PeerIterStateFinished) peerIterState()            {}
func (*PeerIterStateWaitingMessage[K]) peerIterState()   {}
func (*PeerIterStateWaiting) peerIterState()             {}
func (*PeerIterStateWaitingAtCapacity) peerIterState()   {}
func (*PeerIterStateWaitingWithCapacity) peerIterState() {}

// A PeerIter iterates peers according to some strategy.
type PeerIter[K kad.Key[K]] interface {
	Task[PeerIterState]
	OnMessageSuccess(context.Context, address.NodeID[K], message.MinKadResponseMessage[K])
}

// var _ PeerIter = (*ClosestPeersIter)(nil)

type ClosestPeersIter[K kad.Key[K]] struct {
	// The target whose distance to any peer determines the position of the peer in the iterator.
	target K

	mr *MessageRouter[K]

	// current state of the iterator
	mu    sync.Mutex
	state ClosestPeersIterState

	// The closest peers to the target, ordered by increasing distance.
	peerlist *PeerList[K]

	// Number of peers to search for.
	numResults int

	// Maximum number of concurrent requests that may be in flight.
	concurrency int

	// Timeout for contacting a single peer
	timeout time.Duration

	// number of requests in flight, will be <= concurrency
	inFlight int
}

func NewClosestPeersIter[K kad.Key[K]](target K, mr *MessageRouter[K], knownClosestPeers []address.NodeID[K], numResults int, concurrency int, timeout time.Duration) *ClosestPeersIter[K] {
	iter := &ClosestPeersIter[K]{
		target:      target,
		mr:          mr,
		peerlist:    &PeerList[K]{},
		numResults:  numResults,
		concurrency: concurrency,
		timeout:     timeout,
		state:       &ClosestPeersIterStateIterating{},
	}

	trace("NewClosestPeersIter number of known closest peers=%d", len(knownClosestPeers))

	for _, node := range knownClosestPeers {
		heap.Push(iter.peerlist, &PeerInfo[K]{
			Distance: target.Xor(node.Key()),
			NodeID:   node,
			State:    &PeerStateNotContacted{},
		})
	}

	return iter
}

func (pi *ClosestPeersIter[K]) Advance(ctx context.Context) (rstate PeerIterState) {
	defer func() {
		traceCurrentState("ClosestPeersIter.Advance.exit", pi.state)
		traceReturnState("ClosestPeersIter.Advance", rstate)
	}()
	pi.mu.Lock()
	st := pi.state
	pi.mu.Unlock()
	traceCurrentState("ClosestPeersIter.Advance.entry", st)
	if _, ok := st.(*ClosestPeersIterStateFinished); ok {
		return &PeerIterStateFinished{}
	}

	successes := 0
	progressing := false

	atCapacity := pi.IsAtCapacity()

	trace("peerlist length: %d", pi.peerlist.Len())

	// peerlist is ordered by distance
	for _, p := range *pi.peerlist {
		traceCurrentState("ClosestPeersIter.Advance.peer_state", p.State)
		switch st := p.State.(type) {
		case *PeerStateWaiting:
			if time.Now().After(st.Deadline) {
				// mark peer as unresponsive
				p.State = &PeerStateUnresponsive{}
				pi.inFlight--
			} else if atCapacity {
				return &PeerIterStateWaitingAtCapacity{}
			} else {
				// The iterator is still waiting for a result from a peer so can't be considered done
				progressing = true
			}
		case *PeerStateSucceeded:
			successes++
			if !progressing && successes >= pi.numResults {
				pi.setState(&ClosestPeersIterStateFinished{})
				return &PeerIterStateFinished{}
			}

		case *PeerStateNotContacted:
			if !atCapacity {
				deadline := time.Now().Add(pi.timeout)
				p.State = &PeerStateWaiting{Deadline: deadline}
				pi.inFlight++

				// TODO: send find nodes to peer
				return &PeerIterStateWaitingMessage[K]{
					NodeID: p.NodeID,
				}

			}
			return &PeerIterStateWaitingAtCapacity{}
		case *PeerStateUnresponsive:
			// ignore
		case *PeerStateFailed:
			// ignore
		default:
			panic(fmt.Sprintf("unexpected state: %T", p.State))
		}
	}

	if pi.inFlight > 0 {
		// The iterator is still waiting for results and not at capacity
		return &PeerIterStateWaitingWithCapacity{}
	}

	// The iterator is finished because all available peers have been contacted
	// and the iterator is not waiting for any more results.
	pi.setState(&ClosestPeersIterStateFinished{})
	return &PeerIterStateFinished{}
}

func (pi *ClosestPeersIter[K]) IsAtCapacity() bool {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	switch pi.state.(type) {
	case *ClosestPeersIterStateStalled:
		// TODO: if stalled then we should contact all remaining nodes that have not already been queried
		return pi.inFlight >= pi.concurrency
	case *ClosestPeersIterStateIterating:
		return pi.inFlight >= pi.concurrency
	case *ClosestPeersIterStateFinished:
		return true
	default:
		panic(fmt.Sprintf("unexpected state: %T", pi.state))
	}
}

func (pi *ClosestPeersIter[K]) Cancel(ctx context.Context) {
	pi.setState(&ClosestPeersIterStateFinished{})
}

func (pi *ClosestPeersIter[K]) setState(st ClosestPeersIterState) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.state = st
}

// Callback for delivering the result of a successful request to a node.
func (pi *ClosestPeersIter[K]) OnMessageSuccess(ctx context.Context, node address.NodeID[K], msg message.MinKadResponseMessage[K]) {
	pi.mu.Lock()
	st := pi.state
	pi.mu.Unlock()
	if _, ok := st.(*ClosestPeersIterStateFinished); ok {
		return
	}

	for _, p := range *pi.peerlist {
		if !key.Equal(p.NodeID.Key(), node.Key()) {
			continue
		}
		switch st := p.State.(type) {
		case *PeerStateWaiting:
			pi.inFlight--
		case *PeerStateUnresponsive:

		case *PeerStateNotContacted:
			// ignore duplicate or late response
			return
		case *PeerStateFailed:
			// ignore duplicate or late response
			return
		case *PeerStateSucceeded:
			// ignore duplicate or late response
			return
		default:
			panic(fmt.Sprintf("unexpected state: %T", st))
		}

		// add closer peers to list
		for _, cn := range msg.CloserNodes() {
			trace("found closer node: %v", cn)
			if pi.peerlist.Exists(cn.NodeID()) {
				// ignore known node
				trace("ignoring closer node: %v", cn)
				continue
			}
			heap.Push(pi.peerlist, &PeerInfo[K]{
				Distance: pi.target.Xor(cn.NodeID().Key()),
				NodeID:   cn.NodeID(),
				State:    &PeerStateNotContacted{},
			})
		}
		p.State = &PeerStateSucceeded{}
	}
}

// // Callback for informing the iterator about a failed request to a peer.
// func (pi *ClosestPeersIter) onFailure(ctx context.Context, addr address.NodeAddr, err error) {
// 	if _, ok := pi.state.(*ClosestPeersIterStateFinished); ok {
// 		return
// 	}

// 	for _, p := range *pi.peerlist {
// 		if !p.Addr.NodeID().Key().Equal(addr.NodeID().Key()) {
// 			continue
// 		}

// 		// found the peer

// 		switch st := p.State.(type) {
// 		case *PeerStateWaiting:
// 			pi.inFlight--
// 			p.State = &PeerStateFailed{}
// 		case *PeerStateUnresponsive:
// 			p.State = &PeerStateFailed{}
// 		case *PeerStateNotContacted:
// 			// should not happen
// 		case *PeerStateFailed:
// 			// should not happen
// 		case *PeerStateSucceeded:
// 			// should not happen
// 		default:
// 			panic(fmt.Sprintf("unexpected state: %T", st))

// 		}
// 		return
// 	}
// }

// States for ClosestPeersIter

type ClosestPeersIterState interface {
	State
	closestPeersIterState()
}

// ClosestPeersIterStateFinished indicates the ClosestPeersIter has finished
type ClosestPeersIterStateFinished struct{}

// ClosestPeersIterStateStalled indicates the ClosestPeersIter has not made progress
// (this will be when "concurrency" consecutive successful requests have been made)
type ClosestPeersIterStateStalled struct{}

// ClosestPeersIterStateIterating indicates the ClosestPeersIter is still making progress
type ClosestPeersIterStateIterating struct{}

// closestPeersIterState() ensures that only ClosestPeersIter states can be assigned to a ClosestPeersIterState.
func (*ClosestPeersIterStateFinished) closestPeersIterState()  {}
func (*ClosestPeersIterStateStalled) closestPeersIterState()   {}
func (*ClosestPeersIterStateIterating) closestPeersIterState() {}

type PeerInfo[K kad.Key[K]] struct {
	Distance K
	State    PeerState
	NodeID   address.NodeID[K]
}

// PeerList is a list of peer infos ordered by distance. Manage using heap operations.
type PeerList[K kad.Key[K]] []*PeerInfo[K]

func (pl PeerList[K]) Len() int { return len(pl) }

func (pl PeerList[K]) Less(i, j int) bool {
	return pl[i].Distance.Compare(pl[j].Distance) < 0
}

func (pl PeerList[K]) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}

func (pl *PeerList[K]) Push(x any) {
	*pl = append(*pl, x.(*PeerInfo[K]))
}

func (pq *PeerList[K]) Pop() any {
	old := *pq
	n := len(old)
	pi := old[n-1]
	*pq = old[0 : n-1]
	return pi
}

func (pq *PeerList[K]) Exists(id address.NodeID[K]) bool {
	// slow and naieve for now
	for _, p := range *pq {
		if key.Equal(p.NodeID.Key(), id.Key()) {
			return true
		}
	}
	return false
}

type PeerState interface {
	State
	peerState()
}

// PeerStateNotContacted indicates that the peer has not been contacted yet.
type PeerStateNotContacted struct{}

// PeerStateWaiting indicates that the iterator is waiting for a response from the peer.
type PeerStateWaiting struct {
	Deadline time.Time
}

// PeerStateUnresponsive indicates that the peer did not respond within the configured timeout.
type PeerStateUnresponsive struct{}

// PeerStateFailed indicates that the attempt to contact the peer failed.
type PeerStateFailed struct{}

// PeerStateSucceeded indicates that the attempt to contact the peer succeeded.
type PeerStateSucceeded struct{}

// peerState() ensures that only peer states can be assigned to a PeerState.
func (*PeerStateNotContacted) peerState() {}
func (*PeerStateWaiting) peerState()      {}
func (*PeerStateUnresponsive) peerState() {}
func (*PeerStateFailed) peerState()       {}
func (*PeerStateSucceeded) peerState()    {}