package trie

import (
	"encoding/json"
	"github.com/plprobelab/go-kademlia/key"
)

// Trie is a trie for equal-length bit vectors, which stores values only in the leaves.
// Trie node invariants:
// (1) Either both branches are nil, or both are non-nil.
// (2) If branches are non-nil, key must be nil.
// (3) If both branches are leaves, then they are both non-empty (have keys).
type Trie[T any, K key.Kademlia[K]] struct {
	Branch [2]*Trie[T, K]
	Key    K
	Data   T
}

func New[T any, K key.Kademlia[K]]() *Trie[T, K] {
	return &Trie[T, K]{}
}

func FromKeys[T any, K key.Kademlia[K]](keys []K) *Trie[T, K] {
	t := New[T, K]()
	for _, key := range keys {
		t.Add(key)
	}
	return t
}

func FromKeysAtDepth[T any, K key.Kademlia[K]](depth int, k []K) *Trie[T, K] {
	t := New[T, K]()
	for _, k := range k {
		t.AddAtDepth(depth, k)
	}
	return t
}

func (trie *Trie[T, K]) String() string {
	b, _ := json.Marshal(trie)
	return string(b)
}

func (trie *Trie[T, K]) Depth() int {
	return trie.DepthAtDepth(0)
}

func (trie *Trie[T, K]) DepthAtDepth(depth int) int {
	if trie.Branch[0] == nil && trie.Branch[1] == nil {
		return depth
	} else {
		return max(trie.Branch[0].DepthAtDepth(depth+1), trie.Branch[1].DepthAtDepth(depth+1))
	}
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Size returns the number of keys added to the trie.
// In other words, it returns the number of non-empty leaves in the trie.
func (trie *Trie[T, K]) Size() int {
	return trie.SizeAtDepth(0)
}

func (trie *Trie[T, K]) SizeAtDepth(depth int) int {
	if trie.Branch[0] == nil && trie.Branch[1] == nil {
		if trie.IsEmpty() {
			return 0
		} else {
			return 1
		}
	} else {
		return trie.Branch[0].SizeAtDepth(depth+1) + trie.Branch[1].SizeAtDepth(depth+1)
	}
}

func (trie *Trie[T, K]) IsEmpty() bool {
	var zero K
	return trie.Key.Equal(zero)
}

func (trie *Trie[T, K]) IsLeaf() bool {
	return trie.Branch[0] == nil && trie.Branch[1] == nil
}

func (trie *Trie[T, K]) IsEmptyLeaf() bool {
	return trie.IsEmpty() && trie.IsLeaf()
}

func (trie *Trie[T, K]) IsNonEmptyLeaf() bool {
	return !trie.IsEmpty() && trie.IsLeaf()
}

func (trie *Trie[T, K]) Copy() *Trie[T, K] {
	if trie.IsLeaf() {
		return &Trie[T, K]{Key: trie.Key}
	}

	return &Trie[T, K]{Branch: [2]*Trie[T, K]{
		trie.Branch[0].Copy(),
		trie.Branch[1].Copy(),
	}}
}

func (trie *Trie[T, K]) shrink() {
	b0, b1 := trie.Branch[0], trie.Branch[1]
	switch {
	case b0.IsEmptyLeaf() && b1.IsEmptyLeaf():
		trie.Branch[0], trie.Branch[1] = nil, nil
	case b0.IsEmptyLeaf() && b1.IsNonEmptyLeaf():
		trie.Key = b1.Key
		trie.Branch[0], trie.Branch[1] = nil, nil
	case b0.IsNonEmptyLeaf() && b1.IsEmptyLeaf():
		trie.Key = b0.Key
		trie.Branch[0], trie.Branch[1] = nil, nil
	}
}
