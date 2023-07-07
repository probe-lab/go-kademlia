package message

import (
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"google.golang.org/protobuf/proto"
)

type MinKadMessage interface {
}

type MinKadRequestMessage interface {
	MinKadMessage

	Target() key.KadKey
	EmptyResponse() MinKadResponseMessage
}

type MinKadResponseMessage interface {
	MinKadMessage

	CloserNodes() []address.NodeAddr
}

type ProtoKadMessage interface {
	proto.Message
}

type ProtoKadRequestMessage interface {
	ProtoKadMessage
	MinKadRequestMessage
}

type ProtoKadResponseMessage interface {
	ProtoKadMessage
	MinKadResponseMessage
}
