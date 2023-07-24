package key

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/kad"
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
	// t.Run("HexString", kt.TestHexString)
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

// func (kt *KeyTester[K]) TestHexString(t *testing.T) {
// 	str := HexString(kt.KeyX)

// 	bytelen := (kt.KeyX.BitLen() + 3) / 4
// 	require.Equal(t, bytelen, len(str))

// 	for i := 0; i < bytelen; i++ {

// 		for j := 0; j < 8; j++ {
// 			bitpos := kt.KeyX.BitLen() - (i*8 + j)
// 			if bitpos >
// 			require.GreaterOrEqual(t, kt.KeyX.BitLen(), bitpos)

// 		}
// 	}

// 	// for i := 0; i < kt.KeyX.BitLen(); i++ {
// 	// 	expected := byte('0')
// 	// 	if kt.KeyX.Bit(i) == 1 {
// 	// 		expected = byte('1')
// 	// 	}
// 	// }
// }

// func TestHexString(t *testing.T) {
// 	t.Run("bitlen32", func(t *testing.T) {
// 		kk := NewByteKey([]byte{0b01100110, 0b11111111, 0b00000000, 0b10101010})
// 		require.Equal(t, "66ff00aa", HexString(kk))
// 	})
// 	t.Run("bitlen8", func(t *testing.T) {
// 		kk := NewByteKey([]byte{0b01100110})
// 		require.Equal(t, "66", HexString(kk))
// 	})
// 	t.Run("bitlen8zero", func(t *testing.T) {
// 		kk := NewByteKey([]byte{0})
// 		require.Equal(t, "00", HexString(kk))
// 	})
// 	t.Run("bitlen12", func(t *testing.T) {
// 		kk := newBitKey(12, 0b111111111111)
// 		require.Equal(t, "fff", HexString(kk))
// 	})
// 	t.Run("bitlen20", func(t *testing.T) {
// 		kk := newBitKey(20, 0b00001111111111110000)
// 		require.Equal(t, "0fff0", HexString(kk))
// 	})
// }
