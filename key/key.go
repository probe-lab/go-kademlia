package key

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/plprobelab/go-kademlia/kad"
)

const bitPanicMsg = "bit index out of range"

// Key256 is a 256-bit Kademlia key.
type Key256 struct {
	b *[32]byte // this is a pointer to keep the size of Key256 small since it is often passed as argument
}

var _ kad.Key[Key256] = Key256{}

// NewKey256 returns a 256-bit Kademlia key whose bits are set from the supplied bytes.
func NewKey256(data []byte) Key256 {
	if len(data) != 32 {
		panic("invalid data length for key")
	}
	var b [32]byte
	copy(b[:], data)
	return Key256{b: &b}
}

// ZeroKey256 returns a 256-bit Kademlia key with all bits zeroed.
func ZeroKey256() Key256 {
	var b [32]byte
	return Key256{b: &b}
}

// Bit returns the value of the i'th bit of the key from most significant to least.
func (k Key256) Bit(i int) uint {
	if i < 0 || i > 255 {
		panic(bitPanicMsg)
	}
	if k.b == nil {
		return 0
	}
	if k.b[i/8]&(byte(1)<<(7-i%8)) == 0 {
		return 0
	} else {
		return 1
	}
}

// BitLen returns the length of the key in bits, which is always 256.
func (Key256) BitLen() int {
	return 256
}

// Xor returns the result of the eXclusive OR operation between the key and another key of the same type.
func (k Key256) Xor(o Key256) Key256 {
	var xored [32]byte
	if k.b != nil && o.b != nil {
		for i := 0; i < 32; i++ {
			xored[i] = k.b[i] ^ o.b[i]
		}
	} else if k.b != nil && o.b == nil {
		copy(xored[:], k.b[:])
	} else if k.b == nil && o.b != nil {
		copy(xored[:], o.b[:])
	}
	return Key256{b: &xored}
}

// CommonPrefixLength returns the number of leading bits the key shares with another key of the same type.
func (k Key256) CommonPrefixLength(o Key256) int {
	if k.b == nil || o.b == nil {
		return 256
	}
	var x byte
	for i := 0; i < 32; i++ {
		x = k.b[i] ^ o.b[i]
		if x != 0 {
			return i*8 + 7 - int(math.Log2(float64(x))) // TODO: make this more efficient
		}
	}
	return 256
}

// Compare compares the numeric value of the key with another key of the same type.
func (k Key256) Compare(o Key256) int {
	return bytes.Compare(k.b[:], o.b[:])
}

// HexString returns a string containing the hexadecimal representation of the key.
func (k Key256) HexString() string {
	if k.b == nil {
		return ""
	}
	return hex.EncodeToString(k.b[:])
}

// Key32 is a 32-bit Kademlia key, suitable for testing and simulation of small networks.
type Key32 uint32

var _ kad.Key[Key32] = Key32(0)

// BitLen returns the length of the key in bits, which is always 32.
func (Key32) BitLen() int {
	return 32
}

// Bit returns the value of the i'th bit of the key from most significant to least.
func (k Key32) Bit(i int) uint {
	if i < 0 || i > 31 {
		panic(bitPanicMsg)
	}
	return uint((k >> (31 - i)) & 1)
}

// Xor returns the result of the eXclusive OR operation between the key and another key of the same type.
func (k Key32) Xor(o Key32) Key32 {
	return k ^ o
}

// CommonPrefixLength returns the number of leading bits the key shares with another key of the same type.
func (k Key32) CommonPrefixLength(o Key32) int {
	a := uint32(k)
	b := uint32(o)
	for i := 32; i > 0; i-- {
		if a == b {
			return i
		}
		a >>= 1
		b >>= 1
	}
	return 0
}

// Compare compares the numeric value of the key with another key of the same type.
func (k Key32) Compare(o Key32) int {
	if k < o {
		return -1
	} else if k > o {
		return 1
	}
	return 0
}

// HexString returns a string containing the hexadecimal representation of the key.
func (k Key32) HexString() string {
	return fmt.Sprintf("%04x", uint32(k))
}

// BitString returns a string containing the binary representation of the key.
func (k Key32) BitString() string {
	return fmt.Sprintf("%032b", uint32(k))
}

func (k Key32) String() string {
	return k.HexString()
}

// Key8 is an 8-bit Kademlia key, suitable for testing and simulation of very small networks.
type Key8 uint8

var _ kad.Key[Key8] = Key8(0)

// BitLen returns the length of the key in bits, which is always 8.
func (Key8) BitLen() int {
	return 8
}

// Bit returns the value of the i'th bit of the key from most significant to least.
func (k Key8) Bit(i int) uint {
	if i < 0 || i > 7 {
		panic(bitPanicMsg)
	}
	return uint((k >> (7 - i)) & 1)
}

// Xor returns the result of the eXclusive OR operation between the key and another key of the same type.
func (k Key8) Xor(o Key8) Key8 {
	return k ^ o
}

// CommonPrefixLength returns the number of leading bits the key shares with another key of the same type.
func (k Key8) CommonPrefixLength(o Key8) int {
	a := uint8(k)
	b := uint8(o)
	for i := 8; i > 0; i-- {
		if a == b {
			return i
		}
		a >>= 1
		b >>= 1
	}
	return 0
}

// Compare compares the numeric value of the key with another key of the same type.
func (k Key8) Compare(o Key8) int {
	if k < o {
		return -1
	} else if k > o {
		return 1
	}
	return 0
}

// HexString returns a string containing the hexadecimal representation of the key.
func (k Key8) HexString() string {
	return fmt.Sprintf("%x", uint8(k))
}

func (k Key8) String() string {
	return k.HexString()
}

// HexString returns a string containing the binary representation of the key.
func (k Key8) BitString() string {
	return fmt.Sprintf("%08b", uint8(k))
}

// KeyList is a list of Kademlia keys. It implements sort.Interface.
type KeyList[K kad.Key[K]] []K

func (ks KeyList[K]) Len() int           { return len(ks) }
func (ks KeyList[K]) Swap(i, j int)      { ks[i], ks[j] = ks[j], ks[i] }
func (ks KeyList[K]) Less(i, j int) bool { return ks[i].Compare(ks[j]) < 0 }
