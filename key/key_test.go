package key

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const keysize = 4

type TestKadKey = KadKey[[keysize]byte]

func NewTestKadKey(data []byte) TestKadKey {
	return New[[keysize]byte](data)
}

func zeroBytes(n int) []byte {
	bytes := make([]byte, n)
	for i := 0; i < n; i++ {
		bytes[i] = 0
	}
	return bytes
}

func TestKadKeyString(t *testing.T) {
	zeroKadid := NewTestKadKey(zeroBytes(keysize))
	zeroHex := strings.Repeat("00", keysize)
	require.Equal(t, zeroHex, zeroKadid.String())

	ffKadid := make([]byte, keysize)
	for i := 0; i < keysize; i++ {
		ffKadid[i] = 0xff
	}
	ffHex := strings.Repeat("ff", keysize)
	require.Equal(t, ffHex, NewTestKadKey(ffKadid).String())

	e3Kadid := make([]byte, keysize)
	for i := 0; i < keysize; i++ {
		e3Kadid[i] = 0xe3
	}
	e3Hex := strings.Repeat("e3", keysize)
	require.Equal(t, e3Hex, NewTestKadKey(e3Kadid).String())
}

func TestXor(t *testing.T) {
	key0 := NewTestKadKey(zeroBytes(keysize))                // 00000...000
	randKey := NewTestKadKey([]byte{0x23, 0xe4, 0xdd, 0x03}) // arbitrary key

	xored := key0.Xor(key0)
	require.Equal(t, key0, xored)
	xored = randKey.Xor(key0)
	require.Equal(t, randKey, xored)
	xored = key0.Xor(randKey)
	require.Equal(t, randKey, xored)
	xored = randKey.Xor(randKey)
	require.Equal(t, key0, xored)
}

func TestCommonPrefixLength(t *testing.T) {
	key0 := NewTestKadKey(zeroBytes(keysize))                            // 00000...000
	key1 := NewTestKadKey(append(zeroBytes(keysize-1), 0x01))            // 00000...001
	key2 := NewTestKadKey(append([]byte{0x80}, zeroBytes(keysize-1)...)) // 10000...000
	key3 := NewTestKadKey(append([]byte{0x40}, zeroBytes(keysize-1)...)) // 01000...000

	cpl := key0.CommonPrefixLength(key0)
	require.Equal(t, keysize*8, cpl)
	cpl = key0.CommonPrefixLength(key1)
	require.Equal(t, keysize*8-1, cpl)
	cpl = key0.CommonPrefixLength(key2)
	require.Equal(t, 0, cpl)
	cpl = key0.CommonPrefixLength(key3)
	require.Equal(t, 1, cpl)
}

func TestCompare(t *testing.T) {
	nKeys := 5
	keys := make([]TestKadKey, nKeys)
	// ascending order
	keys[0] = NewTestKadKey(zeroBytes(keysize))                            // 00000...000
	keys[1] = NewTestKadKey(append(zeroBytes(keysize-1), 0x01))            // 00000...001
	keys[2] = NewTestKadKey(append(zeroBytes(keysize-1), 0x02))            // 00000...010
	keys[3] = NewTestKadKey(append([]byte{0x40}, zeroBytes(keysize-1)...)) // 01000...000
	keys[4] = NewTestKadKey(append([]byte{0x80}, zeroBytes(keysize-1)...)) // 10000...000

	for i := 0; i < nKeys; i++ {
		for j := 0; j < nKeys; j++ {
			res := keys[i].Compare(keys[j])
			if i < j {
				require.Equal(t, int(-1), res)
			} else if i > j {
				require.Equal(t, int(1), res)
			} else {
				require.Equal(t, int(0), res)
				equal := keys[i].Equal(keys[j])
				require.True(t, equal)
			}
		}
	}
}
