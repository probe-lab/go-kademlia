package simmessage

import (
	"testing"

	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	si "github.com/libp2p/go-libp2p-kad-dht/network/address/stringid"
	"github.com/stretchr/testify/require"
)

func TestSimRequest(t *testing.T) {
	target := si.StringID("target")
	msg := NewSimRequest(target.Key())

	reqTarget := msg.Target()
	require.NotNil(t, reqTarget)

	b, _ := msg.Target().Equal(target.Key())
	require.True(t, b)
	require.Nil(t, msg.CloserNodes())
}

func TestSimResponse(t *testing.T) {
	closerPeers := []address.NodeID{si.StringID("peer1"), si.StringID("peer2")}
	msg := NewSimResponse(closerPeers)

	require.Nil(t, msg.Target())
	require.Equal(t, len(closerPeers), len(msg.CloserNodes()))
	for i, peer := range closerPeers {
		require.Equal(t, closerPeers[i], peer)
	}
}
