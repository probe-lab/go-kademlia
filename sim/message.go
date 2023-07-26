package sim

import (
	"github.com/plprobelab/go-kademlia/kad"
)

// Message is a simple implementation of `Request` and `Response`.
// It only contains the minimal fields that are required by Kademlia to operate.
type Message[K kad.Key[K], A kad.Address[A]] struct {
	target      K
	closerPeers []kad.NodeInfo[K, A]
}

func NewRequest[K kad.Key[K], A kad.Address[A]](target K) *Message[K, A] {
	return &Message[K, A]{
		target: target,
	}
}

func NewResponse[K kad.Key[K], A kad.Address[A]](closerPeers []kad.NodeInfo[K, A]) *Message[K, A] {
	return &Message[K, A]{
		closerPeers: closerPeers,
	}
}

func (m *Message[K, A]) Target() K {
	return m.target
}

func (m *Message[K, A]) EmptyResponse() kad.Response[K, A] {
	return &Message[K, A]{}
}

func (m *Message[K, A]) CloserNodes() []kad.NodeInfo[K, A] {
	return m.closerPeers
}
