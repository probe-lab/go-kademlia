package kadid

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

type KadID struct {
	key.KadKey
}

var _ address.NodeAddr = (*KadID)(nil)

func NewKadID(k key.KadKey) *KadID {
	return &KadID{k}
}

func (k KadID) Key() key.KadKey {
	return k.KadKey
}

func (k KadID) NodeID() address.NodeID {
	return &k
}

func (k KadID) String() string {
	return k.Hex()
}

func (k KadID) Addresses() []address.Addr {
	return []address.Addr{k}
}
