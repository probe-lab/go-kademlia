package message

import (
	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"google.golang.org/protobuf/proto"
)

type MinKadMessage interface {
}

type MinKadRequestMessage interface {
	MinKadMessage

	Target() *key.KadKey
}

type MinKadResponseMessage interface {
	MinKadMessage

	CloserNodes() []address.NodeID
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
