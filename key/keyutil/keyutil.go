package keyutil

import (
	"encoding/binary"
	"math/rand"
	"strconv"

	"github.com/plprobelab/go-kademlia/key"
)

var rng = rand.New(rand.NewSource(299792458))

// Random returns a KadKey of length l populated with random data.
func Random(l int) key.KadKey {
	buf := make([]byte, l)
	rng.Read(buf)
	return buf
}

// RandomWithPrefix returns a KadKey of length l having a prefix equal to the bit pattern held in s.
func RandomWithPrefix(s string, l int) key.KadKey {
	kk := Random(l)
	if s == "" {
		return kk
	}

	bits := len(s)
	if bits > 64 {
		panic("RandomWithPrefix: prefix too long")
	}
	n, err := strconv.ParseInt(s, 2, 64)
	if err != nil {
		panic("RandomWithPrefix: " + err.Error())
	}
	prefix := uint64(n) << (64 - bits)

	size := l
	if size < 8 {
		size = 8
	}

	buf := make([]byte, size)
	rng.Read(buf)

	lead := binary.BigEndian.Uint64(buf)
	lead <<= bits
	lead >>= bits
	lead |= prefix
	binary.BigEndian.PutUint64(buf, lead)
	return key.KadKey(buf[:l])
}
