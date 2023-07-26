package query

import (
	"time"

	"github.com/plprobelab/go-kademlia/kad"
)

type NodeInfo[K kad.Key[K]] struct {
	Distance K
	State    NodeState
	NodeID   kad.NodeID[K]
}

type NodeState interface {
	nodeState()
}

// NodeStateNotContacted indicates that the peer has not been contacted yet.
type NodeStateNotContacted struct{}

// NodeStateWaiting indicates that the iterator is waiting for a response from the peer.
type NodeStateWaiting struct {
	Deadline time.Time
}

// NodeStateUnresponsive indicates that the peer did not respond within the configured timeout.
type NodeStateUnresponsive struct{}

// NodeStateFailed indicates that the attempt to contact the peer failed.
type NodeStateFailed struct{}

// NodeStateSucceeded indicates that the attempt to contact the peer succeeded.
type NodeStateSucceeded struct{}

// nodeState() ensures that only node states can be assigned to a NodeState.
func (*NodeStateNotContacted) nodeState() {}
func (*NodeStateWaiting) nodeState()      {}
func (*NodeStateUnresponsive) nodeState() {}
func (*NodeStateFailed) nodeState()       {}
func (*NodeStateSucceeded) nodeState()    {}
