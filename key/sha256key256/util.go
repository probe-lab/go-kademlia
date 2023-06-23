package sha256key256

import (
	"crypto/sha256"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	mh "github.com/multiformats/go-multihash"
	mhreg "github.com/multiformats/go-multihash/core"
)

const (
	// HasherID is the identifier hash function used to derive the second hash
	// identifiers associated with a CID or multihash
	HasherID = mh.SHA2_256

	// Keysize is the length in bytes of the hash function's digest, which is
	// equivalent to the keysize in the Kademlia keyspace
	Keysize = sha256.Size
)

// StringKadID produces a 256-bit long KadKey from a string, using the SHA256
// hash function.
func StringKadID(s string) key.KadKey {
	// hasher is the hash function used to derive the second hash identifiers
	hasher, _ := mhreg.GetHasher(HasherID)
	hasher.Write([]byte(s))
	return key.KadKey(hasher.Sum(nil))
}
