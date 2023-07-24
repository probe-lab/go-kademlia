package simmessage

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

type SimMessage[K kad.Key[K]] struct {
	target      K
	closerPeers []address.NodeAddr[K]
}

func NewSimRequest[K kad.Key[K]](target K) *SimMessage[K] {
	return &SimMessage[K]{
		target: target,
	}
}

func NewSimResponse[K kad.Key[K]](closerPeers []address.NodeAddr[K]) *SimMessage[K] {
	return &SimMessage[K]{
		closerPeers: closerPeers,
	}
}

func (m *SimMessage[K]) Target() K {
	return m.target
}

func (m *SimMessage[K]) EmptyResponse() message.MinKadResponseMessage[K] {
	return &SimMessage[K]{}
}

func (m *SimMessage[K]) CloserNodes() []address.NodeAddr[K] {
	return m.closerPeers
}
