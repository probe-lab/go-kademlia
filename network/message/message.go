package message

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"google.golang.org/protobuf/proto"
)

type MinKadMessage interface{}

type MinKadRequestMessage[K kad.Key[K], A any] interface {
	MinKadMessage

	// Target returns the target key and true, or false if no target key has been specfied.
	Target() K
	EmptyResponse() MinKadResponseMessage[K, A]
}

type MinKadResponseMessage[K kad.Key[K], A any] interface {
	MinKadMessage

	CloserNodes() []kad.NodeInfo[K, A]
}

type ProtoKadMessage interface {
	proto.Message
}

type ProtoKadRequestMessage[A any] interface {
	ProtoKadMessage
	MinKadRequestMessage[key.Key256, A]
}

type ProtoKadResponseMessage[A any] interface {
	ProtoKadMessage
	MinKadResponseMessage[key.Key256, A]
}
