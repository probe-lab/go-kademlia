package key

import (
	"encoding/hex"
	"math"
)

type KadKey []byte

func (k KadKey) Size() int {
	return len(k)
}

func (k KadKey) Hex() string {
	return hex.EncodeToString(k[:])
}

func (k KadKey) String() string {
	return k.Hex()
}

func shortLong(a, b KadKey) (min, max KadKey) {
	if len(a) < len(b) {
		return a, b
	}
	return b, a
}

func (a KadKey) Xor(b KadKey) KadKey {
	short, long := shortLong(a, b)
	xored := make([]byte, len(long))
	for i := 0; i < len(short); i++ {
		xored[i] = a[i] ^ b[i]
	}
	copy(xored[len(short):], long[len(short):])
	return xored
}

func (a KadKey) CommonPrefixLength(b KadKey) int {
	short, _ := shortLong(a, b)

	var xored byte
	for i := 0; i < len(short); i++ {
		xored = a[i] ^ b[i]
		if xored != 0 {
			return i*8 + 7 - int(math.Log2(float64(xored)))
		}
	}
	return 8 * len(short)
}

// Compare returns -1 if a < b, 0 if a == b, and 1 if a > b
func (a KadKey) Compare(b KadKey) int {
	short, _ := shortLong(a, b)

	for i := 0; i < len(short); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) == len(b) {
		return 0
	} else if len(a) < len(b) {
		// if both keys don't have the same size, and the shorter is a prefix
		// of the longer, then the shorter is considered smaller
		return -1
	} else {
		return 1
	}
}

func (a KadKey) Equal(b KadKey) bool {
	return a.Compare(b) == 0
}

func (k KadKey) BitAt(offset int) int {
	if k[offset/8]&(byte(1)<<(7-offset%8)) == 0 {
		return 0
	} else {
		return 1
	}
}
