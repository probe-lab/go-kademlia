package query

import (
	"time"

	"github.com/plprobelab/go-kademlia/kad"
)

type nodeInfo[K kad.Key[K]] struct {
	Distance K
	State    nodeState
	NodeID   kad.NodeID[K]
}

type nodeState interface {
	nodeState()
}

// stateNodeNotContacted indicates that the node has not been contacted yet.
type stateNodeNotContacted struct{}

// stateNodeWaiting indicates that the iterator is waiting for a response from the node.
type stateNodeWaiting struct {
	Deadline time.Time
}

// stateNodeUnresponsive indicates that the node did not respond within the configured timeout.
type stateNodeUnresponsive struct{}

// stateNodeFailed indicates that the attempt to contact the node failed.
type stateNodeFailed struct{}

// stateNodeSucceeded indicates that the attempt to contact the node succeeded.
type stateNodeSucceeded struct{}

// nodeState() ensures that only node states can be assigned to a nodeState interface.
func (*stateNodeNotContacted) nodeState() {}
func (*stateNodeWaiting) nodeState()      {}
func (*stateNodeUnresponsive) nodeState() {}
func (*stateNodeFailed) nodeState()       {}
func (*stateNodeSucceeded) nodeState()    {}
