package triert

import "github.com/plprobelab/go-kademlia/key"

// KeyFilterFunc is a function that is applied before a key is added to the table.
// Return false to prevent the key from being added.
type KeyFilterFunc func(rt *TrieRT, kk key.KadKey) bool

// BucketLimit20 is a filter function that limits the occupancy of buckets in the table to 20 keys.
func BucketLimit20(rt *TrieRT, kk key.KadKey) bool {
	cpl := rt.Cpl(kk)
	return rt.CplSize(cpl) < 20
}
