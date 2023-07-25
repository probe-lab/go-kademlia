package sim

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

// Message is a simple implementation of `MinKadRequestMessage` and `MinKadResponseMessage`.
// It only contains the minimal fields that are required by Kademlia to operate.
type Message[K kad.Key[K]] struct {
	target      K
	closerPeers []address.NodeAddr[K]
}

func NewRequest[K kad.Key[K]](target K) *Message[K] {
	return &Message[K]{
		target: target,
	}
}

func NewResponse[K kad.Key[K]](closerPeers []address.NodeAddr[K]) *Message[K] {
	return &Message[K]{
		closerPeers: closerPeers,
	}
}

func (m *Message[K]) Target() K {
	return m.target
}

func (m *Message[K]) EmptyResponse() message.MinKadResponseMessage[K] {
	return &Message[K]{}
}

func (m *Message[K]) CloserNodes() []address.NodeAddr[K] {
	return m.closerPeers
}
