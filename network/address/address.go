package address

import "github.com/plprobelab/go-kademlia/key"

// NodeID is a generic node identifier. It is used to identify a node and can
// also include extra information about the node, such as its network addresses.
type NodeID[T key.Kademlia[T]] interface {
	// Key returns the KadKey of the NodeID.
	Key() T
	// String returns the string representation of the NodeID. String
	// representation should be unique for each NodeID.
	String() string
}

// ProtocolID is a protocol identifier.
type ProtocolID string
