package libp2p

import (
	"google.golang.org/protobuf/proto"

	"github.com/plprobelab/go-kademlia/kad"
)

type ProtoKadMessage interface {
	proto.Message
}

type ProtoKadRequestMessage[K kad.Key[K], A kad.Address[A]] interface {
	ProtoKadMessage
	kad.Request[K, A]
}

type ProtoKadResponseMessage[K kad.Key[K], A kad.Address[A]] interface {
	ProtoKadMessage
	kad.Response[K, A]
}
