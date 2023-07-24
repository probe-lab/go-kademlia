package kadaddr

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
)

type KadAddr[K kad.Key[K]] struct {
	id    *kadid.KadID[K]
	addrs []string
}

var _ address.NodeAddr[key.Key8] = (*KadAddr[key.Key8])(nil)

func NewKadAddr[K kad.Key[K]](id *kadid.KadID[K], addrs []string) *KadAddr[K] {
	return &KadAddr[K]{
		id:    id,
		addrs: addrs,
	}
}

func (ka *KadAddr[K]) AddAddr(addr string) {
	ka.addrs = append(ka.addrs, addr)
}

func (ka *KadAddr[K]) RemoveAddr(addr string) {
	writeIndex := 0
	// remove all occurrences of addr
	for _, a := range ka.addrs {
		if a != addr {
			ka.addrs[writeIndex] = a
			writeIndex++
		}
	}
	ka.addrs = ka.addrs[:writeIndex]
}

func (ka *KadAddr[K]) NodeID() address.NodeID[K] {
	return ka.id
}

func (ka *KadAddr[K]) Addresses() []address.Addr {
	addresses := make([]address.Addr, len(ka.addrs))
	for i, a := range ka.addrs {
		addresses[i] = a
	}
	return addresses
}
