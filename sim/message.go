package sim

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/message"
)

// Message is a simple implementation of `MinKadRequestMessage` and `MinKadResponseMessage`.
// It only contains the minimal fields that are required by Kademlia to operate.
type Message[K kad.Key[K], A any] struct {
	target      K
	closerPeers []kad.NodeInfo[K, A]
}

func NewRequest[K kad.Key[K], A any](target K) *Message[K, A] {
	return &Message[K, A]{
		target: target,
	}
}

func NewResponse[K kad.Key[K], A any](closerPeers []kad.NodeInfo[K, A]) *Message[K, A] {
	return &Message[K, A]{
		closerPeers: closerPeers,
	}
}

func (m *Message[K, A]) Target() K {
	return m.target
}

func (m *Message[K, A]) EmptyResponse() message.MinKadResponseMessage[K, A] {
	return &Message[K, A]{}
}

func (m *Message[K, A]) CloserNodes() []kad.NodeInfo[K, A] {
	return m.closerPeers
}
