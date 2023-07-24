package kadid

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

type KadID[K kad.Key[K]] struct {
	key K
}

var _ address.NodeID[key.Key8] = (*KadID[key.Key8])(nil)

func NewKadID[K kad.Key[K]](k K) *KadID[K] {
	return &KadID[K]{k}
}

func (k KadID[K]) Key() K {
	return k.key
}

func (k KadID[K]) String() string {
	return key.HexString(k.key)
}

func (k KadID[K]) Addresses() []address.Addr {
	return []address.Addr{k}
}
