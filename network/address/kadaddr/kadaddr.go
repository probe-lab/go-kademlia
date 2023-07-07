package kadaddr

import (
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
)

type KadAddr struct {
	id    *kadid.KadID
	addrs []string
}

var _ address.NodeAddr = (*KadAddr)(nil)

func NewKadAddr(id *kadid.KadID, addrs []string) *KadAddr {
	return &KadAddr{
		id:    id,
		addrs: addrs,
	}
}

func (ka *KadAddr) AddAddr(addr string) {
	ka.addrs = append(ka.addrs, addr)
}

func (ka *KadAddr) RemoveAddr(addr string) {
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

func (ka *KadAddr) NodeID() address.NodeID {
	return ka.id
}

func (ka *KadAddr) Addresses() []address.Addr {
	addresses := make([]address.Addr, len(ka.addrs))
	for i, a := range ka.addrs {
		addresses[i] = a
	}
	return addresses
}
