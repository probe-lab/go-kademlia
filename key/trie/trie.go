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
	branch [2]*Trie[T]
	key    key.KadKey
	data   T
}

func New[T any]() *Trie[T] {
	return &Trie[T]{}
}

func (tr *Trie[T]) Key() key.KadKey {
	return tr.key
}

func (tr *Trie[T]) Data() T {
	return tr.data
}

func (tr *Trie[T]) Branch(dir int) *Trie[T] {
	return tr.branch[dir]
}

// Size returns the number of keys added to the trie.
func (tr *Trie[T]) Size() int {
	return tr.sizeAtDepth(0)
}

// Size returns the number of keys added to the trie at or beyond depth d.
func (tr *Trie[T]) sizeAtDepth(d int) int {
	if tr.IsLeaf() {
		if !tr.HasKey() {
			return 0
		} else {
			return 1
		}
	} else {
		return tr.branch[0].sizeAtDepth(d+1) + tr.branch[1].sizeAtDepth(d+1)
	}
}

// HasKey reports whether the Trie node holds a key.
func (tr *Trie[T]) HasKey() bool {
	return tr.key != nil
}

// IsLeaf reports whether the Trie is a leaf node. A leaf node has no child branches but may hold a key and data.
func (tr *Trie[T]) IsLeaf() bool {
	return tr.branch[0] == nil && tr.branch[1] == nil
}

// IsEmptyLeaf reports whether the Trie is a leaf node without branches that also has no key.
func (tr *Trie[T]) IsEmptyLeaf() bool {
	return !tr.HasKey() && tr.IsLeaf()
}

// IsEmptyLeaf reports whether the Trie is a leaf node without branches but has a key.
func (tr *Trie[T]) IsNonEmptyLeaf() bool {
	return tr.HasKey() && tr.IsLeaf()
}

func (tr *Trie[T]) Copy() *Trie[T] {
	if tr.IsLeaf() {
		return &Trie[T]{key: tr.key, data: tr.data}
	}

	return &Trie[T]{branch: [2]*Trie[T]{
		tr.branch[0].Copy(),
		tr.branch[1].Copy(),
	}}
}

func (tr *Trie[T]) shrink() {
	b0, b1 := tr.branch[0], tr.branch[1]
	switch {
	case b0.IsEmptyLeaf() && b1.IsEmptyLeaf():
		tr.branch[0], tr.branch[1] = nil, nil
	case b0.IsEmptyLeaf() && b1.IsNonEmptyLeaf():
		tr.key = b1.key
		tr.branch[0], tr.branch[1] = nil, nil
	case b0.IsNonEmptyLeaf() && b1.IsEmptyLeaf():
		tr.key = b0.key
		tr.branch[0], tr.branch[1] = nil, nil
	}
}

func (tr *Trie[T]) firstNonEmptyLeaf() *Trie[T] {
	if tr.IsLeaf() {
		if tr.HasKey() {
			return tr
		}
		return nil
	}
	f := tr.branch[0].firstNonEmptyLeaf()
	if f != nil {
		return f
	}
	return tr.branch[1].firstNonEmptyLeaf()
}

// Add attemptes to add a key to the trie, mutating the trie.
// Returns true if the key was added, false otherwise.
func (tr *Trie[T]) Add(kk key.KadKey, data T) (bool, error) {
	f := tr.firstNonEmptyLeaf()
	if f != nil {
		if f.key.Size() != kk.Size() {
			return false, ErrMismatchedKeyLength
		}
	}
	return tr.addAtDepth(0, kk, data), nil
}

func (tr *Trie[T]) addAtDepth(depth int, kk key.KadKey, data T) bool {
	switch {
	case tr.IsEmptyLeaf():
		tr.key = kk
		tr.data = data
		return true
	case tr.IsNonEmptyLeaf():
		if tr.key.Equal(kk) {
			return false
		} else {
			p := tr.key
			tr.key = nil
			// both branches are nil
			tr.branch[0], tr.branch[1] = &Trie[T]{}, &Trie[T]{}
			tr.branch[p.BitAt(depth)].key = p
			return tr.branch[kk.BitAt(depth)].addAtDepth(depth+1, kk, data)
		}
	default:
		return tr.branch[kk.BitAt(depth)].addAtDepth(depth+1, kk, data)
	}
}

