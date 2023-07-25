package query

import (
	"time"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type NodeInfo[K kad.Key[K]] struct {
	Distance K
	State    NodeState
	NodeID   kad.NodeID[K]
}

// NodeList is a list of node infos ordered by distance. Manage using heap operations.
type NodeList[K kad.Key[K]] []*NodeInfo[K]

func (pl NodeList[K]) Len() int { return len(pl) }

func (pl NodeList[K]) Less(i, j int) bool {
	return pl[i].Distance.Compare(pl[j].Distance) < 0
}

func (pl NodeList[K]) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}

func (pl *NodeList[K]) Push(x any) {
	*pl = append(*pl, x.(*NodeInfo[K]))
}

func (pq *NodeList[K]) Pop() any {
	old := *pq
	n := len(old)
	pi := old[n-1]
	*pq = old[0 : n-1]
	return pi
}

func (pq *NodeList[K]) Exists(id kad.NodeID[K]) bool {
	// slow and naieve for now
	for _, p := range *pq {
		if key.Equal(p.NodeID.Key(), id.Key()) {
			return true
		}
	}
	return false
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
