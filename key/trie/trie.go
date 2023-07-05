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
type Trie[T any] struct {
	Branch [2]*Trie[T]
	Key    key.KadKey
	Data   T
}

func New[T any]() *Trie[T] {
	return &Trie[T]{}
}

func (trie *Trie[T]) String() string {
	b, _ := json.Marshal(trie)
	return string(b)
}

func (trie *Trie[T]) Depth() int {
	return trie.DepthAtDepth(0)
}

func (trie *Trie[T]) DepthAtDepth(depth int) int {
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
func (trie *Trie[T]) Size() int {
	return trie.SizeAtDepth(0)
}

func (trie *Trie[T]) SizeAtDepth(depth int) int {
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

func (trie *Trie[T]) IsEmpty() bool {
	return trie.Key == nil
}

func (trie *Trie[T]) IsLeaf() bool {
	return trie.Branch[0] == nil && trie.Branch[1] == nil
}

func (trie *Trie[T]) IsEmptyLeaf() bool {
	return trie.IsEmpty() && trie.IsLeaf()
}

func (trie *Trie[T]) IsNonEmptyLeaf() bool {
	return !trie.IsEmpty() && trie.IsLeaf()
}

func (trie *Trie[T]) Copy() *Trie[T] {
	if trie.IsLeaf() {
		return &Trie[T]{Key: trie.Key, Data: trie.Data}
	}

	return &Trie[T]{Branch: [2]*Trie[T]{
		trie.Branch[0].Copy(),
		trie.Branch[1].Copy(),
	}}
}

func (trie *Trie[T]) shrink() {
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

// Add adds the key to trie, returning a new trie.
// Add is immutable/non-destructive: The original trie remains unchanged.
func Add[T any](trie *Trie[T], kk key.KadKey, data T) (*Trie[T], error) {
	return AddAtDepth(0, trie, kk, data)
}

func AddAtDepth[T any](depth int, trie *Trie[T], kk key.KadKey, data T) (*Trie[T], error) {
	switch {
	case trie.IsEmptyLeaf():
		return &Trie[T]{Key: kk, Data: data}, nil
	case trie.IsNonEmptyLeaf():
		eq := trie.Key.Equal(kk)
		if eq {
			return trie, nil
		}
		return trieForTwo[T](depth, trie.Key, trie.Data, kk, data), nil

	default:
		dir := kk.BitAt(depth)
		s := &Trie[T]{}
		b, err := AddAtDepth(depth+1, trie.Branch[dir], kk, data)
		if err != nil {
			return nil, err
		}
		s.Branch[dir] = b
		s.Branch[1-dir] = trie.Branch[1-dir]
		return s, nil
	}
}

func trieForTwo[T any](depth int, p key.KadKey, pdata T, q key.KadKey, qdata T) *Trie[T] {
	pDir, qDir := p.BitAt(depth), q.BitAt(depth)
	if qDir == pDir {
		s := &Trie[T]{}
		s.Branch[pDir] = trieForTwo[T](depth+1, p, pdata, q, qdata)
		s.Branch[1-pDir] = &Trie[T]{}
		return s
	} else {
		s := &Trie[T]{}
		s.Branch[pDir] = &Trie[T]{Key: p, Data: pdata}
		s.Branch[qDir] = &Trie[T]{Key: q, Data: qdata}
		return s
	}
}

func Remove[T any](trie *Trie[T], q key.KadKey) (*Trie[T], error) {
	return RemoveAtDepth(0, trie, q)
}

func RemoveAtDepth[T any](depth int, trie *Trie[T], q key.KadKey) (*Trie[T], error) {
	switch {
	case trie.IsEmptyLeaf():
		return trie, nil
	case trie.IsNonEmptyLeaf():
		eq := trie.Key.Equal(q)
		if !eq {
			return trie, nil
		}
		return &Trie[T]{}, nil

	default:
		dir := q.BitAt(depth)
		b, err := RemoveAtDepth(depth+1, trie.Branch[dir], q)
		if err != nil {
			return nil, err
		}
		afterDelete := b
		if afterDelete == trie.Branch[dir] {
			return trie, nil
		}
		copy := &Trie[T]{}
		copy.Branch[dir] = afterDelete
		copy.Branch[1-dir] = trie.Branch[1-dir]
		copy.shrink()
		return copy, nil
	}
}

func Equal[T any](p, q *Trie[T]) bool {
	switch {
	case p.IsLeaf() && q.IsLeaf():
		eq := p.Key.Equal(q.Key)
		if !eq {
			return false
		}
		return true
	case !p.IsLeaf() && !q.IsLeaf():
		return Equal(p.Branch[0], q.Branch[0]) && Equal(p.Branch[1], q.Branch[1])
	}
	return false
}

// Find looks for the key in the trie.
// It reports whether the key was found along with the depth of the leaf reached along the path
// of the key, regardless of whether the key was found in that leaf.
func Find[T any](trie *Trie[T], kk key.KadKey) (bool, T, int) {
	return FindAtDepth(trie, 0, kk)
}

func FindAtDepth[T any](trie *Trie[T], depth int, kk key.KadKey) (bool, T, int) {
	switch {
	case trie.IsEmptyLeaf():
		var v T
		return false, v, depth
	case trie.IsNonEmptyLeaf():
		eq := trie.Key.Equal(kk)
		return eq, trie.Data, depth
	default:
		return FindAtDepth(trie.Branch[kk.BitAt(depth)], depth+1, kk)
	}
}
