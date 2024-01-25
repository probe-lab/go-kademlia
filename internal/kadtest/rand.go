package kadtest

import (
	"math/rand"
	"strconv"

	"github.com/probe-lab/go-kademlia/key"
)

var rng = rand.New(rand.NewSource(299792458))

// RandomKey returns a random 32-bit Kademlia key.
func RandomKey() key.Key32 {
	return key.Key32(rng.Uint32())
}

// RandomKeyWithPrefix returns a 32-bit Kademlia key having a prefix equal to the bit pattern held in s and
// random following bits. A prefix of up to 32 bits is supported.
func RandomKeyWithPrefix(s string) key.Key32 {
	kk := RandomKey()
	if s == "" {
		return kk
	}

	prefixbits := len(s)
	if prefixbits > 32 {
		panic("RandomKeyWithPrefix: prefix too long")
	}
	n, err := strconv.ParseInt(s, 2, 32)
	if err != nil {
		panic("RandomKeyWithPrefix: " + err.Error())
	}
	prefix := uint32(n) << (32 - prefixbits)

	v := uint32(kk) << prefixbits
	v >>= prefixbits

	return key.Key32(v | prefix)
}

// Key256WithLeadingBytes returns a 256-bit Kademlia key consisting of the given leading bytes padded by
// zero bytes to the end of the key.
func Key256WithLeadingBytes(in []byte) key.Key256 {
	return key.NewKey256(append(in, make([]byte, 32-len(in))...))
}
