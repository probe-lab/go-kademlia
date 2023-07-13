package testutil

import (
	"encoding/binary"
	"math/rand"
	"strconv"

	"github.com/plprobelab/go-kademlia/key"
)

var rng = rand.New(rand.NewSource(299792458))

// Random returns a KadKey with the specificed number of bits populated with random data.
func Random(bits int) key.KadKey {
	buf := make([]byte, (bits+7)/8)
	rng.Read(buf)
	return buf
}

// RandomWithPrefix returns a KadKey with the specificed number of bits having a prefix equal to the bit pattern held in s.
// A prefix of up to 64 bits is supported.
func RandomWithPrefix(s string, bits int) key.KadKey {
	kk := Random(bits)
	if s == "" {
		return kk
	}

	prefixbits := len(s)
	if prefixbits > 64 {
		panic("RandomWithPrefix: prefix too long")
	} else if prefixbits > bits {
		panic("RandomWithPrefix: prefix longer than key length")
	}
	n, err := strconv.ParseInt(s, 2, 64)
	if err != nil {
		panic("RandomWithPrefix: " + err.Error())
	}
	prefix := uint64(n) << (64 - prefixbits)

	// sizes are in bytes
	keySize := (bits + 7) / 8
	bufSize := keySize
	if bufSize < 8 {
		bufSize = 8
	}

	buf := make([]byte, bufSize)
	rng.Read(buf)

	lead := binary.BigEndian.Uint64(buf)
	lead <<= prefixbits
	lead >>= prefixbits
	lead |= prefix
	binary.BigEndian.PutUint64(buf, lead)
	return key.KadKey(buf[:keySize])
}
