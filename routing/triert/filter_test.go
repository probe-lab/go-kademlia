package triert

import (
	"fmt"
	"testing"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/stretchr/testify/require"
)

func TestBucketLimit20(t *testing.T) {
	cfg := DefaultConfig[key.Key32]()
	cfg.KeyFilter = BucketLimit20[key.Key32]
	rt, err := New(key0, cfg)
	require.NoError(t, err)

	nodes := make([]kad.NodeID[key.Key32], 21)
	for i := range nodes {
		kk := kadtest.RandomKeyWithPrefix("000100")
		nodes[i] = NewNode(fmt.Sprintf("QmPeer%d", i), kk)
	}

	// Add 20 peers with cpl 3
	for i := 0; i < 20; i++ {
		success := rt.AddNode(nodes[i])
		require.True(t, success)
	}

	// cannot add 21st
	success := rt.AddNode(nodes[20])
	require.False(t, success)

	// add peer with different cpl
	kk := kadtest.RandomKeyWithPrefix("0000100")
	node22 := NewNode("QmPeer22", kk)
	success = rt.AddNode(node22)
	require.True(t, success)

	// make space for another cpl 3 key
	success = rt.RemoveKey(nodes[0].Key())
	require.True(t, success)

	// now can add cpl 3 key
	success = rt.AddNode(nodes[20])
	require.True(t, success)
}
