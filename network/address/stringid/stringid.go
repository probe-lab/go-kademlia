package address

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

type StringID string

var _ address.NodeID[key.SHA256] = (*StringID)(nil)

func NewStringID(s string) *StringID {
	return (*StringID)(&s)
}

func (s StringID) String() string {
	return string(s)
}

func (s StringID) Key() key.SHA256 {
	return key.NewSHA256([]byte(s))
}

func (s StringID) NodeID() address.NodeID[key.SHA256] {
	return &s
}
