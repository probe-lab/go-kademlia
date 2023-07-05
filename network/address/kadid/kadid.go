package kadid

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

type KadID struct {
	key.SHA256
}

var _ address.NodeID[key.SHA256] = (*KadID)(nil)

func NewKadID(k key.SHA256) *KadID {
	return &KadID{k}
}

func (k KadID) Key() key.SHA256 {
	return k.SHA256
}

func (k KadID) NodeID() address.NodeID[key.SHA256] {
	return &k
}

func (k KadID) String() string {
	return "..."
}
