package query

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/key/trie"
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

// StateNodeIterFinished indicates that the NodeIter has finished.
type StateNodeIterFinished struct {
	Successes int // the number of nodes successfully contacted
}

// StateNodeIterWaitingContact indicates that the NodeIter is waiting to to make contact with a node.
type StateNodeIterWaitingContact[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

// StateNodeIterWaiting indicates that the NodeIter is waiting for results from one or more nodes.
type StateNodeIterWaiting struct{}

// StateNodeIterWaitingAtCapacity indicates that the NodeIter is waiting for results and is at capacity.
type StateNodeIterWaitingAtCapacity struct{}

// StateNodeIterWaitingWithCapacity indicates that the NodeIter is waiting for results but has no further nodes to contact.
type StateNodeIterWaitingWithCapacity struct{}

// nodeIterState() ensures that only NodeIter states can be assigned to a NodeIterState.
func (*StateNodeIterFinished) nodeIterState()            {}
func (*StateNodeIterWaitingContact[K]) nodeIterState()   {}
func (*StateNodeIterWaiting) nodeIterState()             {}
func (*StateNodeIterWaitingAtCapacity) nodeIterState()   {}
func (*StateNodeIterWaitingWithCapacity) nodeIterState() {}

// NodeIter events

type NodeIterEvent interface {
	nodeIterEvent()
}

// EventNodeIterNodeNotContacted is the event that signals a NodeIter to stop processing work.
// After this event is received the NodeIter must return StateNodeIterFinished for all subsequent
// calls to Advance.
type EventNodeIterCancel struct{}

// EventNodeIterNodeNotContacted is the event that signals a NodeIter that a node was successfully contacted.
type EventNodeIterNodeContacted[K kad.Key[K]] struct {
	NodeID      kad.NodeID[K]
	CloserNodes []kad.NodeID[K]
}

// EventNodeIterNodeNotContacted is the event that signals a NodeIter that a node could not be contacted.
type EventNodeIterNodeNotContacted[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

// nodeIterEvent() ensures that only NodeIter events can be assigned to a NodeIterEvent.
func (*EventNodeIterCancel) nodeIterEvent()              {}
func (*EventNodeIterNodeContacted[K]) nodeIterEvent()    {}
func (*EventNodeIterNodeNotContacted[K]) nodeIterEvent() {}

var _ NodeIter[key.Key8] = (*ClosestNodesIter[key.Key8])(nil)

// ClosestNodesIterConfig specifies configuration options for use when creating a ClosestNodesIter
type ClosestNodesIterConfig struct {
	Concurrency int           // the maximum number of concurrent requests that may be in flight
	NumResults  int           // the minimum number of nodes to successfully contact before considering iteration complete
	NodeTimeout time.Duration // the timeout for contacting a single node
	Clock       clock.Clock   // a clock that may replaced by a mock when testing
}

// DefaultClosestNodesIterConfig returns the default configuration options for a ClosestNodesIter.
// Options may be overridden before passing to NewClosestNodesIter
func DefaultClosestNodesIterConfig() *ClosestNodesIterConfig {
	return &ClosestNodesIterConfig{
		Clock:       clock.New(), // use standard time
		Concurrency: 3,
		NumResults:  20,
		NodeTimeout: time.Minute,
	}
}

// A ClosestNodesIter iterates nodes in order of ascending distance from a key.
type ClosestNodesIter[K kad.Key[K]] struct {
	// target is the key whose distance to a node determines the position of that node in the iterator.
	target K

	// cfg is a copy of the configuration used to create the iterator
	cfg ClosestNodesIterConfig

	// nodelist holds the nodes discovered so far, ordered by increasing distance from the target.
	nodes *trie.Trie[K, *NodeInfo[K]]

	// inFlight is number of requests in flight, will be <= concurrency
	inFlight int

	// finished indicates that that the iterator has completed its work or has been stopped.
	finished bool

	// successes counts how many nodes have been successfully contacted
	successes int
}

// NewClosestNodesIter returns a new ClosestNodesIter
func NewClosestNodesIter[K kad.Key[K]](target K, knownClosestNodes []kad.NodeID[K], cfg *ClosestNodesIterConfig) *ClosestNodesIter[K] {
	if cfg == nil {
		cfg = DefaultClosestNodesIterConfig()
	}

	iter := &ClosestNodesIter[K]{
		target: target,
		cfg:    *cfg,
		nodes:  trie.New[K, *NodeInfo[K]](),
	}

	for _, node := range knownClosestNodes {
		iter.nodes.Add(node.Key(), &NodeInfo[K]{
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
		return &StateNodeIterFinished{
			Successes: pi.successes,
		}
	}

	switch tev := ev.(type) {
	case *EventNodeIterCancel:
		pi.finished = true
		return &StateNodeIterFinished{Successes: pi.successes}
	case *EventNodeIterNodeContacted[K]:
		pi.onNodeContacted(ctx, tev.NodeID, tev.CloserNodes)
	case *EventNodeIterNodeNotContacted[K]:
		pi.onNodeNotContacted(ctx, tev.NodeID)
	case nil:
		// TEMPORARY: no event to process
	default:
		panic(fmt.Sprintf("unexpected event: %T", tev))
	}

	// reset the success count to allow recalculation during loop through closest nodes
	pi.successes = 0

	// progressing is set to true if any node is still awaiting contact
	progressing := false

	// TODO: if stalled then we should contact all remaining nodes that have not already been queried
	atCapacity := func() bool {
		return pi.inFlight >= pi.cfg.Concurrency
	}

	// get all the nodes in order of distance from the target
	// TODO: turn this into a walk or iterator on trie.Trie
	entries := trie.Closest(pi.nodes, pi.target, pi.nodes.Size())
	for _, e := range entries {
		ni := e.Data
		switch st := ni.State.(type) {
		case *NodeStateWaiting:
			if pi.cfg.Clock.Now().After(st.Deadline) {
				// mark node as unresponsive
				ni.State = &NodeStateUnresponsive{}
				pi.inFlight--
			} else if atCapacity() {
				return &StateNodeIterWaitingAtCapacity{}
			} else {
				// The iterator is still waiting for a result from a node so can't be considered done
				progressing = true
			}
		case *NodeStateSucceeded:
			pi.successes++
			// The iterator has attempted to contact all nodes closer than this one.
			// If the iterator is not progressing then it doesn't expect any more nodes to be added to the list.
			// If it has contacted at least NumResults nodes successfully then the iteration is done.
			if !progressing && pi.successes >= pi.cfg.NumResults {
				pi.finished = true
				return &StateNodeIterFinished{
					Successes: pi.successes,
				}
			}

		case *NodeStateNotContacted:
			if !atCapacity() {
				deadline := pi.cfg.Clock.Now().Add(pi.cfg.NodeTimeout)
				ni.State = &NodeStateWaiting{Deadline: deadline}
				pi.inFlight++

				// TODO: send find nodes to node
				return &StateNodeIterWaitingContact[K]{
					NodeID: ni.NodeID,
				}

			}
			return &StateNodeIterWaitingAtCapacity{}
		case *NodeStateUnresponsive:
			// ignore
		case *NodeStateFailed:
			// ignore
		default:
			panic(fmt.Sprintf("unexpected state: %T", ni.State))
		}
	}

	if pi.inFlight > 0 {
		// The iterator is still waiting for results and not at capacity
		return &StateNodeIterWaitingWithCapacity{}
	}

	// The iterator is finished because all available nodes have been contacted
	// and the iterator is not waiting for any more results.
	pi.finished = true
	return &StateNodeIterFinished{
		Successes: pi.successes,
	}
}

// onNodeContacted processes the result of a successful attempt to contact a node.
func (pi *ClosestNodesIter[K]) onNodeContacted(ctx context.Context, node kad.NodeID[K], closerNodes []kad.NodeID[K]) {
	found, ni := trie.Find(pi.nodes, node.Key())
	if !found {
		// got a rogue message
		return
	}
	switch st := ni.State.(type) {
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
		pi.nodes.Add(n.Key(), &NodeInfo[K]{
			Distance: pi.target.Xor(n.Key()),
			NodeID:   n,
			State:    &NodeStateNotContacted{},
		})
	}
	ni.State = &NodeStateSucceeded{}
}

// onNodeNotContacted processes the result of a failed attempt to contact a node.
func (pi *ClosestNodesIter[K]) onNodeNotContacted(ctx context.Context, node kad.NodeID[K]) {
	found, ni := trie.Find(pi.nodes, node.Key())
	if !found {
		// got a rogue message
		return
	}
	switch st := ni.State.(type) {
	case *NodeStateWaiting:
		pi.inFlight--
	case *NodeStateUnresponsive:
		// update node state to failed
		break
	case *NodeStateNotContacted:
		// update node state to failed
		break
	case *NodeStateFailed:
		// ignore duplicate or late response
		return
	case *NodeStateSucceeded:
		// ignore duplicate or late response
		return
	default:
		panic(fmt.Sprintf("unexpected state: %T", st))
	}

	ni.State = &NodeStateFailed{}
}

// States for ClosestNodesIter

type ClosestNodesIterState interface {
	closestNodesIterState()
}

// StateClosestNodesIterFinished indicates the ClosestPeersIter has finished
type StateClosestNodesIterFinished struct{}

// StateClosestNodesIterStalled indicates the ClosestPeersIter has not made progress
// (this will be when "concurrency" consecutive successful requests have been made)
type StateClosestNodesIterStalled struct{}

// StateClosestNodesIterIterating indicates the ClosestPeersIter is still making progress
type StateClosestNodesIterIterating struct{}

// closestNodesIterState() ensures that only ClosestNodesIter states can be assigned to a ClosestNodesIterState.
func (*StateClosestNodesIterFinished) closestNodesIterState()  {}
func (*StateClosestNodesIterStalled) closestNodesIterState()   {}
func (*StateClosestNodesIterIterating) closestNodesIterState() {}
