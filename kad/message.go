package kad

import (
	"google.golang.org/protobuf/proto"
)

type MinKadMessage interface{}

type MinKadRequestMessage[K Key[K], A Address[A]] interface {
	MinKadMessage

	// Target returns the target key and true, or false if no target key has been specfied.
	Target() K
	EmptyResponse() MinKadResponseMessage[K, A]
}

type MinKadResponseMessage[K Key[K], A Address[A]] interface {
	MinKadMessage

	CloserNodes() []NodeInfo[K, A]
}

type ProtoKadMessage interface {
	proto.Message
}

type ProtoKadRequestMessage[K Key[K], A Address[A]] interface {
	ProtoKadMessage
	MinKadRequestMessage[K, A]
}

type ProtoKadResponseMessage[K Key[K], A Address[A]] interface {
	ProtoKadMessage
	MinKadResponseMessage[K, A]
}
