package trie

import "github.com/plprobelab/go-kademlia/key"

// Remove removes the key q from the trie. Remove mutates the trie.
// TODO: Also implement an immutable version of Remove.
func (trie *Trie[T, K]) Remove(q K) (removedDepth int, removed bool) {
	return trie.RemoveAtDepth(0, q)
}

func (trie *Trie[T, K]) RemoveAtDepth(depth int, q K) (reachedDepth int, removed bool) {
	switch {
	case trie.IsEmptyLeaf():
		return depth, false
	case trie.IsNonEmptyLeaf():
		trie.Key = *new(K)
		return depth, true
	default:
		if d, removed := trie.Branch[q.BitAt(depth)].RemoveAtDepth(depth+1, q); removed {
			trie.shrink()
			return d, true
		} else {
			return d, false
		}
	}
}

func Remove[T any, K key.Kademlia[K]](trie *Trie[T, K], q K) *Trie[T, K] {
	return RemoveAtDepth(0, trie, q)
}

func RemoveAtDepth[T any, K key.Kademlia[K]](depth int, trie *Trie[T, K], q K) *Trie[T, K] {
	switch {
	case trie.IsEmptyLeaf():
		return trie
	case trie.IsNonEmptyLeaf() && !trie.Key.Equal(q):
		return trie
	case trie.IsNonEmptyLeaf() && trie.Key.Equal(q):
		return &Trie[T, K]{}
	default:
		dir := q.BitAt(depth)
		afterDelete := RemoveAtDepth(depth+1, trie.Branch[dir], q)
		if afterDelete == trie.Branch[dir] {
			return trie
		}
		copy := &Trie[T, K]{}
		copy.Branch[dir] = afterDelete
		copy.Branch[1-dir] = trie.Branch[1-dir]
		copy.shrink()
		return copy
	}
}
