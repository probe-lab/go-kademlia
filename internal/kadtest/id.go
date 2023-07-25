package kadtest

import (
	"crypto/sha256"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type StringID string

var _ kad.NodeID[key.Key256] = (*StringID)(nil)

func NewStringID(s string) *StringID {
	return (*StringID)(&s)
}

func (s StringID) Key() key.Key256 {
	h := sha256.New()
	h.Write([]byte(s))
	return key.NewKey256(h.Sum(nil))
}

func (s StringID) NodeID() kad.NodeID[key.Key256] {
	return &s
}

func (s StringID) String() string {
	return string(s)
}
