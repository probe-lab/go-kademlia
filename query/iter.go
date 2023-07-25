package query

import (
	"container/heap"
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/util"
)

// A NodeIter iterates nodes according to some strategy.
type NodeIter[K kad.Key[K]] interface {
	// Advance signals the iterator to advance its work to the next state.
	Advance(context.Context, NodeIterEvent) NodeIterState
}

// NodeIter states

type NodeIterState interface {
	nodeIterState()
}

// NodeIterStateFinished indicates that the NodeIter has finished.
type NodeIterStateFinished[K kad.Key[K]] struct {
	Nodes []kad.NodeID[K]
}

// NodeIterStateWaitingContact indicates that the NodeIter is waiting to to make contact with a node.
type NodeIterStateWaitingContact[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

// NodeIterStateWaiting indicates that the NodeIter is waiting for results from one or more nodes.
type NodeIterStateWaiting struct{}

// NodeIterStateWaitingAtCapacity indicates that the NodeIter is waiting for results and is at capacity.
type NodeIterStateWaitingAtCapacity struct{}

// NodeIterStateWaitingWithCapacity indicates that the NodeIter is waiting for results but has no further nodes to contact.
type NodeIterStateWaitingWithCapacity struct{}

// nodeIterState() ensures that only NodeIter states can be assigned to a NodeIterState.
func (*NodeIterStateFinished[K]) nodeIterState()         {}
func (*NodeIterStateWaitingContact[K]) nodeIterState()   {}
func (*NodeIterStateWaiting) nodeIterState()             {}
func (*NodeIterStateWaitingAtCapacity) nodeIterState()   {}
func (*NodeIterStateWaitingWithCapacity) nodeIterState() {}

// NodeIter events

type NodeIterEvent interface {
	nodeIterEvent()
}

type NodeIterEventCancel struct{}

type NodeIterEventNodeContacted[K kad.Key[K]] struct {
	NodeID      kad.NodeID[K]
	CloserNodes []kad.NodeID[K]
}

// nodeIterEvent() ensures that only NodeIter events can be assigned to a NodeIterEvent.
func (*NodeIterEventCancel) nodeIterEvent()           {}
func (*NodeIterEventNodeContacted[K]) nodeIterEvent() {}

var _ NodeIter[key.Key8] = (*ClosestNodesIter[key.Key8])(nil)

// A ClosestNodesIter iterates nodes in order of ascending distance from a key.
type ClosestNodesIter[K kad.Key[K]] struct {
	// target is the key whose distance to a node determines the position of that node in the iterator.
	target K

	// clk holds a clock that may replaced by a mock when testing
	clk clock.Clock

	// nodelist holds the nodes discovered so far, ordered by increasing distance from the target.
	nodelist *NodeList[K]

	// numResults is the number of nodes to search for.
	numResults int

	// concurrency is the maximum number of concurrent requests that may be in flight.
	concurrency int

	// timeout is the timeout for contacting a single nodes
	timeout time.Duration

	// inFlight is number of requests in flight, will be <= concurrency
	inFlight int

	// finished indicates that that the iterator has completed its work or has been stopped.
	finished bool
}

// NewClosestNodesIter returns a new ClosestNodesIter
func NewClosestNodesIter[K kad.Key[K]](target K, knownClosestNodes []kad.NodeID[K], numResults int, concurrency int, timeout time.Duration, clk clock.Clock) *ClosestNodesIter[K] {
	iter := &ClosestNodesIter[K]{
		target:      target,
		clk:         clk,
		nodelist:    &NodeList[K]{},
		numResults:  numResults,
		concurrency: concurrency,
		timeout:     timeout,
	}

	for _, node := range knownClosestNodes {
		heap.Push(iter.nodelist, &NodeInfo[K]{
			Distance: target.Xor(node.Key()),
			NodeID:   node,
			State:    &NodeStateNotContacted{},
		})
	}

	return iter
}

func (pi *ClosestNodesIter[K]) Advance(ctx context.Context, ev NodeIterEvent) (rstate NodeIterState) {
	ctx, span := util.StartSpan(ctx, "ClosestNodesIter.Advance")
	defer span.End()
	if pi.finished {
		return &NodeIterStateFinished[K]{
			Nodes: pi.results(ctx),
		}
	}

	switch tev := ev.(type) {
	case *NodeIterEventCancel:
		pi.finished = true
		return &NodeIterStateFinished[K]{
			Nodes: pi.results(ctx),
		}
	case *NodeIterEventNodeContacted[K]:
		pi.onMessageSuccess(ctx, tev.NodeID, tev.CloserNodes)
	case nil:
		// TEMPORARY: no event to process
	default:
		panic(fmt.Sprintf("unexpected event: %T", tev))
	}

	successes := 0
	progressing := false

	// TODO: if stalled then we should contact all remaining nodes that have not already been queried
	atCapacity := pi.inFlight >= pi.concurrency

	// nodelist is ordered by distance
	for _, p := range *pi.nodelist {
		switch st := p.State.(type) {
		case *NodeStateWaiting:
			if pi.clk.Now().After(st.Deadline) {
				// mark node as unresponsive
				p.State = &NodeStateUnresponsive{}
				pi.inFlight--
			} else if atCapacity {
				return &NodeIterStateWaitingAtCapacity{}
			} else {
				// The iterator is still waiting for a result from a node so can't be considered done
				progressing = true
			}
		case *NodeStateSucceeded:
			successes++
			if !progressing && successes >= pi.numResults {
				pi.finished = true
				return &NodeIterStateFinished[K]{
					Nodes: pi.results(ctx),
				}
			}

		case *NodeStateNotContacted:
			if !atCapacity {
				deadline := time.Now().Add(pi.timeout)
				p.State = &NodeStateWaiting{Deadline: deadline}
				pi.inFlight++

				// TODO: send find nodes to node
				return &NodeIterStateWaitingContact[K]{
					NodeID: p.NodeID,
				}

			}
			return &NodeIterStateWaitingAtCapacity{}
		case *NodeStateUnresponsive:
			// ignore
		case *NodeStateFailed:
			// ignore
		default:
			panic(fmt.Sprintf("unexpected state: %T", p.State))
		}
	}

	if pi.inFlight > 0 {
		// The iterator is still waiting for results and not at capacity
		return &NodeIterStateWaitingWithCapacity{}
	}

	// The iterator is finished because all available nodes have been contacted
	// and the iterator is not waiting for any more results.
	pi.finished = true
	return &NodeIterStateFinished[K]{
		Nodes: pi.results(ctx),
	}
}

// Callback for delivering the result of a successful request to a node.
func (pi *ClosestNodesIter[K]) onMessageSuccess(ctx context.Context, node kad.NodeID[K], closerNodes []kad.NodeID[K]) {
	for _, n := range *pi.nodelist {
		if !key.Equal(n.NodeID.Key(), node.Key()) {
			continue
		}
		switch st := n.State.(type) {
		case *NodeStateWaiting:
			pi.inFlight--
		case *NodeStateUnresponsive:

		case *NodeStateNotContacted:
			// ignore duplicate or late response
			return
		case *NodeStateFailed:
			// ignore duplicate or late response
			return
		case *NodeStateSucceeded:
			// ignore duplicate or late response
			return
		default:
			panic(fmt.Sprintf("unexpected state: %T", st))
		}

		// add closer nodes to list
		for _, n := range closerNodes {
			if pi.nodelist.Exists(n) {
				// ignore known node
				continue
			}
			heap.Push(pi.nodelist, &NodeInfo[K]{
				Distance: pi.target.Xor(n.Key()),
				NodeID:   n,
				State:    &NodeStateNotContacted{},
			})
		}
		n.State = &NodeStateSucceeded{}
	}
}

func (pi *ClosestNodesIter[K]) results(ctx context.Context) []kad.NodeID[K] {
	nodes := make([]kad.NodeID[K], 0, pi.numResults)
	for _, n := range *pi.nodelist {
		if _, ok := n.State.(*NodeStateSucceeded); !ok {
			continue
		}
		nodes = append(nodes, n.NodeID)
		if len(nodes) >= pi.numResults {
			break
		}
	}
	return nodes
}

// States for ClosestNodesIter

type ClosestNodesIterState interface {
	closestNodesIterState()
}

// ClosestNodesIterStateFinished indicates the ClosestPeersIter has finished
type ClosestNodesIterStateFinished struct{}

// ClosestNodesIterStateStalled indicates the ClosestPeersIter has not made progress
// (this will be when "concurrency" consecutive successful requests have been made)
type ClosestNodesIterStateStalled struct{}

// ClosestNodesIterStateIterating indicates the ClosestPeersIter is still making progress
type ClosestNodesIterStateIterating struct{}

// closestNodesIterState() ensures that only ClosestNodesIter states can be assigned to a ClosestNodesIterState.
func (*ClosestNodesIterStateFinished) closestNodesIterState()  {}
func (*ClosestNodesIterStateStalled) closestNodesIterState()   {}
func (*ClosestNodesIterStateIterating) closestNodesIterState() {}
