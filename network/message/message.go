package message

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"google.golang.org/protobuf/proto"
)

type MinKadMessage interface{}

type MinKadRequestMessage[K kad.Key[K]] interface {
	MinKadMessage

	// Target returns the target key and true, or false if no target key has been specfied.
	Target() K
	EmptyResponse() MinKadResponseMessage[K]
}

type MinKadResponseMessage[K kad.Key[K]] interface {
	MinKadMessage

	CloserNodes() []address.NodeAddr[K]
}

type ProtoKadMessage interface {
	proto.Message
}

type ProtoKadRequestMessage interface {
	ProtoKadMessage
	MinKadRequestMessage[key.Key256]
}

type ProtoKadResponseMessage interface {
	ProtoKadMessage
	MinKadResponseMessage[key.Key256]
}
