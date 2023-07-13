package triert

import (
	"context"
	"fmt"
	"testing"

	"github.com/plprobelab/go-kademlia/internal/testutil"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/stretchr/testify/require"
)

func TestBucketLimit20(t *testing.T) {
	ctx := context.Background()

	cfg := DefaultConfig()
	cfg.KeyFilter = BucketLimit20
	rt, err := New(key0, cfg)
	require.NoError(t, err)

	nodes := make([]address.NodeID, 21)
	for i := range nodes {
		kk := testutil.RandomWithPrefix("000100", 256)
		nodes[i] = NewNode(fmt.Sprintf("QmPeer%d", i), kk)
	}

	// Add 20 peers with cpl 3
	for i := 0; i < 20; i++ {
		success, err := rt.AddPeer(ctx, nodes[i])
		require.NoError(t, err)
		require.True(t, success)
	}

	// cannot add 21st
	success, err := rt.AddPeer(ctx, nodes[20])
	require.NoError(t, err)
	require.False(t, success)

	// add peer with different cpl
	kk := testutil.RandomWithPrefix("0000100", 256)
	node22 := NewNode("QmPeer22", kk)
	success, err = rt.AddPeer(ctx, node22)
	require.NoError(t, err)
	require.True(t, success)

	// make space for another cpl 3 key
	success, err = rt.RemoveKey(ctx, nodes[0].Key())
	require.NoError(t, err)
	require.True(t, success)

	// now can add cpl 3 key
	success, err = rt.AddPeer(ctx, nodes[20])
	require.NoError(t, err)
	require.True(t, success)
}
