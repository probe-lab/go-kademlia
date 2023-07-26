package sim

import (
	"github.com/plprobelab/go-kademlia/kad"
)

// SimMessage is a simple implementation of `Request` and `Response`.
// It only contains the minimal fields that are required by Kademlia to operate.
type SimMessage[K kad.Key[K], A kad.Address[A]] struct {
	target      K
	closerPeers []kad.NodeInfo[K, A]
}

func NewRequest[K kad.Key[K], A kad.Address[A]](target K) *SimMessage[K, A] {
	return &SimMessage[K, A]{
		target: target,
	}
}

func NewResponse[K kad.Key[K], A kad.Address[A]](closerPeers []kad.NodeInfo[K, A]) *SimMessage[K, A] {
	return &SimMessage[K, A]{
		closerPeers: closerPeers,
	}
}

func (m *SimMessage[K, A]) Target() K {
	return m.target
}

func (m *SimMessage[K, A]) EmptyResponse() kad.Response[K, A] {
	return &SimMessage[K, A]{}
}

func (m *SimMessage[K, A]) CloserNodes() []kad.NodeInfo[K, A] {
	return m.closerPeers
}
