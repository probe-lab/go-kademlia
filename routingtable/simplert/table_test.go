package simplert

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/peerid"
	si "github.com/libp2p/go-libp2p-kad-dht/network/address/stringid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func zeroBytes(n int) []byte {
	bytes := make([]byte, n)
	for i := 0; i < n; i++ {
		bytes[i] = 0
	}
	return bytes
}

var (
	key0  = key.KadKey(zeroBytes(32))                          // 000000...000
	key1  = key.KadKey(append([]byte{0x40}, zeroBytes(31)...)) // 010000...000
	key2  = key.KadKey(append([]byte{0x80}, zeroBytes(31)...)) // 100000...000
	key3  = key.KadKey(append([]byte{0xc0}, zeroBytes(31)...)) // 110000...000
	key4  = key.KadKey(append([]byte{0xe0}, zeroBytes(31)...)) // 111000...000
	key5  = key.KadKey(append([]byte{0x60}, zeroBytes(31)...)) // 011000...000
	key6  = key.KadKey(append([]byte{0x70}, zeroBytes(31)...)) // 011100...000
	key7  = key.KadKey(append([]byte{0x18}, zeroBytes(31)...)) // 000110...000
	key8  = key.KadKey(append([]byte{0x14}, zeroBytes(31)...)) // 000101...000
	key9  = key.KadKey(append([]byte{0x10}, zeroBytes(31)...)) // 000100...000
	key10 = key.KadKey(append([]byte{0x20}, zeroBytes(31)...)) // 001000...000
	key11 = key.KadKey(append([]byte{0x30}, zeroBytes(31)...)) // 001100...100
)

func TestBasic(t *testing.T) {
	bucketSize := 100
	rt := NewSimpleRT(key0, bucketSize)
	require.Equal(t, bucketSize, rt.BucketSize())

	require.Equal(t, key0, rt.Self())
}

func TestAddPeer(t *testing.T) {
	ctx := context.Background()

	p := peerid.PeerID{ID: peer.ID("")}

	rt := NewSimpleRT(key0, 2)

	require.Equal(t, 0, rt.SizeOfBucket(0))

	// add peer CPL=1, bucket=0
	success, err := rt.addPeer(ctx, key1, p)
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, 1, rt.SizeOfBucket(0))

	// cannot add the same peer twice
	success, err = rt.addPeer(ctx, key1, p)
	require.NoError(t, err)
	require.False(t, success)
	require.Equal(t, 1, rt.SizeOfBucket(0))

	// add peer CPL=0, bucket=0
	success, err = rt.addPeer(ctx, key2, p)
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, 2, rt.SizeOfBucket(0))

	// add peer CPL=0, bucket=0. split of bucket0
	// key1 goes to bucket1
	success, err = rt.addPeer(ctx, key3, p)
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, 2, rt.SizeOfBucket(0))
	require.Equal(t, 1, rt.SizeOfBucket(1))

	// already 2 peers with CPL = 0, so this should fail
	success, err = rt.addPeer(ctx, key4, p)
	require.NoError(t, err)
	require.False(t, success)
	// add peer CPL=1, bucket=1
	success, err = rt.addPeer(ctx, key5, p)
	require.NoError(t, err)
	require.True(t, success)
	require.Equal(t, 2, rt.SizeOfBucket(1))

	// already 2 peers with CPL = 1, so this should fail
	// even if bucket 1 is the last bucket
	success, err = rt.addPeer(ctx, key6, p)
	require.NoError(t, err)
	require.False(t, success)

	// add two peers with CPL = 3, bucket=2
	success, err = rt.addPeer(ctx, key7, p)
	require.NoError(t, err)
	require.True(t, success)
	success, err = rt.addPeer(ctx, key8, p)
	require.NoError(t, err)
	require.True(t, success)
	// cannot add a third peer with CPL = 3
	success, err = rt.addPeer(ctx, key9, p)
	require.NoError(t, err)
	require.False(t, success)

	// add two peers with CPL = 2, bucket=2
	success, err = rt.addPeer(ctx, key10, p)
	require.NoError(t, err)
	require.True(t, success)
	success, err = rt.addPeer(ctx, key11, p)
	require.NoError(t, err)
	require.True(t, success)

	// remove all peers with CPL = 0
	success, err = rt.RemoveKey(ctx, key2)
	require.NoError(t, err)
	require.True(t, success)
	success, err = rt.RemoveKey(ctx, key3)
	require.NoError(t, err)
	require.True(t, success)

	// a new peer with CPL = 0 can be added
	// note: p belongs to bucket 0
	success, err = rt.AddPeer(ctx, p)
	require.NoError(t, err)
	require.True(t, success)
	// cannot add the same peer twice even tough
	// the bucket is not full
	success, err = rt.AddPeer(ctx, p)
	require.NoError(t, err)
	require.False(t, success)
}

