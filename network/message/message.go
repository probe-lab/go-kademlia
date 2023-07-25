package message

import (
	"github.com/plprobelab/go-kademlia/kad"
	"google.golang.org/protobuf/proto"
)

type MinKadMessage interface{}

type MinKadRequestMessage[K kad.Key[K], A kad.Address[A]] interface {
	MinKadMessage

	// Target returns the target key and true, or false if no target key has been specfied.
	Target() K
	EmptyResponse() MinKadResponseMessage[K, A]
}

type MinKadResponseMessage[K kad.Key[K], A kad.Address[A]] interface {
	MinKadMessage

	CloserNodes() []kad.NodeInfo[K, A]
}

type ProtoKadMessage interface {
	proto.Message
}

type ProtoKadRequestMessage[K kad.Key[K], A kad.Address[A]] interface {
	ProtoKadMessage
	MinKadRequestMessage[K, A]
}

type ProtoKadResponseMessage[K kad.Key[K], A kad.Address[A]] interface {
	ProtoKadMessage
	MinKadResponseMessage[K, A]
}
