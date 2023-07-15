package stringid

import (
	"github.com/plprobelab/go-kademlia/key"
	builder "github.com/plprobelab/go-kademlia/key/sha256key256"
	"github.com/plprobelab/go-kademlia/network/address"
)

// StringID is a string-based representation of a NodeID and a NodeAddr.
// The wrapped string is used as the address of the node.
// Its key is derived using a SHA256 hash of the string value.
type StringID string

var (
	_ address.NodeAddr = (*StringID)(nil)
	_ address.NodeID   = (*StringID)(nil)
)

func NewStringID(s string) StringID {
	return StringID(s)
}

func (s StringID) String() string {
	return string(s)
}

func (s StringID) Key() key.KadKey {
	return builder.StringKadID(s.String())
}

func (s StringID) NodeID() address.NodeID {
	return s
}

func (s StringID) Addresses() []address.Addr {
	return []address.Addr{s}
}

func (s StringID) Equal(other address.NodeID) bool {
	tother, ok := other.(StringID)
	if !ok {
		return false
	}
	return s == tother
}
