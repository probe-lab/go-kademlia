package address

import "github.com/plprobelab/go-kademlia/key"

// A NodeID identifies a node in a Kademlia network.
type NodeID interface {
	// Key returns the Kademlia key of the node.
	Key() key.KadKey

	// String returns the string representation of the NodeID.
	// It should be unique for each distinct NodeID.
	String() string

	// Equal reports whether another NodeID represents the same identifier.
	Equal(NodeID) bool
}

// A NodeAddr carries network addressing information for a node.
type NodeAddr interface {
	NodeID() NodeID

	Addresses() []Addr
}

// Addr is a network address.
type Addr any

// ProtocolID is a protocol identifier.
type ProtocolID string
