package query

import (
	"time"

	"github.com/probe-lab/go-kademlia/kad"
)

type NodeStatus[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
	State  NodeState
}

type NodeState interface {
	nodeState()
}

// StateNodeNotContacted indicates that the node has not been contacted yet.
type StateNodeNotContacted struct{}

// StateNodeWaiting indicates that a query is waiting for a response from the node.
type StateNodeWaiting struct {
	Deadline time.Time
}

// StateNodeUnresponsive indicates that the node did not respond within the configured timeout.
type StateNodeUnresponsive struct{}

// StateNodeFailed indicates that the attempt to contact the node failed.
type StateNodeFailed struct{}

// StateNodeSucceeded indicates that the attempt to contact the node succeeded.
type StateNodeSucceeded struct{}

// nodeState() ensures that only node states can be assigned to a nodeState interface.
func (*StateNodeNotContacted) nodeState() {}
func (*StateNodeWaiting) nodeState()      {}
func (*StateNodeUnresponsive) nodeState() {}
func (*StateNodeFailed) nodeState()       {}
func (*StateNodeSucceeded) nodeState()    {}
