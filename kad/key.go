package kad

// Key is the interface all Kademlia key types support.
//
// A Kademlia key is defined as a bit string of arbitrary size. In practice different Kademlia implementations use
// different key sizes. For instance, the Kademlia paper (https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf)
// defines keys as 160-bits long and IPFS uses 256-bit keys.
//
// Keys are usually generated using cryptographic hash functions, however the specifics of key generation
// do not matter for key operations.
type Key[K any] interface {
	// BitLen returns the length of the key in bits.
	BitLen() int

	// Bit returns the value of the i'th bit of the key from most significant to least. It is equivalent to (key>>(bitlen-i-1))&1.
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
