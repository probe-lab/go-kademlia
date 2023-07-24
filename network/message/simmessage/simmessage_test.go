package simmessage

import (
	"testing"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	si "github.com/plprobelab/go-kademlia/network/address/stringid"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/stretchr/testify/require"
)

var (
	_ message.MinKadRequestMessage[key.Key8]  = (*SimMessage[key.Key8])(nil)
	_ message.MinKadResponseMessage[key.Key8] = (*SimMessage[key.Key8])(nil)
)

func TestSimRequest(t *testing.T) {
	target := si.StringID("target")
	msg := NewSimRequest(target.Key())

	require.Equal(t, &SimMessage[key.Key256]{}, msg.EmptyResponse())

	b := key.Equal(msg.Target(), target.Key())
	require.True(t, b)
	require.Nil(t, msg.CloserNodes())
}

func TestSimResponse(t *testing.T) {
	closerPeers := []address.NodeAddr[key.Key256]{si.StringID("peer1"), si.StringID("peer2")}
	msg := NewSimResponse(closerPeers)

	// require.Nil(t, msg.Target())
	require.Equal(t, len(closerPeers), len(msg.CloserNodes()))
	for i, peer := range closerPeers {
		require.Equal(t, closerPeers[i], peer)
	}
}
