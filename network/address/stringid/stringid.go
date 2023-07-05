package address

import (
	"github.com/plprobelab/go-kademlia/key"
	builder "github.com/plprobelab/go-kademlia/key/sha256key256"
	"github.com/plprobelab/go-kademlia/network/address"
)

type StringID string

var _ address.NodeID256 = (*StringID)(nil)

func NewStringID[T any](s string) *StringID {
	return (*StringID)(&s)
}

func (s StringID) String() string {
	return string(s)
}

func (s StringID) Key() key.KadKey256 {
	return builder.StringKadID(s.String())
}

func (s StringID) NodeID() address.NodeID256 {
	return &s
}
