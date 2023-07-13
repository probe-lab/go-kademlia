package triert

import (
	"context"
	"math/rand"
	"testing"

	"github.com/plprobelab/go-kademlia/internal/testutil"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/key/keyutil"
	"github.com/plprobelab/go-kademlia/key/trie"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/stretchr/testify/require"
)

var (
	key0 = key.KadKey(make([]byte, 32)) // 000000...000

	key1  = keyutil.RandomWithPrefix("010000", 32)
	key2  = keyutil.RandomWithPrefix("100000", 32)
	key3  = keyutil.RandomWithPrefix("110000", 32)
	key4  = keyutil.RandomWithPrefix("111000", 32)
	key5  = keyutil.RandomWithPrefix("011000", 32)
	key6  = keyutil.RandomWithPrefix("011100", 32)
	key7  = keyutil.RandomWithPrefix("000110", 32)
	key8  = keyutil.RandomWithPrefix("000101", 32)
	key9  = keyutil.RandomWithPrefix("000100", 32)
	key10 = keyutil.RandomWithPrefix("001000", 32)
	key11 = keyutil.RandomWithPrefix("001100", 32)

	node1  = NewNode("QmPeer1", key1)
	node2  = NewNode("QmPeer2", key2)
	node3  = NewNode("QmPeer3", key3)
	node4  = NewNode("QmPeer4", key4)
	node5  = NewNode("QmPeer5", key5)
	node6  = NewNode("QmPeer6", key6)
	node7  = NewNode("QmPeer7", key7)
	node8  = NewNode("QmPeer8", key8)
	node9  = NewNode("QmPeer9", key9)
	node10 = NewNode("QmPeer10", key10)
	node11 = NewNode("QmPeer11", key11)
)

func TestAddPeer(t *testing.T) {
	t.Run("one", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success, err := rt.AddPeer(context.Background(), node1)
		require.NoError(t, err)
		require.True(t, success)
		require.Equal(t, 1, rt.Size())
	})

	t.Run("ignore duplicate", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success, err := rt.AddPeer(context.Background(), node1)
		require.NoError(t, err)
		require.True(t, success)
		require.Equal(t, 1, rt.Size())

		success, err = rt.AddPeer(context.Background(), node1)
		require.NoError(t, err)
		require.False(t, success)
		require.Equal(t, 1, rt.Size())
	})

	t.Run("many", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success, err := rt.AddPeer(context.Background(), node1)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node2)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node3)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node4)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node5)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node6)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node7)
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(context.Background(), node8)
		require.NoError(t, err)
		require.True(t, success)

		require.Equal(t, 8, rt.Size())
	})
}

func TestRemovePeer(t *testing.T) {
	rt, err := New(key0, nil)
	require.NoError(t, err)
	success, err := rt.AddPeer(context.Background(), node1)
	require.NoError(t, err)
	require.True(t, success)

	t.Run("unknown peer", func(t *testing.T) {
		success, err := rt.RemoveKey(context.Background(), key2)
		require.NoError(t, err)
		require.False(t, success)
	})

	t.Run("known peer", func(t *testing.T) {
		success, err := rt.RemoveKey(context.Background(), key1)
		require.NoError(t, err)
		require.True(t, success)
	})
}

