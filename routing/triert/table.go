package triert

import (
	"context"
	"fmt"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/key/trie"
	"github.com/plprobelab/go-kademlia/network/address"
)

// TrieRT is a routing table backed by a XOR Trie which offers good scalablity and performance
// for large networks.
type TrieRT struct {
	self      key.KadKey
	keyFilter KeyFilterFunc

	keys *nodeTrie
}

type nodeTrie = trie.Trie[address.NodeID]

// New creates a new TrieRT using the supplied key as the local node's Kademlia key.
// If cfg is nil, the default config is used.
func New(self key.KadKey, cfg *Config) (*TrieRT, error) {
	rt := &TrieRT{
		self: self,
		keys: &nodeTrie{},
	}
	if err := rt.apply(cfg); err != nil {
		return nil, fmt.Errorf("apply config: %w", err)
	}
	return rt, nil
}

func (rt *TrieRT) apply(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	rt.keyFilter = cfg.KeyFilter

	return nil
}

// Self returns the local node's Kademlia key.
func (rt *TrieRT) Self() key.KadKey {
	return rt.self
}

// AddPeer tries to add a peer to the routing table.
func (rt *TrieRT) AddPeer(ctx context.Context, node address.NodeID) (bool, error) {
	kk := node.Key()
	if kk.Size() != rt.self.Size() {
		return false, trie.ErrMismatchedKeyLength
	}

	if rt.keyFilter != nil && !rt.keyFilter(rt, kk) {
		return false, nil
	}

	return rt.keys.Add(kk, node)
}

// RemoveKey tries to remove a peer identified by its Kademlia key from the
// routing table. It returns true if the key was found to be present in the table and was removed.
func (rt *TrieRT) RemoveKey(ctx context.Context, kk key.KadKey) (bool, error) {
	if kk.Size() != rt.self.Size() {
		return false, trie.ErrMismatchedKeyLength
	}
	return rt.keys.Remove(kk)
}

// NearestPeers returns the n closest peers to a given key.
func (rt *TrieRT) NearestPeers(ctx context.Context, kk key.KadKey, n int) ([]address.NodeID, error) {
	if kk.Size() != rt.self.Size() {
		return nil, trie.ErrMismatchedKeyLength
	}

	closestEntries := closestAtDepth(kk, rt.keys, 0, n)
	if len(closestEntries) == 0 {
		return []address.NodeID{}, nil
	}

	nodes := make([]address.NodeID, 0, len(closestEntries))
	for _, c := range closestEntries {
		nodes = append(nodes, c.data)
	}

	return nodes, nil
}

type entry struct {
	key  key.KadKey
	data address.NodeID
}

func closestAtDepth(kk key.KadKey, t *nodeTrie, depth int, n int) []entry {
	if t.IsLeaf() {
		if t.HasKey() {
			// We've found a leaf
			return []entry{
				{key: t.Key(), data: t.Data()},
			}
		}
		// We've found an empty node?
		return nil
	}

	if depth > kk.BitLen() {
		return nil
	}

	// Find the closest direction.
	dir := kk.BitAt(depth)
	// Add peers from the closest direction first
	found := closestAtDepth(kk, t.Branch(dir), depth+1, n)
	if len(found) == n {
		return found
	}
	// Didn't find enough peers in the closest direction, try the other direction.
	return append(found, closestAtDepth(kk, t.Branch(1-dir), depth+1, n-len(found))...)
}

func (rt *TrieRT) Find(ctx context.Context, kk key.KadKey) (address.NodeID, error) {
	if kk.Size() != rt.self.Size() {
		return nil, trie.ErrMismatchedKeyLength
	}

	found, node := trie.Find(rt.keys, kk)
	if found {
		return node, nil
	}

	return nil, nil
}

// Size returns the number of peers contained in the table.
func (rt *TrieRT) Size() int {
	return rt.keys.Size()
}

// Cpl returns the longest common prefix length the supplied key shares with the table's key.
func (rt *TrieRT) Cpl(kk key.KadKey) int {
	return rt.self.CommonPrefixLength(kk)
}

// CplSize returns the number of peers in the table whose longest common prefix with the table's key is of length cpl.
func (rt *TrieRT) CplSize(cpl int) int {
	n, err := countCpl(rt.keys, rt.self, cpl, 0)
	if err != nil {
		return 0
	}
	return n
}

func countCpl(t *nodeTrie, kk key.KadKey, cpl int, depth int) (int, error) {
	// special cases for very small tables where keys may be placed higher in the trie due to low population
	if t.IsLeaf() {
		if t.HasKey() {
			return 0, nil
		}
		keyCpl := kk.CommonPrefixLength(key.KadKey(t.Key()))
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
		return t.Branch(1 - kk.BitAt(depth)).Size(), nil
	}

	return countCpl(t.Branch(kk.BitAt(depth)), kk, cpl, depth+1)
}
