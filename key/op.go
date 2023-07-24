package key

import (
	"strings"

	"github.com/plprobelab/go-kademlia/kad"
)

// Equal reports whether two keys have equal numeric values.
func Equal[K kad.Key[K]](a, b K) bool {
	return a.Compare(b) == 0
}

// BitString returns a string containing the binary representation of a key.
func BitString[K kad.Key[K]](k K) string {
	if bs, ok := any(k).(interface{ BitString() string }); ok {
		return bs.BitString()
	}
	b := new(strings.Builder)
	b.Grow(k.BitLen())
	for i := 0; i < k.BitLen(); i++ {
		if k.Bit(i) == 0 {
			b.WriteByte('0')
		} else {
			b.WriteByte('1')
		}
	}
	return b.String()
}

// HexString returns a string containing the hexadecimal representation of a key.
func HexString[K kad.Key[K]](k K) string {
	if hs, ok := any(k).(interface{ HexString() string }); ok {
		return hs.HexString()
	}
	b := new(strings.Builder)
	b.Grow((k.BitLen() + 3) / 4)

	h := [...]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

	prebits := k.BitLen() % 4
	// TODO: deal with initial nibble
	for i := prebits; i < k.BitLen(); i += 4 {
		var n byte
		n |= byte(k.Bit(i)) << 3
		n |= byte(k.Bit(i+1)) << 2
		n |= byte(k.Bit(i+2)) << 1
		n |= byte(k.Bit(i + 3))
		b.WriteByte(h[n])
	}
	return b.String()
}
