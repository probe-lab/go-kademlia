package key

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-libdht/kad"
)

func TestKey256(t *testing.T) {
	tester := &KeyTester[Key256]{
		// kt.Key0 is 00000...000
		Key0: ZeroKey256(),

		// key1 is key0 + 1 (00000...001)
		Key1: NewKey256([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}),

		// key2 is key0 + 2 (00000...010)
		Key2: NewKey256([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02}),

		// key1xor2 is key1 ^ key2 (00000...011)
		Key1xor2: NewKey256([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03}),

		// key100 is key0 with the most significant bit set (10000...000)
		Key100: NewKey256([]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),

		// key010 is key0 with the second most significant bit set (01000...000)
		Key010: NewKey256([]byte{0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),

		KeyX: NewKey256([]byte{0x23, 0xe4, 0xdd, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
	}

	tester.RunTests(t)

	testBinaryMarshaler(t, tester.KeyX, NewKey256)
}

func TestKey32(t *testing.T) {
	tester := &KeyTester[Key32]{
		Key0:     Key32(0),
		Key1:     Key32(1),
		Key2:     Key32(2),
		Key1xor2: Key32(3),
		Key100:   Key32(0x80000000),
		Key010:   Key32(0x40000000),
		KeyX:     Key32(0x23e4dd03),
	}

	tester.RunTests(t)
}

func TestKey8(t *testing.T) {
	tester := &KeyTester[Key8]{
		Key0:     Key8(0),
		Key1:     Key8(1),
		Key2:     Key8(2),
		Key1xor2: Key8(3),
		Key100:   Key8(0x80),
		Key010:   Key8(0x40),
		KeyX:     Key8(0x23),
	}

	tester.RunTests(t)
}

// TestBitStrKey7 tests a strange 7-bit binary key
func TestBitStrKey7(t *testing.T) {
	tester := &KeyTester[BitStrKey]{
		Key0:     BitStrKey("0000000"),
		Key1:     BitStrKey("0000001"),
		Key2:     BitStrKey("0000010"),
		Key1xor2: BitStrKey("0000011"),
		Key100:   BitStrKey("1000000"),
		Key010:   BitStrKey("0100000"),
		KeyX:     BitStrKey("1010110"),
	}

	tester.RunTests(t)
}

// KeyTester tests a kad.Key's implementation
type KeyTester[K kad.Key[K]] struct {
	// Key 0 is zero
	Key0 K

	// Key1 is Key0 + 1 (00000...001)
	Key1 K

	// Key2 is Key0 + 2 (00000...010)
	Key2 K

	// Key1xor2 is Key1 ^ Key2 (00000...011)
	Key1xor2 K

	// Key100 is Key0 with the most significant bit set (10000...000)
	Key100 K

	// Key010 is Key0 with the second most significant bit set (01000...000)
	Key010 K

	// KeyX is a random key
	KeyX K
}

func (kt *KeyTester[K]) RunTests(t *testing.T) {
	t.Helper()
	t.Run("Xor", kt.TestXor)
	t.Run("CommonPrefixLength", kt.TestCommonPrefixLength)
	t.Run("Compare", kt.TestCompare)
	t.Run("Bit", kt.TestBit)
	t.Run("BitString", kt.TestBitString)
	t.Run("HexString", kt.TestHexString)
}

func (kt *KeyTester[K]) TestXor(t *testing.T) {
	xored := kt.Key0.Xor(kt.Key0)
	require.Equal(t, kt.Key0, xored)

	xored = kt.KeyX.Xor(kt.Key0)
	require.Equal(t, kt.KeyX, xored)

	xored = kt.Key0.Xor(kt.KeyX)
	require.Equal(t, kt.KeyX, xored)

	xored = kt.KeyX.Xor(kt.KeyX)
	require.Equal(t, kt.Key0, xored)

	xored = kt.Key1.Xor(kt.Key2)
	require.Equal(t, kt.Key1xor2, xored)

	var empty K // zero value of key
	xored = kt.Key0.Xor(empty)
	require.Equal(t, kt.Key0, xored)
	xored = empty.Xor(kt.Key0)
	require.Equal(t, kt.Key0, xored)
}

func (kt *KeyTester[K]) TestCommonPrefixLength(t *testing.T) {
	cpl := kt.Key0.CommonPrefixLength(kt.Key0)
	require.Equal(t, kt.Key0.BitLen(), cpl)

	cpl = kt.Key0.CommonPrefixLength(kt.Key1)
	require.Equal(t, kt.Key0.BitLen()-1, cpl)

	cpl = kt.Key0.CommonPrefixLength(kt.Key100)
	require.Equal(t, 0, cpl)

	cpl = kt.Key0.CommonPrefixLength(kt.Key010)
	require.Equal(t, 1, cpl)

	var empty K // zero value of key
	cpl = kt.Key0.CommonPrefixLength(empty)
	require.Equal(t, kt.Key0.BitLen(), cpl)
	cpl = empty.CommonPrefixLength(kt.Key0)
	require.Equal(t, kt.Key0.BitLen(), cpl)
}

func (kt *KeyTester[K]) TestCompare(t *testing.T) {
	res := kt.Key0.Compare(kt.Key0)
	require.Equal(t, 0, res)

	res = kt.Key0.Compare(kt.Key1)
	require.Equal(t, -1, res)

	res = kt.Key0.Compare(kt.Key2)
	require.Equal(t, -1, res)

	res = kt.Key0.Compare(kt.Key100)
	require.Equal(t, -1, res)

	res = kt.Key0.Compare(kt.Key010)
	require.Equal(t, -1, res)

	res = kt.Key1.Compare(kt.Key1)
	require.Equal(t, 0, res)

	res = kt.Key1.Compare(kt.Key0)
	require.Equal(t, 1, res)

	res = kt.Key1.Compare(kt.Key2)
	require.Equal(t, -1, res)

	var empty K // zero value of key
	res = kt.Key0.Compare(empty)
	require.Equal(t, 0, res)
	res = empty.Compare(kt.Key0)
	require.Equal(t, 0, res)
}

func (kt *KeyTester[K]) TestBit(t *testing.T) {
	for i := 0; i < kt.Key0.BitLen(); i++ {
		require.Equal(t, uint(0), kt.Key0.Bit(i), fmt.Sprintf("Key0.Bit(%d)=%d", i, kt.Key0.Bit(i)))
	}

	for i := 0; i < kt.Key1.BitLen()-1; i++ {
		require.Equal(t, uint(0), kt.Key1.Bit(i), fmt.Sprintf("Key1.Bit(%d)=%d", i, kt.Key1.Bit(i)))
	}
	require.Equal(t, uint(1), kt.Key1.Bit(kt.Key0.BitLen()-1), fmt.Sprintf("Key1.Bit(%d)=%d", kt.Key1.BitLen()-1, kt.Key1.Bit(kt.Key1.BitLen()-1)))

	for i := 0; i < kt.Key0.BitLen()-2; i++ {
		require.Equal(t, uint(0), kt.Key2.Bit(i), fmt.Sprintf("Key1.Bit(%d)=%d", i, kt.Key2.Bit(i)))
	}
	require.Equal(t, uint(1), kt.Key2.Bit(kt.Key2.BitLen()-2), fmt.Sprintf("Key1.Bit(%d)=%d", kt.Key2.BitLen()-2, kt.Key2.BitLen()-2))
	require.Equal(t, uint(0), kt.Key2.Bit(kt.Key2.BitLen()-1), fmt.Sprintf("Key1.Bit(%d)=%d", kt.Key2.BitLen()-2, kt.Key2.BitLen()-1))

	var empty K // zero value of key
	for i := 0; i < empty.BitLen(); i++ {
		require.Equal(t, uint(0), empty.Bit(i), fmt.Sprintf("empty.Bit(%d)=%d", i, kt.Key0.Bit(i)))
	}
}

func (kt *KeyTester[K]) TestBitString(t *testing.T) {
	str := BitString(kt.KeyX)
	t.Logf("BitString(%v)=%s", kt.KeyX, str)
	for i := 0; i < kt.KeyX.BitLen(); i++ {
		expected := byte('0')
		if kt.KeyX.Bit(i) == 1 {
			expected = byte('1')
		}
		require.Equal(t, string(expected), string(str[i]))
	}
}

func (kt *KeyTester[K]) TestHexString(t *testing.T) {
	str := HexString(kt.KeyX)
	t.Logf("HexString(%v)=%s", kt.KeyX, str)

	bitpos := kt.KeyX.BitLen() - 1

	for i := len(str) - 1; i >= 0; i-- {
		v, err := strconv.ParseInt(string(str[i]), 16, 8)
		require.NoError(t, err)
		mask := uint(0x1)
		for b := 0; b < 4; b++ {
			got := (uint(v) & mask) >> b
			want := kt.KeyX.Bit(bitpos)
			require.Equal(t, want, got, fmt.Sprintf("bit %d: (%04b & %04b)>>%d = %d, wanted kt.KeyX.Bit(%d)=%d", bitpos, uint(v), b, mask, (uint(v)&mask), bitpos, want))
			bitpos--
			if bitpos < 0 {
				break
			}
			mask <<= 1
		}

		if bitpos < 0 && i > 0 {
			t.Errorf("hex string had length %d, but expected %d", len(str), (kt.KeyX.BitLen()+3)/4)
			break
		}
	}

	if bitpos >= 0 {
		t.Errorf("hex string had length %d, but expected %d", len(str), (kt.KeyX.BitLen()+3)/4)
	}
}

// testBinaryMarshaler tests the behaviour of a kad.Key implementation that also implements the BinaryMarshaler interface
func testBinaryMarshaler[K interface {
	kad.Key[K]
	MarshalBinary() ([]byte, error)
}](t *testing.T, k K, newFunc func([]byte) K,
) {
	b, err := k.MarshalBinary()
	require.NoError(t, err)

	other := newFunc(b)

	res := k.Compare(other)
	require.Equal(t, 0, res)
}

// BitStrKey is a key represented by a string of 1's and 0's
type BitStrKey string

var _ kad.Key[BitStrKey] = BitStrKey("1010")

func (k BitStrKey) BitLen() int {
	return len(k)
}

func (k BitStrKey) Bit(i int) uint {
	if i < 0 || i > len(k) {
		panic(bitPanicMsg)
	}
	if k[i] == '1' {
		return 1
	} else if k[i] == '0' {
		return 0
	}
	panic("BitStrKey: not a binary string")
}

func (k BitStrKey) Xor(o BitStrKey) BitStrKey {
	if len(k) != len(o) {
		if len(k) == 0 && o.isZero() {
			return BitStrKey(o)
		}
		if len(o) == 0 && k.isZero() {
			return BitStrKey(k)
		}
		panic("BitStrKey: other key has different length")
	}
	buf := make([]byte, len(k))
	for i := range buf {
		if k[i] != o[i] {
			buf[i] = '1'
		} else {
			buf[i] = '0'
		}
	}
	return BitStrKey(string(buf))
}

func (k BitStrKey) CommonPrefixLength(o BitStrKey) int {
	if len(k) != len(o) {
		if len(k) == 0 && o.isZero() {
			return len(o)
		}
		if len(o) == 0 && k.isZero() {
			return len(k)
		}
		panic("BitStrKey: other key has different length")
	}
	for i := 0; i < len(k); i++ {
		if k[i] != o[i] {
			return i
		}
	}
	return len(k)
}

func (k BitStrKey) Compare(o BitStrKey) int {
	if len(k) != len(o) {
		if len(k) == 0 && o.isZero() {
			return 0
		}
		if len(o) == 0 && k.isZero() {
			return 0
		}
		panic("BitStrKey: other key has different length")
	}
	for i := 0; i < len(k); i++ {
		if k[i] != o[i] {
			if k[i] < o[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func (k BitStrKey) isZero() bool {
	for i := 0; i < len(k); i++ {
		if k[i] != '0' {
			return false
		}
	}
	return true
}