func TestFindPeer(t *testing.T) {
	t.Run("known peer", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success, err := rt.AddPeer(context.Background(), node1)
		require.NoError(t, err)
		require.True(t, success)

		want := node1
		got, err := rt.Find(context.Background(), key1)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("unknown peer", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		got, err := rt.Find(context.Background(), key2)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("removed peer", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success, err := rt.AddPeer(context.Background(), node1)
		require.NoError(t, err)
		require.True(t, success)

		want := node1
		got, err := rt.Find(context.Background(), key1)
		require.NoError(t, err)
		require.Equal(t, want, got)

		success, err = rt.RemoveKey(context.Background(), key1)
		require.NoError(t, err)
		require.True(t, success)

		got, err = rt.Find(context.Background(), key1)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

func TestNearestPeers(t *testing.T) {
	ctx := context.Background()

	rt, err := New(key0, nil)
	require.NoError(t, err)
	rt.AddPeer(ctx, node1)
	rt.AddPeer(ctx, node2)
	rt.AddPeer(ctx, node3)
	rt.AddPeer(ctx, node4)
	rt.AddPeer(ctx, node5)
	rt.AddPeer(ctx, node6)
	rt.AddPeer(ctx, node7)
	rt.AddPeer(ctx, node8)
	rt.AddPeer(ctx, node9)
	rt.AddPeer(ctx, node10)
	rt.AddPeer(ctx, node11)

	// find the 5 nearest peers to key0
	peers, err := rt.NearestPeers(ctx, key0, 5)
	require.NoError(t, err)
	require.Equal(t, 5, len(peers))

	expectedOrder := []address.NodeID{node9, node8, node7, node10, node11}
	require.Equal(t, expectedOrder, peers)

	peers, err = rt.NearestPeers(ctx, node11.Key(), 2)
	require.NoError(t, err)
	require.Equal(t, 2, len(peers))
}

func TestInvalidKeys(t *testing.T) {
	ctx := context.Background()
	incompatKey := key.KadKey(make([]byte, 4)) // key is shorter (4 bytes only)
	incompatNode := NewNode("inv", incompatKey)

	rt, err := New(key0, nil)
	require.NoError(t, err)

	t.Run("add peer", func(t *testing.T) {
		success, err := rt.AddPeer(ctx, incompatNode)
		require.ErrorIs(t, err, trie.ErrMismatchedKeyLength)
		require.False(t, success)
	})

	t.Run("remove key", func(t *testing.T) {
		success, err := rt.RemoveKey(ctx, incompatKey)
		require.ErrorIs(t, err, trie.ErrMismatchedKeyLength)
		require.False(t, success)
	})

	t.Run("find", func(t *testing.T) {
		nodeID, err := rt.Find(ctx, incompatKey)
		require.ErrorIs(t, err, trie.ErrMismatchedKeyLength)
		require.Nil(t, nodeID)
	})

	t.Run("nearest peers", func(t *testing.T) {
		nodeIDs, err := rt.NearestPeers(ctx, incompatKey, 2)
		require.ErrorIs(t, err, trie.ErrMismatchedKeyLength)
		require.Nil(t, nodeIDs)
	})
}

func TestCplSize(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		require.Equal(t, 0, rt.Size())
		require.Equal(t, 0, rt.CplSize(1))
		require.Equal(t, 0, rt.CplSize(2))
		require.Equal(t, 0, rt.CplSize(3))
		require.Equal(t, 0, rt.CplSize(4))
	})

	t.Run("cpl 1", func(t *testing.T) {
		ctx := context.Background()
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success, err := rt.AddPeer(ctx, NewNode("cpl1a", keyutil.RandomWithPrefix("01", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl1b", keyutil.RandomWithPrefix("01", 32)))
		require.NoError(t, err)
		require.True(t, success)
		require.Equal(t, 2, rt.Size())
		require.Equal(t, 2, rt.CplSize(1))

		require.Equal(t, 0, rt.CplSize(2))
		require.Equal(t, 0, rt.CplSize(3))
	})

	t.Run("cpl 2", func(t *testing.T) {
		ctx := context.Background()
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success, err := rt.AddPeer(ctx, NewNode("cpl2a", keyutil.RandomWithPrefix("001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl2b", keyutil.RandomWithPrefix("001", 32)))
		require.NoError(t, err)
		require.True(t, success)

		require.Equal(t, 2, rt.Size())
		require.Equal(t, 2, rt.CplSize(2))

		require.Equal(t, 0, rt.CplSize(1))
		require.Equal(t, 0, rt.CplSize(3))
	})

	t.Run("cpl 3", func(t *testing.T) {
		ctx := context.Background()
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success, err := rt.AddPeer(ctx, NewNode("cpl3a", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3b", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3c", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3d", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)

		require.Equal(t, 4, rt.Size())
		require.Equal(t, 4, rt.CplSize(3))

		require.Equal(t, 0, rt.CplSize(1))
		require.Equal(t, 0, rt.CplSize(2))
	})

	t.Run("cpl mixed", func(t *testing.T) {
		ctx := context.Background()
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success, err := rt.AddPeer(ctx, NewNode("cpl1a", keyutil.RandomWithPrefix("01", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl1b", keyutil.RandomWithPrefix("01", 32)))
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(ctx, NewNode("cpl2a", keyutil.RandomWithPrefix("001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl2b", keyutil.RandomWithPrefix("001", 32)))
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(ctx, NewNode("cpl3a", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3b", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3c", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3d", keyutil.RandomWithPrefix("0001", 32)))
		require.NoError(t, err)
		require.True(t, success)

		require.Equal(t, 8, rt.Size())
		require.Equal(t, 2, rt.CplSize(1))
		require.Equal(t, 2, rt.CplSize(2))
		require.Equal(t, 4, rt.CplSize(3))
	})
}

func TestKeyFilter(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig()
	cfg.KeyFilter = func(rt *TrieRT, kk key.KadKey) bool {
		return !kk.Equal(key2) // don't allow key2 to be added
	}
	rt, err := New(key0, cfg)
	require.NoError(t, err)

	// can't add key2
	success, err := rt.AddPeer(ctx, node2)
	require.NoError(t, err)
	require.False(t, success)

	got, err := rt.Find(ctx, key2)
	require.NoError(t, err)
	require.Nil(t, got)

	// can add other key
	success, err = rt.AddPeer(ctx, node1)
	require.NoError(t, err)
	require.True(t, success)

	want := node1
	got, err = rt.Find(ctx, key1)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func BenchmarkBuildTable(b *testing.B) {
	b.Run("1000", benchmarkBuildTable(1000))
	b.Run("10000", benchmarkBuildTable(10000))
	b.Run("100000", benchmarkBuildTable(100000))
}

func BenchmarkFindPositive(b *testing.B) {
	b.Run("1000", benchmarkFindPositive(1000))
	b.Run("10000", benchmarkFindPositive(10000))
	b.Run("100000", benchmarkFindPositive(100000))
}

func BenchmarkFindNegative(b *testing.B) {
	b.Run("1000", benchmarkFindNegative(1000))
	b.Run("10000", benchmarkFindNegative(10000))
	b.Run("100000", benchmarkFindNegative(100000))
}

func BenchmarkNearestPeers(b *testing.B) {
	b.Run("1000", benchmarkNearestPeers(1000))
	b.Run("10000", benchmarkNearestPeers(10000))
	b.Run("100000", benchmarkNearestPeers(100000))
}

func BenchmarkChurn(b *testing.B) {
	b.Run("1000", benchmarkChurn(1000))
	b.Run("10000", benchmarkChurn(10000))
	b.Run("100000", benchmarkChurn(100000))
}

func benchmarkBuildTable(n int) func(b *testing.B) {
	return func(b *testing.B) {
		nodes := make([]address.NodeID, n)
		for i := 0; i < n; i++ {
			nodes[i] = kadid.NewKadID(keyutil.Random(32))
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt, err := New(key0, nil)
			if err != nil {
				b.Fatalf("unexpected error creating table: %v", err)
			}
			for _, node := range nodes {
				rt.AddPeer(context.Background(), node)
			}
		}
		testutil.ReportTimePerItemMetric(b, len(nodes), "node")
	}
}

func benchmarkFindPositive(n int) func(b *testing.B) {
	return func(b *testing.B) {
		keys := make([]key.KadKey, n)
		for i := 0; i < n; i++ {
			keys[i] = keyutil.Random(32)
		}
		rt, err := New(key0, nil)
		if err != nil {
			b.Fatalf("unexpected error creating table: %v", err)
		}
		for _, kk := range keys {
			rt.AddPeer(context.Background(), kadid.NewKadID(kk))
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt.Find(context.Background(), keys[i%len(keys)])
		}
	}
}

func benchmarkFindNegative(n int) func(b *testing.B) {
	return func(b *testing.B) {
		keys := make([]key.KadKey, n)
		for i := 0; i < n; i++ {
			keys[i] = keyutil.Random(32)
		}
		rt, err := New(key0, nil)
		if err != nil {
			b.Fatalf("unexpected error creating table: %v", err)
		}
		for _, kk := range keys {
			rt.AddPeer(context.Background(), kadid.NewKadID(kk))
		}

		unknown := make([]key.KadKey, n)
		for i := 0; i < n; i++ {
			kk := keyutil.Random(32)
			if found, _ := rt.Find(context.Background(), kk); found != nil {
				continue
			}
			unknown[i] = kk
		}

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt.Find(context.Background(), unknown[i%len(unknown)])
		}
	}
}

func benchmarkNearestPeers(n int) func(b *testing.B) {
	return func(b *testing.B) {
		keys := make([]key.KadKey, n)
		for i := 0; i < n; i++ {
			keys[i] = keyutil.Random(32)
		}
		rt, err := New(key0, nil)
		if err != nil {
			b.Fatalf("unexpected error creating table: %v", err)
		}
		for _, kk := range keys {
			rt.AddPeer(context.Background(), kadid.NewKadID(kk))
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt.NearestPeers(context.Background(), keyutil.Random(32), 20)
		}
	}
}

func benchmarkChurn(n int) func(b *testing.B) {
	return func(b *testing.B) {
		universe := make([]address.NodeID, n)
		for i := 0; i < n; i++ {
			universe[i] = kadid.NewKadID(keyutil.Random(32))
		}
		rt, err := New(key0, nil)
		if err != nil {
			b.Fatalf("unexpected error creating table: %v", err)
		}
		// Add a portion of the universe to the routing table
		for i := 0; i < len(universe)/4; i++ {
			rt.AddPeer(context.Background(), universe[i])
		}
		rand.Shuffle(len(universe), func(i, j int) { universe[i], universe[j] = universe[j], universe[i] })

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			node := universe[i%len(universe)]
			found, _ := rt.Find(context.Background(), node.Key())
			if found == nil {
				// add new peer
				rt.AddPeer(context.Background(), universe[i%len(universe)])
			} else {
				// remove it
				rt.RemoveKey(context.Background(), node.Key())
			}
		}
	}
}

var _ address.NodeID = (*node)(nil)

type node struct {
	id  string
	key key.KadKey
}

func NewNode(id string, k key.KadKey) *node {
	return &node{
		id:  id,
		key: k,
	}
}

func (n node) String() string {
	return n.id
}

func (n node) Key() key.KadKey {
	return n.key
}

func (n node) NodeID() address.NodeID {
	return &n
}
