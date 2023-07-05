package trie

import "github.com/plprobelab/go-kademlia/key"

func Equal[T any, K key.Kademlia[K]](p, q *Trie[T, K]) bool {
	switch {
	case p.IsLeaf() && q.IsLeaf():
		return p.Key.Equal(q.Key)
	case !p.IsLeaf() && !q.IsLeaf():
		return Equal(p.Branch[0], q.Branch[0]) && Equal(p.Branch[1], q.Branch[1])
	}
	return false
}
