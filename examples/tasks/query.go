package main

import (
	"container/heap"
	"context"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/task"
)

type QueryPool struct {
	self        *FakeNode
	mr          *MessageRouter
	timeout     time.Duration
	concurrency int // the 'α' parameter defined by Kademlia
	replication int // the 'k' parameter defined by Kademlia
	queries     map[uint64]*Query
	nextID      uint64
}

func NewQueryPool(self *FakeNode, mr *MessageRouter) *QueryPool {
	return &QueryPool{
		self:        self,
		mr:          mr,
		timeout:     time.Minute,
		concurrency: 3,
		replication: 20,
		queries:     make(map[uint64]*Query),
		nextID:      1,
	}
}

var _ task.Task = (*QueryPool)(nil)

// Advance advances the state of the query pool by attempting to advance one of its queries
func (p *QueryPool) Advance(ctx context.Context) (rstate task.State) {
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
				NodeID:  st.NodeID,
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
func (qp *QueryPool) AddQuery(ctx context.Context, target key.KadKey) (uint64, error) {
	knownClosestPeers, err := qp.self.Closest(ctx, target, qp.replication)
	if err != nil {
		return 0, nil
	}

	iter := NewClosestPeersIter(target, qp.mr, knownClosestPeers, qp.replication, qp.concurrency, qp.timeout)
	id := qp.nextQueryID()
	qp.queries[id] = &Query{
		id:   id,
		iter: iter,
	}
	return id, nil
}

func (q *QueryPool) nextQueryID() uint64 {
	id := q.nextID
	q.nextID++
	return id
}

func (q *QueryPool) Cancel(context.Context) {
	panic("not implemented")
}

type Query struct {
	id    uint64
	iter  PeerIter
	stats QueryStats
}

var _ task.Task = (*Query)(nil)

// Advance advances the state of the query by attempting to advance its iterator
func (q *Query) Advance(ctx context.Context) task.State {
	state := q.iter.Advance(ctx)
	if _, ok := state.(*PeerIterStateWaiting); ok {
		q.stats.Requests++
	}
	return state
}

func (q *Query) Cancel(context.Context) {
	panic("not implemented")
}

type QueryStats struct {
	Start    time.Time
	End      time.Time
	Requests int
	Success  int
	Failure  int
}

// States

// QueryPoolIdle indicates that the pool is idle, i.e. there are no queries to process.
type QueryPoolIdle struct{}

// QueryPoolWaiting indicates that at least one query is waiting for results.
type QueryPoolWaiting struct {
	QueryID uint64
	NodeID  address.NodeID
	Stats   QueryStats
}

// QueryPoolWaitingWithCapacity indicates that at least one query is waiting for results but it is not at capacity.
type QueryPoolWaitingWithCapacity struct {
	QueryID uint64
	Stats   QueryStats
}

// QueryPoolFinished indicates that a query has finished.
type QueryPoolFinished struct {
	QueryID uint64
	Stats   QueryStats
}

// QueryPoolTimeout indicates that at a query has timed out.
type QueryPoolTimeout struct {
	QueryID uint64
	Stats   QueryStats
}

// General Peer Iterator states

// PeerIterStateFinished indicates that the PeerIter has finished.
type PeerIterStateFinished struct{}

// PeerIterStateWaiting indicates that the PeerIter is waiting for results from a specific peer.
type PeerIterStateWaiting struct {
	NodeID address.NodeID
}

// PeerIterStateWaiting indicates that the PeerIter is waiting for results and is at capacity.
type PeerIterStateWaitingAtCapacity struct{}

// PeerIterStateWaiting indicates that the PeerIter is waiting for results but has no further peers to contact.
type PeerIterStateWaitingWithCapacity struct{}

// A PeerIter iterates peers according to some strategy.
type PeerIter interface {
	task.Task
}

type ClosestPeersIter struct {
	// The target whose distance to any peer determines the position of the peer in the iterator.
	target key.KadKey

	mr *MessageRouter

	// current state of the iterator
	state task.State

	// The closest peers to the target, ordered by increasing distance.
	peerlist *PeerList

	// Number of peers to search for.
	numResults int

	// Maximum number of concurrent requests that may be in flight.
	concurrency int

	// Timeout for contacting a single peer
	timeout time.Duration

	// number of requests in flight, will be <= concurrency
	inFlight int
}

func NewClosestPeersIter(target key.KadKey, mr *MessageRouter, knownClosestPeers []address.NodeAddr, numResults int, concurrency int, timeout time.Duration) *ClosestPeersIter {
	iter := &ClosestPeersIter{
		target:      target,
		mr:          mr,
		peerlist:    &PeerList{},
		numResults:  numResults,
		concurrency: concurrency,
		timeout:     timeout,
		state:       &ClosestPeersIterStateIterating{},
	}

	trace("NewClosestPeersIter number of known closest peers=%d", len(knownClosestPeers))

	for _, addr := range knownClosestPeers {
		heap.Push(iter.peerlist, &PeerInfo{
			Distance: target.Xor(addr.NodeID().Key()),
			Addr:     addr,
			State:    &PeerStateNotContacted{},
		})
	}

	return iter
}

func (pi *ClosestPeersIter) Advance(ctx context.Context) (rstate task.State) {
	defer func() {
		traceCurrentState("ClosestPeersIter.Advance.exit", pi.state)
		traceReturnState("ClosestPeersIter.Advance", rstate)
	}()
	traceCurrentState("ClosestPeersIter.Advance.entry", pi.state)
	if _, ok := pi.state.(*ClosestPeersIterStateFinished); ok {
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
				pi.state = &ClosestPeersIterStateFinished{}
				return &PeerIterStateFinished{}
			}

		case *PeerStateNotContacted:
			if !atCapacity {
				deadline := time.Now().Add(pi.timeout)
				p.State = &PeerStateWaiting{Deadline: deadline}
				pi.inFlight++

				pi.mr.SendMessage(ctx, p.Addr, &FindNodeRequest{Key: pi.target}, pi.onSuccess, pi.onFailure)

				// TODO: send find nodes to peer
				return &PeerIterStateWaiting{}

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
	pi.state = &ClosestPeersIterStateFinished{}
	return &PeerIterStateFinished{}
}

func (pi *ClosestPeersIter) IsAtCapacity() bool {
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

func (pi *ClosestPeersIter) Cancel(ctx context.Context) {
	panic("not implemented")
}

// Callback for delivering the result of a successful request to a peer.
func (pi *ClosestPeersIter) onSuccess(ctx context.Context, addr address.NodeAddr, msg message.MinKadResponseMessage) {
	if _, ok := pi.state.(*ClosestPeersIterStateFinished); ok {
		return
	}

	switch tmsg := msg.(type) {
	case *FindNodeResponse:
		for _, p := range *pi.peerlist {
			if !p.Addr.NodeID().Key().Equal(addr.NodeID().Key()) {
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
			for _, cp := range tmsg.CloserPeers {
				trace("found closer peer: %v", cp)
				if pi.peerlist.Exists(cp.NodeID()) {
					// ignore known peers
					trace("ignoring closer peer: %v", cp)
					continue
				}
				heap.Push(pi.peerlist, &PeerInfo{
					Distance: pi.target.Xor(cp.NodeID().Key()),
					Addr:     cp,
					State:    &PeerStateNotContacted{},
				})
			}
			p.State = &PeerStateSucceeded{}
		}
	default:
		// ignore unknown message

	}
}

// Callback for informing the iterator about a failed request to a peer.
func (pi *ClosestPeersIter) onFailure(ctx context.Context, addr address.NodeAddr, err error) {
	if _, ok := pi.state.(*ClosestPeersIterStateFinished); ok {
		return
	}

	for _, p := range *pi.peerlist {
		if !p.Addr.NodeID().Key().Equal(addr.NodeID().Key()) {
			continue
		}

		// found the peer

		switch st := p.State.(type) {
		case *PeerStateWaiting:
			pi.inFlight--
			p.State = &PeerStateFailed{}
		case *PeerStateUnresponsive:
			p.State = &PeerStateFailed{}
		case *PeerStateNotContacted:
			// should not happen
		case *PeerStateFailed:
			// should not happen
		case *PeerStateSucceeded:
			// should not happen
		default:
			panic(fmt.Sprintf("unexpected state: %T", st))

		}
		return
	}
}

// States for ClosestPeersIter

// ClosestPeersIterStateFinished indicates the ClosestPeersIter has finished
type ClosestPeersIterStateFinished struct{}

// ClosestPeersIterStateStalled indicates the ClosestPeersIter has not made progress
// (this will be when "concurrency" consecutive successful requests have been made)
type ClosestPeersIterStateStalled struct{}

// ClosestPeersIterStateIterating indicates the ClosestPeersIter is still making progress
type ClosestPeersIterStateIterating struct{}

type PeerInfo struct {
	Distance key.KadKey
	State    task.State
	Addr     address.NodeAddr
}

// PeerList is a list of peer infos ordered by distance. Manage using heap operations.
type PeerList []*PeerInfo

func (pl PeerList) Len() int { return len(pl) }

func (pl PeerList) Less(i, j int) bool {
	return pl[i].Distance.Compare(pl[j].Distance) < 0
}

func (pl PeerList) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}

func (pl *PeerList) Push(x any) {
	*pl = append(*pl, x.(*PeerInfo))
}

func (pq *PeerList) Pop() any {
	old := *pq
	n := len(old)
	pi := old[n-1]
	*pq = old[0 : n-1]
	return pi
}

func (pq *PeerList) Exists(id address.NodeID) bool {
	// slow and naieve for now
	for _, p := range *pq {
		if p.Addr.NodeID().Key().Equal(id.Key()) {
			return true
		}
	}
	return false
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
