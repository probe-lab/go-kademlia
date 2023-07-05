package key

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	mh "github.com/multiformats/go-multihash"
	mhreg "github.com/multiformats/go-multihash/core"
	"hash"
	"math"
)

var sha256Hasher hash.Hash

func init() {
	hasher, err := mhreg.GetHasher(mh.SHA2_256)
	if err != nil {
		panic("no hasher found for SHA2_256")
	}
	sha256Hasher = hasher
}

type Kademlia[T any] interface {
	XOR(T) T
	CommonPrefixLength(T) int
	Compare(T) int8
	Equal(Kademlia[T]) bool
	BitAt(int) int
	Bytes() []byte
	String() string
	Key() T
}

type SHA256 [sha256.Size]byte

var _ Kademlia[SHA256] = (SHA256)([sha256.Size]byte{})

func NewSHA256(data []byte) SHA256 {
	sha256Hasher.Write(data)
	return SHA256(sha256Hasher.Sum(nil))
}

func (s SHA256) XOR(other SHA256) SHA256 {
	xor := [sha256.Size]byte{}
	for i := 0; i < sha256.Size; i++ {
		xor[i] = s[i] ^ other[i]
	}
	return xor
}

func (s SHA256) CommonPrefixLength(other SHA256) int {
	var xored byte
	for i := 0; i < sha256.Size; i++ {
		xored = s[i] ^ other[i]
		if xored != 0 {
			return i*8 + 7 - int(math.Log2(float64(xored)))
		}
	}
	return 8 * sha256.Size
}

// Compare returns -1 if a < b, 0 if a == b, and 1 if a > b
func (s SHA256) Compare(other SHA256) int8 {
	for i := 0; i < sha256.Size; i++ {
		if s[i] < other[i] {
			return -1
		}
		if s[i] > other[i] {
			return 1
		}
	}
	return 0
}

func (s SHA256) Equal(other Kademlia[SHA256]) bool {
	return s.Compare(other.Key()) == 0
}

func (s SHA256) BitAt(offset int) int {
	if s[offset/8]&(byte(1)<<(7-offset%8)) == 0 {
		return 0
	} else {
		return 1
	}
}

func (s SHA256) String() string {
	return hex.EncodeToString(s[:])
}

func (s SHA256) Bytes() []byte {
	return s[:]
}
func (s SHA256) Key() SHA256 {
	return s
}

type Bytes []byte

var _ Kademlia[Bytes] = (Bytes)([]byte{})

func (b Bytes) XOR(other Bytes) Bytes {
	b.panicOnUnequalSize(other)

	xor := make([]byte, len(b))
	for i := 0; i < len(b); i++ {
		xor[i] = b[i] ^ other[i]
	}
	return xor
}

func (b Bytes) CommonPrefixLength(other Bytes) int {
	b.panicOnUnequalSize(other)

	var xored byte
	for i := 0; i < len(b); i++ {
		xored = b[i] ^ other[i]
		if xored != 0 {
			return i*8 + 7 - int(math.Log2(float64(xored)))
		}
	}
	return 8 * len(b)
}

func (b Bytes) Compare(other Bytes) int8 {
	b.panicOnUnequalSize(other)

	for i := 0; i < len(b); i++ {
		if b[i] < other[i] {
			return -1
		}
		if b[i] > other[i] {
			return 1
		}
	}
	return 0
}

func (b Bytes) Equal(other Kademlia[Bytes]) bool {

	if b == nil && other.Key() == nil {
		return true
	} else if b == nil && other.Key() != nil {
		return false
	} else if b != nil && other.Key() == nil {
		return false
	}
	return b.Compare(other.Key()) == 0
}

func (b Bytes) BitAt(offset int) int {
	if b[offset/8]&(byte(1)<<(7-offset%8)) == 0 {
		return 0
	} else {
		return 1
	}
}

func (b Bytes) Bytes() []byte {
	return b
}

func (b Bytes) String() string {
	return hex.EncodeToString(b[:])
}

func (b Bytes) Key() Bytes {
	return b
}

func (b Bytes) panicOnUnequalSize(other Bytes) {
	if len(other) != len(b) {
		panic(fmt.Sprintf("kademlia key bytes have different sizes: %d != %d", len(other), len(b)))
	}
}
