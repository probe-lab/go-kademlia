package triert

import (
	"context"
	"math/rand"
	"testing"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
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
		success := rt.AddNode(node1)
		require.True(t, success)
		require.Equal(t, 1, rt.Size())
	})

	t.Run("ignore duplicate", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success := rt.AddNode(node1)
		require.True(t, success)
		require.Equal(t, 1, rt.Size())

		success = rt.AddNode(node1)
		require.False(t, success)
		require.Equal(t, 1, rt.Size())
	})

	t.Run("many", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success := rt.AddNode(node1)
		require.True(t, success)

		success = rt.AddNode(node2)
		require.True(t, success)

		success = rt.AddNode(node3)
		require.True(t, success)

		success = rt.AddNode(node4)
		require.True(t, success)

		success = rt.AddNode(node5)
		require.True(t, success)

		success = rt.AddNode(node6)
		require.True(t, success)

		success = rt.AddNode(node7)
		require.True(t, success)

		success = rt.AddNode(node8)
		require.True(t, success)

		require.Equal(t, 8, rt.Size())
	})
}

func TestRemovePeer(t *testing.T) {
	rt, err := New(key0, nil)
	require.NoError(t, err)
	success := rt.AddNode(node1)
	require.NoError(t, err)
	require.True(t, success)

	t.Run("unknown peer", func(t *testing.T) {
		success = rt.RemoveKey(key2)
		require.False(t, success)
	})

	t.Run("known peer", func(t *testing.T) {
		success = rt.RemoveKey(key1)
		require.True(t, success)
	})
}

func TestFindPeer(t *testing.T) {
	t.Run("known peer", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)
		success := rt.AddNode(node1)
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
		success := rt.AddNode(node1)
		require.True(t, success)

		want := node1
		got, err := rt.Find(context.Background(), key1)
		require.NoError(t, err)
		require.Equal(t, want, got)

		success = rt.RemoveKey(key1)
		require.True(t, success)

		got, err = rt.Find(context.Background(), key1)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}

func TestNearestPeers(t *testing.T) {
	rt, err := New(key0, nil)
	require.NoError(t, err)
	rt.AddNode(node1)
	rt.AddNode(node2)
	rt.AddNode(node3)
	rt.AddNode(node4)
	rt.AddNode(node5)
	rt.AddNode(node6)
	rt.AddNode(node7)
	rt.AddNode(node8)
	rt.AddNode(node9)
	rt.AddNode(node10)
	rt.AddNode(node11)

	// find the 5 nearest peers to key0
	peers := rt.NearestNodes(key0, 5)
	require.Equal(t, 5, len(peers))

	expectedOrder := []kad.NodeID[key.Key32]{node9, node8, node7, node10, node11}
	require.Equal(t, expectedOrder, peers)

	peers = rt.NearestNodes(node11.Key(), 2)
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
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success := rt.AddNode(NewNode("cpl1a", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl1b", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)
		require.Equal(t, 2, rt.Size())
		require.Equal(t, 2, rt.CplSize(1))

		require.Equal(t, 0, rt.CplSize(2))
		require.Equal(t, 0, rt.CplSize(3))
	})

	t.Run("cpl 2", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success := rt.AddNode(NewNode("cpl2a", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl2b", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)

		require.Equal(t, 2, rt.Size())
		require.Equal(t, 2, rt.CplSize(2))

		require.Equal(t, 0, rt.CplSize(1))
		require.Equal(t, 0, rt.CplSize(3))
	})

	t.Run("cpl 3", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success := rt.AddNode(NewNode("cpl3a", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl3b", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl3c", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl3d", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)

		require.Equal(t, 4, rt.Size())
		require.Equal(t, 4, rt.CplSize(3))

		require.Equal(t, 0, rt.CplSize(1))
		require.Equal(t, 0, rt.CplSize(2))
	})

	t.Run("cpl mixed", func(t *testing.T) {
		rt, err := New(key0, nil)
		require.NoError(t, err)

		success := rt.AddNode(NewNode("cpl1a", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl1b", kadtest.RandomKeyWithPrefix("01")))
		require.NoError(t, err)
		require.True(t, success)

		success = rt.AddNode(NewNode("cpl2a", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl2b", kadtest.RandomKeyWithPrefix("001")))
		require.NoError(t, err)
		require.True(t, success)

		success = rt.AddNode(NewNode("cpl3a", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl3b", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl3c", kadtest.RandomKeyWithPrefix("0001")))
		require.NoError(t, err)
		require.True(t, success)
		success = rt.AddNode(NewNode("cpl3d", kadtest.RandomKeyWithPrefix("0001")))
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
	success := rt.AddNode(node2)
	require.NoError(t, err)
	require.False(t, success)

	got, err := rt.Find(ctx, key2)
	require.NoError(t, err)
	require.Nil(t, got)

	// can add other key
	success = rt.AddNode(node1)
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
		nodes := make([]kad.NodeID[key.Key32], n)
		for i := 0; i < n; i++ {
			nodes[i] = kadtest.NewID(kadtest.RandomKey())
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt, err := New(key0, nil)
			if err != nil {
				b.Fatalf("unexpected error creating table: %v", err)
			}
			for _, node := range nodes {
				rt.AddNode(node)
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
			rt.AddNode(kadtest.NewID(kk))
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
			rt.AddNode(kadtest.NewID(kk))
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
			rt.AddNode(kadtest.NewID(kk))
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rt.NearestNodes(kadtest.RandomKey(), 20)
		}
	}
}

func benchmarkChurn(n int) func(b *testing.B) {
	return func(b *testing.B) {
		universe := make([]kad.NodeID[key.Key32], n)
		for i := 0; i < n; i++ {
			universe[i] = kadtest.NewID(kadtest.RandomKey())
		}
		rt, err := New(key0, nil)
		if err != nil {
			b.Fatalf("unexpected error creating table: %v", err)
		}
		// Add a portion of the universe to the routing table
		for i := 0; i < len(universe)/4; i++ {
			rt.AddNode(universe[i])
		}
		rand.Shuffle(len(universe), func(i, j int) { universe[i], universe[j] = universe[j], universe[i] })

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			node := universe[i%len(universe)]
			found, _ := rt.Find(context.Background(), node.Key())
			if found == nil {
				// add new peer
				rt.AddNode(universe[i%len(universe)])
			} else {
				// remove it
				rt.RemoveKey(node.Key())
			}
		}
	}
}

// var _ kad.NodeID = (*node)(nil)

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

func (n node[K]) NodeID() kad.NodeID[K] {
	return &n
}
