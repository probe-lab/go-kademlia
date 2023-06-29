package simmessage

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

type SimMessage struct {
	target      key.KadKey
	closerPeers []address.NodeID
}

var _ message.MinKadRequestMessage = (*SimMessage)(nil)
var _ message.MinKadResponseMessage = (*SimMessage)(nil)

func NewSimRequest(target key.KadKey) *SimMessage {
	return &SimMessage{
		target: target,
	}
}

func NewSimResponse(closerPeers []address.NodeID) *SimMessage {
	return &SimMessage{
		closerPeers: closerPeers,
	}
}

func (m *SimMessage) Target() key.KadKey {
	return m.target
}

func (m *SimMessage) EmptyResponse() message.MinKadResponseMessage {
	return &SimMessage{}
}

func (m *SimMessage) CloserNodes() []address.NodeID {
	return m.closerPeers
}
