package kadid

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

// KadID wraps a key.KadKey to serve as a NodeID and a NodeAddr.
// It enables testing of various routing scenarios by allowing for
// controlled placement of the node within the keyspace.
type KadID struct {
	key.KadKey
}

var (
	_ address.NodeAddr = (*KadID)(nil)
	_ address.NodeID   = (*KadID)(nil)
)

func NewKadID(k key.KadKey) *KadID {
	return &KadID{KadKey: k}
}

func (k *KadID) Key() key.KadKey {
	return k.KadKey
}

func (k *KadID) NodeID() address.NodeID {
	return k
}

func (k *KadID) String() string {
	return k.Hex()
}

func (k *KadID) Addresses() []address.Addr {
	return []address.Addr{k}
}

func (k *KadID) Equal(other address.NodeID) bool {
	tother, ok := other.(*KadID)
	if !ok {
		return false
	}
	if k == nil || k.KadKey == nil || tother == nil || tother.KadKey == nil {
		return false
	}
	return k.KadKey.Equal(tother.KadKey)
}