func TestRemovePeer(t *testing.T) {
	ctx := context.Background()
	p := peerid.PeerID{ID: peer.ID("")}

	rt := NewSimpleRT(key0, 2)
	rt.addPeer(ctx, key1, p)
	success, err := rt.RemoveKey(ctx, key2)
	require.NoError(t, err)
	require.False(t, success)
	success, err = rt.RemoveKey(ctx, key1)
	require.NoError(t, err)
	require.True(t, success)
}

func TestFindPeer(t *testing.T) {
	ctx := context.Background()
	p := peerid.PeerID{ID: peer.ID("QmPeer")}

	rt := NewSimpleRT(key0, 2)
	success, err := rt.addPeer(ctx, key1, p)
	require.NoError(t, err)
	require.True(t, success)

	peerid, err := rt.Find(ctx, key1)
	require.NoError(t, err)
	require.Equal(t, p, peerid)

	peerid, err = rt.Find(ctx, key2)
	require.NoError(t, err)
	require.Nil(t, peerid)

	success, err = rt.RemoveKey(ctx, key1)
	require.NoError(t, err)
	require.True(t, success)

	peerid, err = rt.Find(ctx, key1)
	require.NoError(t, err)
	require.Nil(t, peerid)
}

func TestNearestPeers(t *testing.T) {
	ctx := context.Background()

	peerIds := make([]peerid.PeerID, 0, 12)
	for i := 0; i < 12; i++ {
		peerIds = append(peerIds, peerid.PeerID{ID: peer.ID(fmt.Sprintf("QmPeer%d", i))})
	}

	bucketSize := 5

	rt := NewSimpleRT(key0, bucketSize)
	rt.addPeer(ctx, key1, peerIds[1])
	rt.addPeer(ctx, key2, peerIds[2])
	rt.addPeer(ctx, key3, peerIds[3])
	rt.addPeer(ctx, key4, peerIds[4])
	rt.addPeer(ctx, key5, peerIds[5])
	rt.addPeer(ctx, key6, peerIds[6])
	rt.addPeer(ctx, key7, peerIds[7])
	rt.addPeer(ctx, key8, peerIds[8])
	rt.addPeer(ctx, key9, peerIds[9])
	rt.addPeer(ctx, key10, peerIds[10])
	rt.addPeer(ctx, key11, peerIds[11])

	// find the 5 nearest peers to key0
	peers, err := rt.NearestPeers(ctx, key0, bucketSize)
	require.NoError(t, err)
	require.Equal(t, bucketSize, len(peers))

	expectedOrder := []address.NodeID{peerIds[9], peerIds[8], peerIds[7], peerIds[10], peerIds[11]}
	require.Equal(t, expectedOrder, peers)

	peers, err = rt.NearestPeers(ctx, key11, 2)
	require.NoError(t, err)
	require.Equal(t, 2, len(peers))

	// create routing table with a single duplicate peer
	// useful to test peers sorting with duplicate (even tough it should never happen)
	rt2 := NewSimpleRT(key0, 2)
	rt2.buckets[0] = append(rt2.buckets[0], peerInfo{peerIds[1], key1})
	rt2.buckets[0] = append(rt2.buckets[0], peerInfo{peerIds[1], key1})
	peers, err = rt2.NearestPeers(ctx, key0, 10)
	require.NoError(t, err)
	require.Equal(t, peers[0], peers[1])
}

func TestInvalidKeys(t *testing.T) {
	ctx := context.Background()
	dummyNodeId := si.StringID("dummy")
	invalidKey := key.KadKey(zeroBytes(4)) // key is shorter (4 bytes only)

	rt := NewSimpleRT(key0, 2)
	success, err := rt.addPeer(ctx, invalidKey, dummyNodeId)
	require.Equal(t, err, key.ErrInvalidKey(32))
	require.False(t, success)

	bid, err := rt.BucketIdForKey(invalidKey)
	require.Equal(t, err, key.ErrInvalidKey(32))
	require.Equal(t, bid, 0)

	success, err = rt.RemoveKey(ctx, invalidKey)
	require.Equal(t, err, key.ErrInvalidKey(32))
	require.False(t, success)

	nodeID, err := rt.Find(ctx, invalidKey)
	require.Equal(t, err, key.ErrInvalidKey(32))
	require.Nil(t, nodeID)

	nodeIDs, err := rt.NearestPeers(ctx, invalidKey, 2)
	require.Equal(t, err, key.ErrInvalidKey(32))
	require.Nil(t, nodeIDs)
}
