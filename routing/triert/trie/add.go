package trie

import (
	"github.com/plprobelab/go-kademlia/key"
)

// Add adds the key q to the trie. Add mutates the trie.
func (trie *Trie[T, K]) Add(q K) (insertedDepth int, insertedOK bool) {
	return trie.AddAtDepth(0, q)
}

func (trie *Trie[T, K]) AddAtDepth(depth int, q K) (insertedDepth int, insertedOK bool) {
	switch {
	case trie.IsEmptyLeaf():
		trie.Key = q
		return depth, true
	case trie.IsNonEmptyLeaf():
		if trie.Key.Equal(q) {
			return depth, false
		} else {
			p := trie.Key
			var zero K
			trie.Key = zero
			// both branches are nil
			trie.Branch[0], trie.Branch[1] = &Trie[T, K]{}, &Trie[T, K]{}
			trie.Branch[p.BitAt(depth)].Key = p
			return trie.Branch[q.BitAt(depth)].AddAtDepth(depth+1, q)
		}
	default:
		return trie.Branch[q.BitAt(depth)].AddAtDepth(depth+1, q)
	}
}

// Add adds the key q to trie, returning a new trie.
// Add is immutable/non-destructive: The original trie remains unchanged.
func Add[T any, K key.Kademlia[K]](trie *Trie[T, K], q K) *Trie[T, K] {
	return AddAtDepth(0, trie, q)
}

func AddAtDepth[T any, K key.Kademlia[K]](depth int, trie *Trie[T, K], q K) *Trie[T, K] {
	switch {
	case trie.IsEmptyLeaf():
		return &Trie[T, K]{Key: q}
	case trie.IsNonEmptyLeaf():
		if trie.Key.Equal(q) {
			return trie
		} else {
			return trieForTwo[T, K](depth, trie.Key, q)
		}
	default:
		dir := q.BitAt(depth)
		s := &Trie[T, K]{}
		s.Branch[dir] = AddAtDepth(depth+1, trie.Branch[dir], q)
		s.Branch[1-dir] = trie.Branch[1-dir]
		return s
	}
}

func trieForTwo[T any, K key.Kademlia[K]](depth int, p, q K) *Trie[T, K] {
	pDir, qDir := p.BitAt(depth), q.BitAt(depth)
	if qDir == pDir {
		s := &Trie[T, K]{}
		s.Branch[pDir] = trieForTwo[T](depth+1, p, q)
		s.Branch[1-pDir] = &Trie[T, K]{}
		return s
	} else {
		s := &Trie[T, K]{}
		s.Branch[pDir] = &Trie[T, K]{Key: p}
		s.Branch[qDir] = &Trie[T, K]{Key: q}
		return s
	}
}
