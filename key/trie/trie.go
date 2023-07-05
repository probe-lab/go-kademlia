// Package trie provides an implementation of a XOR Trie
package trie

import (
	"errors"

	"github.com/plprobelab/go-kademlia/key"
)

var ErrMismatchedKeyLength = errors.New("key length does not match existing keys")

// Trie is a trie for equal-length bit vectors, which stores values only in the leaves.
// A node may optionally hold data of type T
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

// Depth returns the maximum depth of the Trie.
func (tr *Trie[T]) Depth() int {
	return tr.DepthAtDepth(0)
}

// Depth returns the maximum depth at or beyond depth d.
func (tr *Trie[T]) DepthAtDepth(d int) int {
	if tr.IsLeaf() {
		return d
	} else {
		return max(tr.Branch[0].DepthAtDepth(d+1), tr.Branch[1].DepthAtDepth(d+1))
	}
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Size returns the number of keys added to the trie.
func (tr *Trie[T]) Size() int {
	return tr.SizeAtDepth(0)
}

// Size returns the number of keys added to the trie at or beyond depth d.
func (tr *Trie[T]) SizeAtDepth(d int) int {
	if tr.IsLeaf() {
		if !tr.HasKey() {
			return 0
		} else {
			return 1
		}
	} else {
		return tr.Branch[0].SizeAtDepth(d+1) + tr.Branch[1].SizeAtDepth(d+1)
	}
}

// HasKey reports whether the Trie node holds a key.
func (tr *Trie[T]) HasKey() bool {
	return tr.Key != nil
}

// IsLeaf reports whether the Trie is a leaf node. A leaf node has no child branches but may hold a key and data.
func (tr *Trie[T]) IsLeaf() bool {
	return tr.Branch[0] == nil && tr.Branch[1] == nil
}

func (tr *Trie[T]) IsEmptyLeaf() bool {
	return !tr.HasKey() && tr.IsLeaf()
}

func (tr *Trie[T]) IsNonEmptyLeaf() bool {
	return tr.HasKey() && tr.IsLeaf()
}

func (tr *Trie[T]) Copy() *Trie[T] {
	if tr.IsLeaf() {
		return &Trie[T]{Key: tr.Key, Data: tr.Data}
	}

	return &Trie[T]{Branch: [2]*Trie[T]{
		tr.Branch[0].Copy(),
		tr.Branch[1].Copy(),
	}}
}

func (tr *Trie[T]) shrink() {
	b0, b1 := tr.Branch[0], tr.Branch[1]
	switch {
	case b0.IsEmptyLeaf() && b1.IsEmptyLeaf():
		tr.Branch[0], tr.Branch[1] = nil, nil
	case b0.IsEmptyLeaf() && b1.IsNonEmptyLeaf():
		tr.Key = b1.Key
		tr.Branch[0], tr.Branch[1] = nil, nil
	case b0.IsNonEmptyLeaf() && b1.IsEmptyLeaf():
		tr.Key = b0.Key
		tr.Branch[0], tr.Branch[1] = nil, nil
	}
}

// Add adds the key to trie, returning a new trie.
// Add is immutable/non-destructive: The original trie remains unchanged.
func Add[T any](tr *Trie[T], kk key.KadKey, data T) (*Trie[T], error) {
	return AddAtDepth(0, tr, kk, data)
}

func AddAtDepth[T any](depth int, tr *Trie[T], kk key.KadKey, data T) (*Trie[T], error) {
	switch {
	case tr.IsEmptyLeaf():
		return &Trie[T]{Key: kk, Data: data}, nil
	case tr.IsNonEmptyLeaf():
		if tr.Key.Size() != kk.Size() {
			return nil, ErrMismatchedKeyLength
		}
		eq := tr.Key.Equal(kk)
		if eq {
			return tr, nil
		}
		return trieForTwo[T](depth, tr.Key, tr.Data, kk, data), nil

	default:
		dir := kk.BitAt(depth)
		s := &Trie[T]{}
		b, err := AddAtDepth(depth+1, tr.Branch[dir], kk, data)
		if err != nil {
			return nil, err
		}
		s.Branch[dir] = b
		s.Branch[1-dir] = tr.Branch[1-dir]
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

// Remove is immutable/non-destructive: The original trie remains unchanged.
func Remove[T any](tr *Trie[T], q key.KadKey) (*Trie[T], error) {
	return RemoveAtDepth(0, tr, q)
}

func RemoveAtDepth[T any](depth int, tr *Trie[T], kk key.KadKey) (*Trie[T], error) {
	switch {
	case tr.IsEmptyLeaf():
		return tr, nil
	case tr.IsNonEmptyLeaf():
		if tr.Key.Size() != kk.Size() {
			return nil, ErrMismatchedKeyLength
		}
		eq := tr.Key.Equal(kk)
		if !eq {
			return tr, nil
		}
		return &Trie[T]{}, nil

	default:
		dir := kk.BitAt(depth)
		b, err := RemoveAtDepth(depth+1, tr.Branch[dir], kk)
		if err != nil {
			return nil, err
		}
		afterDelete := b
		if afterDelete == tr.Branch[dir] {
			return tr, nil
		}
		copy := &Trie[T]{}
		copy.Branch[dir] = afterDelete
		copy.Branch[1-dir] = tr.Branch[1-dir]
		copy.shrink()
		return copy, nil
	}
}

func Equal[T any](a, b *Trie[T]) bool {
	switch {
	case a.IsLeaf() && b.IsLeaf():
		eq := a.Key.Equal(b.Key)
		if !eq {
			return false
		}
		return true
	case !a.IsLeaf() && !b.IsLeaf():
		return Equal(a.Branch[0], b.Branch[0]) && Equal(a.Branch[1], b.Branch[1])
	}
	return false
}

// Find looks for a key in the trie.
// It reports whether the key was found along with data value held with the key.
func Find[T any](tr *Trie[T], kk key.KadKey) (bool, T) {
	f, _ := findFromDepth(tr, 0, kk)
	if f == nil {
		var v T
		return false, v
	}
	return true, f.Data
}

// Locate looks for the position of a key in the trie.
// It reports whether the key was found along with the depth of the leaf reached along the path
// of the key, regardless of whether the key was found in that leaf.
func Locate[T any](tr *Trie[T], kk key.KadKey) (bool, int) {
	f, depth := findFromDepth(tr, 0, kk)
	if f == nil {
		return false, depth
	}
	return true, depth
}

func findFromDepth[T any](tr *Trie[T], depth int, kk key.KadKey) (*Trie[T], int) {
	switch {
	case tr.IsEmptyLeaf():
		return nil, depth
	case tr.IsNonEmptyLeaf():
		eq := tr.Key.Equal(kk)
		if !eq {
			return nil, depth
		}
		return tr, depth
	default:
		return findFromDepth(tr.Branch[kk.BitAt(depth)], depth+1, kk)
	}
}
