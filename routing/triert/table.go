package triert

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/key/trie"
	"github.com/plprobelab/go-kademlia/network/address"
)

// TrieRT is a routing table backed by a Xor Trie which offers good scalablity and performance
// for large networks. All exported methods are safe for concurrent use.
type TrieRT struct {
	self key.KadKey

	// keymu is held during trie mutations to serialize changes
	keymu sync.Mutex

	// keys holds a pointer to an immutable trie
	// any store to keys must be performed while holding keymu, loads may be performed without the lock
	keys atomic.Value
}

type nodeTrie = trie.Trie[address.NodeID]

func New(self key.KadKey) *TrieRT {
	rt := &TrieRT{
		self: self,
	}
	rt.keys.Store(&nodeTrie{})
	return rt
}

func (rt *TrieRT) Self() key.KadKey {
	return rt.self
}

// AddPeer tries to add a peer to the routing table
func (rt *TrieRT) AddPeer(ctx context.Context, node address.NodeID) (bool, error) {
	kk := node.Key()
	if kk.Size() != rt.self.Size() {
		return false, trie.ErrMismatchedKeyLength
	}

	rt.keymu.Lock()
	defer rt.keymu.Unlock()
	// load the old trie, derive a mutated variant and store it in place of the original
	keys := rt.keys.Load().(*nodeTrie)
	keysNext, err := trie.Add(keys, kk, node)
	if err != nil {
		return false, err
	}

	// if trie is unchanged then we didn't add the key
	if keysNext == keys {
		return false, nil
	}
	rt.keys.Store(keysNext)
	return true, nil
}

// RemoveKey tries to remove a peer identified by its Kademlia key from the
// routing table
func (rt *TrieRT) RemoveKey(ctx context.Context, kk key.KadKey) (bool, error) {
	if kk.Size() != rt.self.Size() {
		return false, trie.ErrMismatchedKeyLength
	}
	rt.keymu.Lock()
	defer rt.keymu.Unlock()
	// load the old trie, derive a mutated variant and store it in place of the original
	keys := rt.keys.Load().(*nodeTrie)
	keysNext, err := trie.Remove(keys, kk)
	if err != nil {
		return false, err
	}
	// if trie is unchanged then we didn't remove the key
	if keysNext == keys {
		return false, nil
	}

	rt.keys.Store(keysNext)
	return true, nil
}

// NearestPeers returns the n closest peers to a given key
func (rt *TrieRT) NearestPeers(ctx context.Context, kk key.KadKey, n int) ([]address.NodeID, error) {
	if kk.Size() != rt.self.Size() {
		return nil, trie.ErrMismatchedKeyLength
	}

	keys := rt.keys.Load().(*nodeTrie)
	closestEntries := closestAtDepth(kk, keys, 0, n)
	if len(closestEntries) == 0 {
		return []address.NodeID{}, nil
	}

	nodes := make([]address.NodeID, 0, len(closestEntries))
	for _, c := range closestEntries {
		nodes = append(nodes, c.Data)
	}

	return nodes, nil
}

type entry struct {
	Key  key.KadKey
	Data address.NodeID
}

func closestAtDepth(kk key.KadKey, t *nodeTrie, depth int, n int) []entry {
	if t.IsLeaf() {
		if t.HasKey() {
			// We've found a leaf
			return []entry{
				{Key: t.Key(), Data: t.Data()},
			}
		}
		// We've found an empty node?
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

	keys := rt.keys.Load().(*nodeTrie)

	found, node := trie.Find(keys, kk)

	if found {
		return node, nil
	}

	return nil, nil
}

// Size returns the number of peers contained in the table.
func (rt *TrieRT) Size() int {
	keys := rt.keys.Load().(*nodeTrie)
	return keys.Size()
}

// CplSize returns the number of peers in the table whose longest common prefix with the table's key is of length cpl.
func (rt *TrieRT) CplSize(cpl int) int {
	keys := rt.keys.Load().(*nodeTrie)
	n, err := countCpl(keys, rt.self, cpl, 0)
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

	if depth == cpl {
		// return the number of entries that do not share the next bit with kk
		return t.Branch(1 - kk.BitAt(depth)).Size(), nil
	}

	return countCpl(t.Branch(kk.BitAt(depth)), kk, cpl, depth+1)
}
