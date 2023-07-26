package triert

import (
	"context"
	"fmt"

	"github.com/plprobelab/go-kademlia/key"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key/trie"
)

// TrieRT is a routing table backed by a XOR Trie which offers good scalablity and performance
// for large networks.
type TrieRT[K kad.Key[K]] struct {
	self      K
	keyFilter KeyFilterFunc[K]

	keys *trie.Trie[K, kad.NodeID[K]]
}

var _ kad.RoutingTable[key.Key256] = (*TrieRT[key.Key256])(nil)

// New creates a new TrieRT using the supplied key as the local node's Kademlia key.
// If cfg is nil, the default config is used.
func New[K kad.Key[K]](self K, cfg *Config[K]) (*TrieRT[K], error) {
	rt := &TrieRT[K]{
		self: self,
		keys: &trie.Trie[K, kad.NodeID[K]]{},
	}
	if err := rt.apply(cfg); err != nil {
		return nil, fmt.Errorf("apply config: %w", err)
	}
	return rt, nil
}

func (rt *TrieRT[K]) apply(cfg *Config[K]) error {
	if cfg == nil {
		cfg = DefaultConfig[K]()
	}

	rt.keyFilter = cfg.KeyFilter

	return nil
}

// Self returns the local node's Kademlia key.
func (rt *TrieRT[K]) Self() K {
	return rt.self
}

// AddNode tries to add a peer to the routing table.
func (rt *TrieRT[K]) AddNode(node kad.NodeID[K]) bool {
	kk := node.Key()
	if rt.keyFilter != nil && !rt.keyFilter(rt, kk) {
		return false
	}

	return rt.keys.Add(kk, node)
}

// RemoveKey tries to remove a peer identified by its Kademlia key from the
// routing table. It returns true if the key was found to be present in the table and was removed.
func (rt *TrieRT[K]) RemoveKey(kk K) bool {
	return rt.keys.Remove(kk)
}

// NearestNodes returns the n closest peers to a given key.
func (rt *TrieRT[K]) NearestNodes(kk K, n int) []kad.NodeID[K] {
	closestEntries := closestAtDepth(kk, rt.keys, 0, n)
	if len(closestEntries) == 0 {
		return []kad.NodeID[K]{}
	}

	nodes := make([]kad.NodeID[K], 0, len(closestEntries))
	for _, c := range closestEntries {
		nodes = append(nodes, c.data)
	}

	return nodes
}

type entry[K kad.Key[K]] struct {
	key  K
	data kad.NodeID[K]
}

func closestAtDepth[K kad.Key[K]](kk K, t *trie.Trie[K, kad.NodeID[K]], depth int, n int) []entry[K] {
	if t.IsLeaf() {
		if t.HasKey() {
			// We've found a leaf
			return []entry[K]{
				{key: *t.Key(), data: t.Data()},
			}
		}
		// We've found an empty node?
		return nil
	}

	if depth > kk.BitLen() {
		return nil
	}

	// Find the closest direction.
	dir := int(kk.Bit(depth))
	// Add peers from the closest direction first
	found := closestAtDepth(kk, t.Branch(dir), depth+1, n)
	if len(found) == n {
		return found
	}
	// Didn't find enough peers in the closest direction, try the other direction.
	return append(found, closestAtDepth(kk, t.Branch(1-dir), depth+1, n-len(found))...)
}

func (rt *TrieRT[K]) Find(ctx context.Context, kk K) (kad.NodeID[K], error) {
	found, node := trie.Find(rt.keys, kk)
	if found {
		return node, nil
	}

	return nil, nil
}

// Size returns the number of peers contained in the table.
func (rt *TrieRT[K]) Size() int {
	return rt.keys.Size()
}

// Cpl returns the longest common prefix length the supplied key shares with the table's key.
func (rt *TrieRT[K]) Cpl(kk K) int {
	return rt.self.CommonPrefixLength(kk)
}

// CplSize returns the number of peers in the table whose longest common prefix with the table's key is of length cpl.
func (rt *TrieRT[K]) CplSize(cpl int) int {
	n, err := countCpl(rt.keys, rt.self, cpl, 0)
	if err != nil {
		return 0
	}
	return n
}

func countCpl[K kad.Key[K]](t *trie.Trie[K, kad.NodeID[K]], kk K, cpl int, depth int) (int, error) {
	// special cases for very small tables where keys may be placed higher in the trie due to low population
	if t.IsLeaf() {
		if !t.HasKey() {
			return 0, nil
		}
		keyCpl := kk.CommonPrefixLength(*t.Key())
		if keyCpl == cpl {
			return 1, nil
		}
		return 0, nil
	}

	if depth > kk.BitLen() {
		return 0, nil
	}

	if depth == cpl {
		// return the number of entries that do not share the next bit with kk
		return t.Branch(1 - int(kk.Bit(depth))).Size(), nil
	}

	return countCpl(t.Branch(int(kk.Bit(depth))), kk, cpl, depth+1)
}
