package key

import (
	"bytes"
	"encoding/hex"
	"math"
	"unsafe"
)

type KadKey[T any] struct {
	bytes []byte
}

type KadKey256 = KadKey[[256]byte]

func New[T any](data []byte) KadKey[T] {
	var v T
	if uintptr(len(data)) != unsafe.Sizeof(v) {
		panic("invalid data length for key")
	}
	return KadKey[T]{bytes: data}
}

func (k KadKey[T]) Size() int {
	return len(k.bytes)
}

func (k KadKey[T]) Hex() string {
	return hex.EncodeToString(k.bytes[:])
}

func (k KadKey[T]) String() string {
	return k.Hex()
}

func (a KadKey[T]) Xor(b KadKey[T]) KadKey[T] {
	xored := make([]byte, a.Size())
	for i := 0; i < a.Size(); i++ {
		xored[i] = a.bytes[i] ^ b.bytes[i]
	}
	return KadKey[T]{bytes: xored}
}

func (a KadKey[T]) CommonPrefixLength(b KadKey[T]) int {
	var xored byte
	for i := 0; i < a.Size(); i++ {
		xored = a.bytes[i] ^ b.bytes[i]
		if xored != 0 {
			return i*8 + 7 - int(math.Log2(float64(xored)))
		}
	}
	return 8 * a.Size()
}

// Compare returns -1 if a < b, 0 if a == b, and 1 if a > b
func (a KadKey[T]) Compare(b KadKey[T]) int {
	return bytes.Compare(a.bytes, b.bytes)
}

func (a KadKey[T]) Equal(b KadKey[T]) bool {
	return a.Compare(b) == 0
}
