package sim

import (
	"github.com/plprobelab/go-kademlia/kad"
)

// Message is a simple implementation of `Request` and `Response`.
// It only contains the minimal fields that are required by Kademlia to operate.
type Message[K kad.Key[K], N kad.NodeID[K], A kad.Address[A]] struct {
	target      K
	closerPeers []kad.NodeInfo[K, N, A]
}

func NewRequest[K kad.Key[K], N kad.NodeID[K], A kad.Address[A]](target K) *Message[K, N, A] {
	return &Message[K, N, A]{
		target: target,
	}
}

func NewResponse[K kad.Key[K], N kad.NodeID[K], A kad.Address[A]](closerPeers []kad.NodeInfo[K, N, A]) *Message[K, N, A] {
	return &Message[K, N, A]{
		closerPeers: closerPeers,
	}
}

func (m *Message[K, N, A]) Target() K {
	return m.target
}

func (m *Message[K, N, A]) EmptyResponse() kad.Response[K, N, A] {
	return &Message[K, N, A]{}
}

func (m *Message[K, N, A]) CloserNodes() []kad.NodeInfo[K, N, A] {
	return m.closerPeers
}
