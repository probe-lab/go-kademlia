package trie

// Find looks for the key q in the trie.
// It returns the depth of the leaf reached along the path of q, regardless of whether q was found in that leaf.
// It also returns a boolean flag indicating whether the key was found.
func (trie *Trie[T, K]) Find(q K) (reachedDepth int, found bool) {
	return trie.FindAtDepth(0, q)
}

func (trie *Trie[T, K]) FindAtDepth(depth int, q K) (reachedDepth int, found bool) {
	switch {
	case trie.IsEmptyLeaf():
		return depth, false
	case trie.IsNonEmptyLeaf():
		return depth, trie.Key.Equal(q)
	default:
		return trie.Branch[q.BitAt(depth)].FindAtDepth(depth+1, q)
	}
}

func (trie *Trie[T, K]) ClosestAtDepth(k K, depth int, n int) []K {
	var zero K
	if !trie.Key.Equal(zero) {
		// We've found a leaf
		return []K{}
	} else if trie.Branch[0] == nil && trie.Branch[1] == nil {
		// We've found an empty node?
		return nil
	}

	// Find the closest direction.
	dir := k.BitAt(depth)
	// Add peers from the closest direction first
	found := trie.Branch[dir].ClosestAtDepth(k, depth+1, n)
	if len(found) == n {
		return found
	}
	// Didn't find enough peers in the closest direction, try the other direction.
	return append(found, trie.Branch[1-dir].ClosestAtDepth(k, depth+1, n-len(found))...)
}
