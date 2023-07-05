package trie

// List returns a list of all keys in the trie.
func (trie *Trie[T, K]) List() []K {
	switch {
	case trie.IsEmptyLeaf():
		return nil
	case trie.IsNonEmptyLeaf():
		return []K{trie.Key}
	default:
		return append(trie.Branch[0].List(), trie.Branch[1].List()...)
	}
}
