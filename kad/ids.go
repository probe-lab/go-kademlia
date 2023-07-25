package kad

import (
	"github.com/plprobelab/go-kademlia/key"
)

// ID is a concrete implementation of the NodeID interface.
type ID[K Key[K]] struct {
	key K
}

// interface assertion. The concrete key type of key.Key8 does not
// limit the validity of the assertion for other key types.
var _ NodeID[key.Key8] = (*ID[key.Key8])(nil)

// NewKadID returns a new Kademlia identifier that implements the NodeID interface.
// Instead of deriving the Kademlia key from a NodeID, this method directly takes
// the Kademlia key.
func NewKadID[K Key[K]](k K) *ID[K] {
	return &ID[K]{key: k}
}

// Key returns the Kademlia key that is used by, e.g., the routing table
// implementation to group nodes into buckets. The returned key was manually
// defined in the ID constructor NewKadID and not derived via, e.g., hashing
// a preimage.
func (i ID[K]) Key() K {
	return i.key
}

type Addr[K Key[K], A comparable] struct {
	id    *ID[K]
	addrs []A
}

var _ NodeAddr[key.Key8] = (*Addr[key.Key8, any])(nil)

func NewKadAddr[K Key[K], A comparable](id *ID[K], addrs []A) *Addr[K, A] {
	return &Addr[K, A]{
		id:    id,
		addrs: addrs,
	}
}

func (a *Addr[K, A]) AddAddr(addr A) {
	a.addrs = append(a.addrs, addr)
}

func (a *Addr[K, A]) RemoveAddr(addr A) {
	writeIndex := 0
	// remove all occurrences of addr
	for _, ad := range a.addrs {
		if ad != addr {
			a.addrs[writeIndex] = ad
			writeIndex++
		}
	}
	a.addrs = a.addrs[:writeIndex]
}

func (a *Addr[K, A]) ID() NodeID[K] {
	return a.id
}

func (a *Addr[K, A]) Addresses() []Address {
	addresses := make([]Address, len(a.addrs))
	for i, addr := range a.addrs {
		addresses[i] = addr
	}
	return addresses
}
