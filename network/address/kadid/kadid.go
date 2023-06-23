package kadid

import (
	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
)

type KadID struct {
	key.KadKey
}

var _ address.NodeID = (*KadID)(nil)

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
