package kadid

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

type KadID[T any] struct {
	key.KadKey[T]
}

// var _ address.NodeID[T] = (*KadID[T])(nil)

func NewKadID[T any](k key.KadKey[T]) *KadID[T] {
	return &KadID[T]{k}
}

func (k KadID[T]) Key() key.KadKey[T] {
	return k.KadKey
}

func (k KadID[T]) NodeID() address.NodeID[T] {
	return &k
}

func (k KadID[T]) String() string {
	return k.Hex()
}
