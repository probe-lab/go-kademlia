package address

import (
	"github.com/plprobelab/go-kademlia/kad"
)

// NodeID is a generic node identifier. It is used to identify a node and can
// also include extra information about the node, such as its network addresses.
type NodeID[K kad.Key[K]] interface {
	// Key returns the KadKey of the NodeID.
	Key() K
	// String returns the string representation of the NodeID. String
	// representation should be unique for each NodeID.
	String() string
}

type Addr any

type NodeAddr[K kad.Key[K]] interface {
	NodeID() NodeID[K]

	Addresses() []Addr
}

// ProtocolID is a protocol identifier.
type ProtocolID string
