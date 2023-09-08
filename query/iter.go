package query

import (
	"context"

	"github.com/plprobelab/go-kademlia/internal/kadtest"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/key/trie"
)

// A NodeIter iterates nodes according to some strategy.
type NodeIter[K kad.Key[K], A kad.Address[A]] interface {
	// Add adds node information to the iterator
	Add(*NodeStatus[K, A])

	// Find returns the node information corresponding to the given Kademlia key
	Find(K) (*NodeStatus[K, A], bool)

	// Each applies fn to each entry in the iterator in order. Each stops and returns true if fn returns true.
	// Otherwise Each returns false when there are no further entries.
	Each(ctx context.Context, fn func(context.Context, *NodeStatus[K, A]) bool) bool
}

// A ClosestNodesIter iterates nodes in order of ascending distance from a key.
type ClosestNodesIter[K kad.Key[K], A kad.Address[A]] struct {
	// target is the key whose distance to a node determines the position of that node in the iterator.
	target K

	// nodelist holds the nodes discovered so far, ordered by increasing distance from the target.
	nodes *trie.Trie[K, *NodeStatus[K, A]]
}

var _ NodeIter[key.Key8, kadtest.StrAddr] = (*ClosestNodesIter[key.Key8, kadtest.StrAddr])(nil)

// NewClosestNodesIter creates a new ClosestNodesIter
func NewClosestNodesIter[K kad.Key[K], A kad.Address[A]](target K) *ClosestNodesIter[K, A] {
	return &ClosestNodesIter[K, A]{
		target: target,
		nodes:  trie.New[K, *NodeStatus[K, A]](),
	}
}

func (iter *ClosestNodesIter[K, A]) Add(ni *NodeStatus[K, A]) {
	iter.nodes.Add(ni.Node.ID().Key(), ni)
}

func (iter *ClosestNodesIter[K, A]) Find(k K) (*NodeStatus[K, A], bool) {
	found, ni := trie.Find(iter.nodes, k)
	return ni, found
}

func (iter *ClosestNodesIter[K, A]) Each(ctx context.Context, fn func(context.Context, *NodeStatus[K, A]) bool) bool {
	// get all the nodes in order of distance from the target
	// TODO: turn this into a walk or iterator on trie.Trie
	entries := trie.Closest(iter.nodes, iter.target, iter.nodes.Size())
	for _, e := range entries {
		ni := e.Data
		if fn(ctx, ni) {
			return true
		}
	}
	return false
}

// A SequentialIter iterates nodes in the order they were added to the iterator.
type SequentialIter[K kad.Key[K], A kad.Address[A]] struct {
	// nodelist holds the nodes discovered so far, ordered by increasing distance from the target.
	nodes []*NodeStatus[K, A]
}

var _ NodeIter[key.Key8, kadtest.StrAddr] = (*SequentialIter[key.Key8, kadtest.StrAddr])(nil)

// NewSequentialIter creates a new SequentialIter
func NewSequentialIter[K kad.Key[K], A kad.Address[A]]() *SequentialIter[K, A] {
	return &SequentialIter[K, A]{
		nodes: make([]*NodeStatus[K, A], 0),
	}
}

func (iter *SequentialIter[K, A]) Add(ni *NodeStatus[K, A]) {
	iter.nodes = append(iter.nodes, ni)
}

// Find returns the node information corresponding to the given Kademlia key. It uses a linear
// search which makes it unsuitable for large numbers of entries.
func (iter *SequentialIter[K, A]) Find(k K) (*NodeStatus[K, A], bool) {
	for i := range iter.nodes {
		if key.Equal(k, iter.nodes[i].Node.ID().Key()) {
			return iter.nodes[i], true
		}
	}

	return nil, false
}

func (iter *SequentialIter[K, A]) Each(ctx context.Context, fn func(context.Context, *NodeStatus[K, A]) bool) bool {
	for _, ns := range iter.nodes {
		if fn(ctx, ns) {
			return true
		}
	}
	return false
}
