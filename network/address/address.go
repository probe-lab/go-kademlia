package address

import "github.com/libp2p/go-libp2p-kad-dht/key"

// NodeID is a generic node identifier. It is used to identify a node and can
// also include extra information about the node, such as its network addresses.
type NodeID interface {
	// Key returns the KadKey of the NodeID.
	Key() key.KadKey
	// String returns the string representation of the NodeID. String
	// representation should be unique for each NodeID.
	String() string
}

// ProtocolID is a protocol identifier.
type ProtocolID string
