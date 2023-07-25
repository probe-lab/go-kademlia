package triert

import (
	"context"
	"math/rand"
	"testing"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/stretchr/testify/require"
)

var (
	key0 = key.Key32(0) // 000000...000

	key1  = kadtest.RandomKeyWithPrefix("010000")
	key2  = kadtest.RandomKeyWithPrefix("100000")
	key3  = kadtest.RandomKeyWithPrefix("110000")
	key4  = kadtest.RandomKeyWithPrefix("111000")
	key5  = kadtest.RandomKeyWithPrefix("011000")
	key6  = kadtest.RandomKeyWithPrefix("011100")
	key7  = kadtest.RandomKeyWithPrefix("000110")
	key8  = kadtest.RandomKeyWithPrefix("000101")
	key9  = kadtest.RandomKeyWithPrefix("000100")
	key10 = kadtest.RandomKeyWithPrefix("001000")
	key11 = kadtest.RandomKeyWithPrefix("001100")

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

	expectedOrder := []address.NodeID[key.Key32]{node9, node8, node7, node10, node11}
	require.Equal(t, expectedOrder, peers)

	peers, err = rt.NearestPeers(ctx, node11.Key(), 2)
	require.NoError(t, err)
	require.Equal(t, 2, len(peers))
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

		success, err := rt.AddPeer(ctx, NewNode("cpl1a", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl1b", kadtest.RandomKeyWithPrefix("01")))
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

		success, err := rt.AddPeer(ctx, NewNode("cpl2a", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl2b", kadtest.RandomKeyWithPrefix("001")))
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

		success, err := rt.AddPeer(ctx, NewNode("cpl3a", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3b", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3c", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3d", kadtest.RandomKeyWithPrefix("0001")))
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

		success, err := rt.AddPeer(ctx, NewNode("cpl1a", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl1b", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(ctx, NewNode("cpl2a", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl2b", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)

		success, err = rt.AddPeer(ctx, NewNode("cpl3a", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3b", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3c", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success, err = rt.AddPeer(ctx, NewNode("cpl3d", kadtest.RandomKeyWithPrefix("0001")))
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
	cfg := DefaultConfig[key.Key32]()
	cfg.KeyFilter = func(rt *TrieRT[key.Key32], kk key.Key32) bool {
		return !key.Equal(kk, key2) // don't allow key2 to be added
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
		nodes := make([]address.NodeID[key.Key32], n)
		for i := 0; i < n; i++ {
			nodes[i] = kadid.NewKadID(kadtest.RandomKey())
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
		kadtest.ReportTimePerItemMetric(b, len(nodes), "node")
	}
}

func benchmarkFindPositive(n int) func(b *testing.B) {
	return func(b *testing.B) {
		keys := make([]key.Key32, n)
		for i := 0; i < n; i++ {
			keys[i] = kadtest.RandomKey()
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
		keys := make([]key.Key32, n)
		for i := 0; i < n; i++ {
			keys[i] = kadtest.RandomKey()
		}
		rt, err := New(key0, nil)
		if err != nil {
			b.Fatalf("unexpected error creating table: %v", err)
		}
		for _, kk := range keys {
			rt.AddPeer(context.Background(), kadid.NewKadID(kk))
		}

		unknown := make([]key.Key32, n)
		for i := 0; i < n; i++ {
			kk := kadtest.RandomKey()
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
		keys := make([]key.Key32, n)
		for i := 0; i < n; i++ {
			keys[i] = kadtest.RandomKey()
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
			rt.NearestPeers(context.Background(), kadtest.RandomKey(), 20)
		}
	}
}

func benchmarkChurn(n int) func(b *testing.B) {
	return func(b *testing.B) {
		universe := make([]address.NodeID[key.Key32], n)
		for i := 0; i < n; i++ {
			universe[i] = kadid.NewKadID(kadtest.RandomKey())
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

// var _ address.NodeID = (*node)(nil)

type node[K kad.Key[K]] struct {
	id  string
	key K
}

func NewNode[K kad.Key[K]](id string, k K) *node[K] {
	return &node[K]{
		id:  id,
		key: k,
	}
}

func (n node[K]) String() string {
	return n.id
}

func (n node[K]) Key() K {
	return n.key
}

func (n node[K]) NodeID() address.NodeID[K] {
	return &n
}
