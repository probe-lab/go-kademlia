package kad

// Key is the interface all Kademlia key types support.
//
// A Kademlia key is defined as a bit string of arbitrary size. In practice, different Kademlia implementations use
// different key sizes. For instance, the Kademlia paper (https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf)
// defines keys as 160-bits long and IPFS uses 256-bit keys.
//
// Keys are usually generated using cryptographic hash functions, however the specifics of key generation
// do not matter for key operations.
type Key[K any] interface {
	// BitLen returns the length of the key in bits.
	BitLen() int

	// Bit returns the value of the i'th bit of the key from most significant to least. It is equivalent to (key>>(bitlen-i-1))&1.
	// Bit will panic if i is out of the range [0,BitLen()-1].
	Bit(i int) uint

	// Xor returns the result of the eXclusive OR operation between the key and another key of the same type.
	Xor(other K) K

	// CommonPrefixLength returns the number of leading bits the key shares with another key of the same type.
	// The CommonPrefixLength of a key with itself is equal to BitLen.
	CommonPrefixLength(other K) int

	// Compare compares the numeric value of the key with another key of the same type.
	// It returns -1 if the key is numerically less than other, +1 if it is greater
	// and 0 if both keys are equal.
	Compare(other K) int
}

// RoutingTable is the interface all Kademlia Routing Tables types support.
type RoutingTable[K Key[K]] interface {
	// Self returns the local node's Kademlia key
	Self() K

	// AddPeer tries to add a peer to the routing table
	AddPeer(NodeID[K]) (bool, error)

	// RemoveKey tries to remove a peer identified by its Kademlia key from the
	// routing table
	RemoveKey(K) (NodeID[K], error)

	// NearestPeers returns the closest peers to a given key
	NearestPeers(K, int) ([]NodeID[K], error)
}

// NodeID is a generic node identifier. It is used to identify a node.
type NodeID[K Key[K]] interface {
	// Key returns the KadKey of the NodeID.
	Key() K

	// String returns the string representation of the NodeID. String
	// representation should be unique for each NodeID.
	String() string
}

// NodeAddr is a generic type that captures node ID and address information at once.
type NodeAddr[K Key[K]] interface {
	// ID returns the node identifier.
	ID() NodeID[K]

	// Addresses returns the network addresses associated with the given node.
	Addresses() []Address
}

type Address any
