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

func (a KadKey) Xor(b KadKey) (KadKey, error) {
	if a.Size() != b.Size() {
		return nil, ErrInvalidKey(a.Size())
	}

	xored := make([]byte, a.Size())
	for i := 0; i < a.Size(); i++ {
		xored[i] = a[i] ^ b[i]
	}
	return xored, nil
}

func (a KadKey) CommonPrefixLength(b KadKey) (int, error) {
	if a.Size() != b.Size() {
		return 0, ErrInvalidKey(a.Size())
	}

	var xored byte
	for i := 0; i < a.Size(); i++ {
		xored = a[i] ^ b[i]
		if xored != 0 {
			return i*8 + 7 - int(math.Log2(float64(xored))), nil
		}
	}
	return 8 * a.Size(), nil
}

// Compare returns -1 if a < b, 0 if a == b, and 1 if a > b
func (a KadKey) Compare(b KadKey) (int8, error) {
	if a.Size() != b.Size() {
		return 2, ErrInvalidKey(a.Size())
	}

	for i := 0; i < a.Size(); i++ {
		if a[i] < b[i] {
			return -1, nil
		}
		if a[i] > b[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func (a KadKey) Equal(b KadKey) (bool, error) {
	cmp, err := a.Compare(b)
	return cmp == 0, err
}
