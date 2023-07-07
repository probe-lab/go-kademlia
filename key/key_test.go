package key

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var keysize = 4

func TestKadKeyString(t *testing.T) {
	zeroKadid := KadKey(make([]byte, keysize))
	zeroHex := strings.Repeat("00", keysize)
	require.Equal(t, zeroHex, zeroKadid.String())

	ffKadid := make([]byte, keysize)
	for i := 0; i < keysize; i++ {
		ffKadid[i] = 0xff
	}
	ffHex := strings.Repeat("ff", keysize)
	require.Equal(t, ffHex, KadKey(ffKadid).String())

	e3Kadid := make([]byte, keysize)
	for i := 0; i < keysize; i++ {
		e3Kadid[i] = 0xe3
	}
	e3Hex := strings.Repeat("e3", keysize)
	require.Equal(t, e3Hex, KadKey(e3Kadid).String())
}

func TestXor(t *testing.T) {
	key0 := KadKey(make([]byte, keysize))             // 00000...000
	randKey := KadKey([]byte{0x23, 0xe4, 0xdd, 0x03}) // arbitrary key

	xored := key0.Xor(key0)
	require.Equal(t, key0, xored)
	xored = randKey.Xor(key0)
	require.Equal(t, randKey, xored)
	xored = key0.Xor(randKey)
	require.Equal(t, randKey, xored)
	xored = randKey.Xor(randKey)
	require.Equal(t, key0, xored)

	shorterKey := KadKey([]byte{0x23, 0xe4, 0xdd}) // shorter key
	xored = key0.Xor(shorterKey)
	expected := append(shorterKey, make([]byte, key0.Size()-shorterKey.Size())...)
	require.Equal(t, expected, xored)
	xored = shorterKey.Xor(key0)
	require.Equal(t, expected, xored)
	xored = key0.Xor(nil)
	require.Equal(t, key0, xored)
}

func TestCommonPrefixLength(t *testing.T) {
	key0 := KadKey(make([]byte, keysize))                            // 00000...000
	key1 := KadKey(append(make([]byte, keysize-1), 0x01))            // 00000...001
	key2 := KadKey(append([]byte{0x80}, make([]byte, keysize-1)...)) // 10000...000
	key3 := KadKey(append([]byte{0x40}, make([]byte, keysize-1)...)) // 01000...000

	cpl := key0.CommonPrefixLength(key0)
	require.Equal(t, keysize*8, cpl)
	cpl = key0.CommonPrefixLength(key1)
	require.Equal(t, keysize*8-1, cpl)
	cpl = key0.CommonPrefixLength(key2)
	require.Equal(t, 0, cpl)
	cpl = key0.CommonPrefixLength(key3)
	require.Equal(t, 1, cpl)

	cpl = key0.CommonPrefixLength(nil)
	require.Equal(t, 0, cpl)
	cpl = key0.CommonPrefixLength([]byte{0x00})
	require.Equal(t, 8, cpl)
	cpl = key0.CommonPrefixLength([]byte{0x00, 0x40})
	require.Equal(t, 9, cpl)
	cpl = key0.CommonPrefixLength([]byte{0x80})
	require.Equal(t, 0, cpl)
}

func TestCompare(t *testing.T) {
	nKeys := 5
	keys := make([]KadKey, nKeys)
	// ascending order
	keys[0] = KadKey(make([]byte, keysize))                            // 00000...000
	keys[1] = KadKey(append(make([]byte, keysize-1), 0x01))            // 00000...001
	keys[2] = KadKey(append(make([]byte, keysize-1), 0x02))            // 00000...010
	keys[3] = KadKey(append([]byte{0x40}, make([]byte, keysize-1)...)) // 01000...000
	keys[4] = KadKey(append([]byte{0x80}, make([]byte, keysize-1)...)) // 10000...000

	for i := 0; i < nKeys; i++ {
		for j := 0; j < nKeys; j++ {
			res := keys[i].Compare(keys[j])
			if i < j {
				require.Equal(t, -1, res)
			} else if i > j {
				require.Equal(t, 1, res)
			} else {
				require.Equal(t, 0, res)
				equal := keys[i].Equal(keys[j])
				require.True(t, equal)
			}
		}
	}

	// compare keys of different sizes
	key := keys[4]                                                          // 10000...000 (32 bits)
	require.Equal(t, 1, key.Compare([]byte{}))                              // b is prefix of a -> 1
	require.Equal(t, 1, key.Compare([]byte{0x00}))                          // a[0] > b [0] -> 1
	require.Equal(t, 1, key.Compare([]byte{0x80}))                          // b is prefix of a -> 1
	require.Equal(t, -1, key.Compare([]byte{0x81}))                         // a[4] < b[4] -> -1
	require.Equal(t, 0, key.Compare([]byte{0x80, 0x00, 0x00, 0x00}))        // a == b -> 0
	require.Equal(t, -1, key.Compare([]byte{0x80, 0x00, 0x00, 0x00, 0x00})) // a is prefix of b -> -1
}
