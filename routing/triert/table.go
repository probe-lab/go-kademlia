package triert

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	// "github.com/libp2p/go-libp2p-xor/kademlia"
	tkey "github.com/libp2p/go-libp2p-xor/key"
	"github.com/libp2p/go-libp2p-xor/trie"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

// TODO: move to key package and replace ErrInvalidKey
// ErrKeyWrongLength indicates a key has the wrong length
var ErrKeyWrongLength = errors.New("key has wrong length")

// TrieRT is a routing table backed by a Xor Trie which offers good scalablity and performance
// for large networks. All exported methods are safe for concurrent use.
type TrieRT struct {
	self key.KadKey

	// keymu is held during keys and keyNodes mutations to serialize changes
	keymu sync.Mutex

	// keys holds a pointer to an immutable trie
	// any store to keys must be performed while holding keymu, loads may be performed without the lock
	keys atomic.Value

	// keyNodes holds a mapping of kademlia key to node id (map[string]address.NodeID)
	// note: this could be eliminated if the trie was modified to allow a tuple to be stored at each node
	// any store to keyNodes must be performed while holding keymu, loads may be performed without the lock
	keyNodes atomic.Value
}

func New(self key.KadKey) *TrieRT {
	rt := &TrieRT{
		self: self,
	}
	rt.keys.Store(&trie.Trie{})
	rt.keyNodes.Store(map[string]address.NodeID{})
	return rt
}

func (rt *TrieRT) Self() key.KadKey {
	return rt.self
}

// AddPeer tries to add a peer to the routing table
func (rt *TrieRT) AddPeer(ctx context.Context, node address.NodeID) (bool, error) {
	kk := node.Key()
	if kk.Size() != rt.self.Size() {
		return false, ErrKeyWrongLength
	}

	rt.keymu.Lock()
	defer rt.keymu.Unlock()
	// load the old trie, derive a mutated variant and store it in place of the original
	keys := rt.keys.Load().(*trie.Trie)
	keysNext := trie.Add(keys, tkey.Key(kk))

	// if trie is unchanged then we didn't add the key
	if keysNext == keys {
		return false, nil
	}

	// make a copy of keyNodes
	// could avoid this if we held key/nodeid tuple in the trie
	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID)
	keyNodesNext := make(map[string]address.NodeID, len(keyNodes)+1)
	for k, v := range keyNodes {
		keyNodesNext[k] = v
	}
	keyNodesNext[string(kk)] = node

	rt.keyNodes.Store(keyNodesNext)
	rt.keys.Store(keysNext)
	return true, nil
}

// RemoveKey tries to remove a peer identified by its Kademlia key from the
// routing table
func (rt *TrieRT) RemoveKey(ctx context.Context, kk key.KadKey) (bool, error) {
	if kk.Size() != rt.self.Size() {
		return false, ErrKeyWrongLength
	}
	rt.keymu.Lock()
	defer rt.keymu.Unlock()
	// load the old trie, derive a mutated variant and store it in place of the original
	keys := rt.keys.Load().(*trie.Trie)
	keysNext := trie.Remove(keys, tkey.Key(kk))

	// if trie is unchanged then we didn't remove the key
	if keysNext == keys {
		return false, nil
	}

	// make a copy of keyNodes without removed key
	// could avoid this if we held key/nodeid tuple in the trie
	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID)
	if _, exists := keyNodes[string(kk)]; exists {
		keyNodesNext := make(map[string]address.NodeID, len(keyNodes))
		for k, v := range keyNodes {
			if k == string(kk) {
				continue
			}
			keyNodesNext[k] = v
		}
		rt.keyNodes.Store(keyNodesNext)
	}

	rt.keys.Store(keysNext)
	return true, nil
}

// NearestPeers returns the n closest peers to a given key
func (rt *TrieRT) NearestPeers(ctx context.Context, kk key.KadKey, n int) ([]address.NodeID, error) {
	if kk.Size() != rt.self.Size() {
		return nil, ErrKeyWrongLength
	}

	keys := rt.keys.Load().(*trie.Trie)
	closestKeys, err := closestAtDepth(ctx, tkey.Key(kk), keys, 0, n, make([]tkey.Key, 0, n))
	if err != nil {
		return nil, err
	}
	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID)

	nodes := make([]address.NodeID, 0, len(closestKeys))
	for _, c := range closestKeys {
		if id, ok := keyNodes[string(key.KadKey(c))]; ok {
			nodes = append(nodes, id)
		}
	}

	return nodes, nil
}

func closestAtDepth(ctx context.Context, k tkey.Key, t *trie.Trie, depth int, n int, found []tkey.Key) ([]tkey.Key, error) {
	if ctx.Err() != nil {
		return found, ctx.Err()
	}

	// If we've already found enough peers, abort.
	if n == len(found) {
		return found, nil
	}

	// Find the closest direction.
	dir := k.BitAt(depth)
	var chosenDir byte
	if t.Branch[dir] != nil {
		// There are peers in the "closer" direction.
		chosenDir = dir
	} else if t.Branch[1-dir] != nil {
		// There are peers in the "less closer" direction.
		chosenDir = 1 - dir
	} else if t.Key != nil {
		// We've found a leaf
		return append(found, t.Key), nil
	} else {
		// We've found an empty node?
		return found, nil
	}

	var err error
	// Add peers from the closest direction first, then from the other direction.
	found, err = closestAtDepth(ctx, k, t.Branch[chosenDir], depth+1, n, found)
	if err != nil {
		return found, err
	}

	return closestAtDepth(ctx, k, t.Branch[1-chosenDir], depth+1, n, found)
}

func (rt *TrieRT) Find(ctx context.Context, kk key.KadKey) (address.NodeID, error) {
	if kk.Size() != rt.self.Size() {
		return nil, ErrKeyWrongLength
	}

	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID)

	if id, ok := keyNodes[string(key.KadKey(kk))]; ok {
		return id, nil
	}

	return nil, nil
}

// Size returns the number of peers contained in the table.
func (rt *TrieRT) Size() int {
	keys := rt.keys.Load().(*trie.Trie)
	return keys.Size()
}

// CplSize returns the number of peers in the table that share the specified common prefix length with the table's key.
func (rt *TrieRT) CplSize(cpl int) int {
	keys := rt.keys.Load().(*trie.Trie)
	n, err := countCpl(keys, rt.self, cpl, 0)
	if err != nil {
		return 0
	}
	return n
}

func countCpl(t *trie.Trie, kk key.KadKey, cpl int, depth int) (int, error) {
	if t.IsLeaf() {
		if t.IsEmpty() {
			return 0, nil
		}
		keyCpl, err := kk.CommonPrefixLength(key.KadKey(t.Key))
		if err != nil {
			return 0, err
		}
		if keyCpl == cpl {
			return 1, nil
		}
		return 0, nil
	}

	if depth == cpl {
		return t.Size(), nil
	}

	return countCpl(t.Branch[kk.BitAt(depth)], kk, cpl, depth+1)
}
