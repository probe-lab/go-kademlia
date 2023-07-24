package sim

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	si "github.com/plprobelab/go-kademlia/network/address/stringid"
	"github.com/plprobelab/go-kademlia/network/message"
)

var (
	_ message.MinKadRequestMessage[key.Key8]  = (*Message[key.Key8])(nil)
	_ message.MinKadResponseMessage[key.Key8] = (*Message[key.Key8])(nil)
)

func TestRequest(t *testing.T) {
	target := si.StringID("target")
	msg := NewRequest(target.Key())

	require.Equal(t, &Message[key.Key256]{}, msg.EmptyResponse())

	b := key.Equal(msg.Target(), target.Key())
	require.True(t, b)
	require.Nil(t, msg.CloserNodes())
}

func TestResponse(t *testing.T) {
	closerPeers := []address.NodeAddr[key.Key256]{si.StringID("peer1"), si.StringID("peer2")}
	msg := NewResponse(closerPeers)

	// require.Nil(t, msg.Target())
	require.Equal(t, len(closerPeers), len(msg.CloserNodes()))
	for i, peer := range closerPeers {
		require.Equal(t, closerPeers[i], peer)
	}
}
