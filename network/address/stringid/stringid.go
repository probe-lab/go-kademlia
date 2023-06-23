package address

import (
	"github.com/libp2p/go-libp2p-kad-dht/key"
	builder "github.com/libp2p/go-libp2p-kad-dht/key/sha256key256"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
)

type StringID string

var _ address.NodeID = (*StringID)(nil)

func NewStringID(s string) *StringID {
	return (*StringID)(&s)
}

func (s StringID) String() string {
	return string(s)
}

func (s StringID) Key() key.KadKey {
	return builder.StringKadID(s.String())
}

func (s StringID) NodeID() address.NodeID {
	return &s
}
