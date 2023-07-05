package triert

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/routing/triert/trie"
)

// TODO: move to key package and replace ErrInvalidKey
// ErrKeyWrongLength indicates a key has the wrong length
var ErrKeyWrongLength = errors.New("key has wrong length")

// TrieRT is a routing table backed by a Xor Trie which offers good scalablity and performance
// for large networks. All exported methods are safe for concurrent use.
type TrieRT[T key.Kademlia[T]] struct {
	self T

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

func New[T key.Kademlia[T]](self T) *TrieRT[T] {
	rt := &TrieRT[T]{
		self: self,
	}
	rt.keys.Store(&trie.Trie[any, T]{})
	rt.keyNodes.Store(map[string]address.NodeID[T]{})
	return rt
}

func (rt *TrieRT[T]) Self() T {
	return rt.self
}

// AddPeer tries to add a peer to the routing table
func (rt *TrieRT[T]) AddPeer(ctx context.Context, node address.NodeID[T]) bool {
	rt.keymu.Lock()
	defer rt.keymu.Unlock()
	// load the old trie, derive a mutated variant and store it in place of the original
	keys := rt.keys.Load().(*trie.Trie[any, T])

	keysNext := trie.Add[any, T](keys, node.Key())

	// if trie is unchanged then we didn't add the key
	if keysNext == keys {
		return false
	}

	// make a copy of keyNodes
	// could avoid this if we held key/nodeid tuple in the trie
	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID[T])
	keyNodesNext := make(map[string]address.NodeID[T], len(keyNodes)+1)
	for k, v := range keyNodes {
		keyNodesNext[k] = v
	}
	keyNodesNext[node.Key().String()] = node

	rt.keyNodes.Store(keyNodesNext)
	rt.keys.Store(keysNext)
	return true
}

// RemoveKey tries to remove a peer identified by its Kademlia key from the
// routing table
func (rt *TrieRT[T]) RemoveKey(ctx context.Context, kk key.Kademlia[T]) bool {
	rt.keymu.Lock()
	defer rt.keymu.Unlock()
	// load the old trie, derive a mutated variant and store it in place of the original
	keys := rt.keys.Load().(*trie.Trie[any, T])
	keysNext := trie.Remove(keys, kk.Key())

	// if trie is unchanged then we didn't remove the key
	if keysNext == keys {
		return false
	}

	// make a copy of keyNodes without removed key
	// could avoid this if we held key/nodeid tuple in the trie
	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID[T])
	if _, exists := keyNodes[kk.String()]; exists {
		keyNodesNext := make(map[string]address.NodeID[T], len(keyNodes))
		for k, v := range keyNodes {
			if k == kk.String() {
				continue
			}
			keyNodesNext[k] = v
		}
		rt.keyNodes.Store(keyNodesNext)
	}

	rt.keys.Store(keysNext)

	return true
}

// NearestPeers returns the n closest peers to a given key
func (rt *TrieRT[T]) NearestPeers(ctx context.Context, kk key.Kademlia[T], n int) ([]address.NodeID[T], error) {
	keys := rt.keys.Load().(*trie.Trie[any, T])

	closestKeys := keys.ClosestAtDepth(kk.Key(), 0, n)
	if len(closestKeys) == 0 {
		return []address.NodeID[T]{}, nil
	}

	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID[T])

	nodes := make([]address.NodeID[T], 0, len(closestKeys))
	for _, c := range closestKeys {
		if id, ok := keyNodes[c.String()]; ok {
			nodes = append(nodes, id)
		}
	}

	return nodes, nil
}

func (rt *TrieRT[T]) Find(ctx context.Context, kk key.Kademlia[T]) (address.NodeID[T], error) {
	keyNodes := rt.keyNodes.Load().(map[string]address.NodeID[T])

	if id, ok := keyNodes[kk.String()]; ok {
		return id, nil
	}

	return nil, nil
}

// Size returns the number of peers contained in the table.
func (rt *TrieRT[T]) Size() int {
	keys := rt.keys.Load().(*trie.Trie[any, T])
	return keys.Size()
}

// CplSize returns the number of peers in the table that share the specified common prefix length with the table's key.
func (rt *TrieRT[T]) CplSize(cpl int) int {
	keys := rt.keys.Load().(*trie.Trie[any, T])
	return countCpl(keys, rt.self, cpl, 0)
}

func countCpl[T key.Kademlia[T]](t *trie.Trie[any, T], kk T, cpl int, depth int) int {
	if t.IsLeaf() {
		if t.IsEmpty() {
			return 0
		}
		keyCpl := kk.CommonPrefixLength(t.Key)
		if keyCpl == cpl {
			return 1
		}
		return 0
	}

	if depth == cpl {
		return t.Size()
	}

	return countCpl(t.Branch[kk.BitAt(depth)], kk, cpl, depth+1)
}