// Add adds the key to trie, returning a new trie.
// Add is immutable/non-destructive: the original trie remains unchanged.
func Add[T any](tr *Trie[T], kk key.KadKey, data T) (*Trie[T], error) {
	f := tr.firstNonEmptyLeaf()
	if f != nil {
		if f.key.Size() != kk.Size() {
			return tr, ErrMismatchedKeyLength
		}
	}
	return addAtDepth(0, tr, kk, data), nil
}

func addAtDepth[T any](depth int, tr *Trie[T], kk key.KadKey, data T) *Trie[T] {
	switch {
	case tr.IsEmptyLeaf():
		return &Trie[T]{key: kk, data: data}
	case tr.IsNonEmptyLeaf():
		if tr.key.Size() != kk.Size() {
			return nil
		}
		eq := tr.key.Equal(kk)
		if eq {
			return tr
		}
		return trieForTwo(depth, tr.key, tr.data, kk, data)

	default:
		dir := kk.BitAt(depth)
		s := &Trie[T]{}
		s.branch[dir] = addAtDepth(depth+1, tr.branch[dir], kk, data)
		s.branch[1-dir] = tr.branch[1-dir]
		return s
	}
}

func trieForTwo[T any](depth int, p key.KadKey, pdata T, q key.KadKey, qdata T) *Trie[T] {
	pDir, qDir := p.BitAt(depth), q.BitAt(depth)
	if qDir == pDir {
		s := &Trie[T]{}
		s.branch[pDir] = trieForTwo(depth+1, p, pdata, q, qdata)
		s.branch[1-pDir] = &Trie[T]{}
		return s
	} else {
		s := &Trie[T]{}
		s.branch[pDir] = &Trie[T]{key: p, data: pdata}
		s.branch[qDir] = &Trie[T]{key: q, data: qdata}
		return s
	}
}

// Remove attempts to remove a key from the trie, mutating the trie.
// Returns true if the key was removed, false otherwise.
func (tr *Trie[T]) Remove(kk key.KadKey) (bool, error) {
	f := tr.firstNonEmptyLeaf()
	if f != nil {
		if f.key.Size() != kk.Size() {
			return false, ErrMismatchedKeyLength
		}
	}
	return tr.removeAtDepth(0, kk), nil
}

func (tr *Trie[T]) removeAtDepth(depth int, kk key.KadKey) bool {
	switch {
	case tr.IsEmptyLeaf():
		return false
	case tr.IsNonEmptyLeaf():
		tr.key = nil
		var v T
		tr.data = v
		return true
	default:
		if tr.branch[kk.BitAt(depth)].removeAtDepth(depth+1, kk) {
			tr.shrink()
			return true
		} else {
			return false
		}
	}
}

// Remove removes the key from the trie.
// Remove is immutable/non-destructive: the original trie remains unchanged.
// If the key did not exist in the trie then the original trie is returned.
func Remove[T any](tr *Trie[T], kk key.KadKey) (*Trie[T], error) {
	f := tr.firstNonEmptyLeaf()
	if f != nil {
		if f.key.Size() != kk.Size() {
			return tr, ErrMismatchedKeyLength
		}
	}
	return removeAtDepth(0, tr, kk), nil
}

func removeAtDepth[T any](depth int, tr *Trie[T], kk key.KadKey) *Trie[T] {
	switch {
	case tr.IsEmptyLeaf():
		return tr
	case tr.IsNonEmptyLeaf():
		if tr.key.Size() != kk.Size() {
			return nil
		}
		eq := tr.key.Equal(kk)
		if !eq {
			return tr
		}
		return &Trie[T]{}

	default:
		dir := kk.BitAt(depth)
		afterDelete := removeAtDepth(depth+1, tr.branch[dir], kk)
		if afterDelete == tr.branch[dir] {
			return tr
		}
		copy := &Trie[T]{}
		copy.branch[dir] = afterDelete
		copy.branch[1-dir] = tr.branch[1-dir]
		copy.shrink()
		return copy
	}
}

func Equal[T any](a, b *Trie[T]) bool {
	switch {
	case a.IsLeaf() && b.IsLeaf():
		eq := a.key.Equal(b.key)
		if !eq {
			return false
		}
		return true
	case !a.IsLeaf() && !b.IsLeaf():
		return Equal(a.branch[0], b.branch[0]) && Equal(a.branch[1], b.branch[1])
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
	return true, f.data
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
		eq := tr.key.Equal(kk)
		if !eq {
			return nil, depth
		}
		return tr, depth
	default:
		return findFromDepth(tr.branch[kk.BitAt(depth)], depth+1, kk)
	}
}
