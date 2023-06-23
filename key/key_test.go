package key

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var keysize = 4

func zeroBytes(n int) []byte {
	bytes := make([]byte, n)
	for i := 0; i < n; i++ {
		bytes[i] = 0
	}
	return bytes
}

func TestKadKeyString(t *testing.T) {
	zeroKadid := KadKey(zeroBytes(keysize))
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
	key0 := KadKey(zeroBytes(keysize))                // 00000...000
	randKey := KadKey([]byte{0x23, 0xe4, 0xdd, 0x03}) // arbitrary key

	xored, err := key0.Xor(key0)
	require.NoError(t, err)
	require.Equal(t, key0, xored)
	xored, _ = randKey.Xor(key0)
	require.Equal(t, randKey, xored)
	xored, _ = key0.Xor(randKey)
	require.Equal(t, randKey, xored)
	xored, _ = randKey.Xor(randKey)
	require.Equal(t, key0, xored)

	invalidKey := KadKey([]byte{0x23, 0xe4, 0xdd}) // invalid key
	_, err = key0.Xor(invalidKey)
	require.Equal(t, ErrInvalidKey(4), err)
}

func TestCommonPrefixLength(t *testing.T) {
	key0 := KadKey(zeroBytes(keysize))                            // 00000...000
	key1 := KadKey(append(zeroBytes(keysize-1), 0x01))            // 00000...001
	key2 := KadKey(append([]byte{0x80}, zeroBytes(keysize-1)...)) // 10000...000
	key3 := KadKey(append([]byte{0x40}, zeroBytes(keysize-1)...)) // 01000...000

	cpl, err := key0.CommonPrefixLength(key0)
	require.NoError(t, err)
	require.Equal(t, keysize*8, cpl)
	cpl, _ = key0.CommonPrefixLength(key1)
	require.Equal(t, keysize*8-1, cpl)
	cpl, _ = key0.CommonPrefixLength(key2)
	require.Equal(t, 0, cpl)
	cpl, _ = key0.CommonPrefixLength(key3)
	require.Equal(t, 1, cpl)

	invalidKey := KadKey([]byte{0x23, 0xe4, 0xdd}) // invalid key
	_, err = key0.CommonPrefixLength(invalidKey)
	require.Equal(t, ErrInvalidKey(4), err)
}

func TestCompare(t *testing.T) {
	nKeys := 5
	keys := make([]KadKey, nKeys)
	// ascending order
	keys[0] = KadKey(zeroBytes(keysize))                            // 00000...000
	keys[1] = KadKey(append(zeroBytes(keysize-1), 0x01))            // 00000...001
	keys[2] = KadKey(append(zeroBytes(keysize-1), 0x02))            // 00000...010
	keys[3] = KadKey(append([]byte{0x40}, zeroBytes(keysize-1)...)) // 01000...000
	keys[4] = KadKey(append([]byte{0x80}, zeroBytes(keysize-1)...)) // 10000...000

	for i := 0; i < nKeys; i++ {
		for j := 0; j < nKeys; j++ {
			res, _ := keys[i].Compare(keys[j])
			if i < j {
				require.Equal(t, int8(-1), res)
			} else if i > j {
				require.Equal(t, int8(1), res)
			} else {
				require.Equal(t, int8(0), res)
				equal, err := keys[i].Equal(keys[j])
				require.NoError(t, err)
				require.True(t, equal)
			}
		}
	}

	invalidKey := KadKey([]byte{0x23, 0xe4, 0xdd}) // invalid key
	_, err := keys[0].Compare(invalidKey)
	require.Equal(t, ErrInvalidKey(4), err)
}
