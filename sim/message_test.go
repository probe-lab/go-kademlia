package sim

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/message"
)

var (
	_ message.MinKadRequestMessage[key.Key8, any]  = (*Message[key.Key8, any])(nil)
	_ message.MinKadResponseMessage[key.Key8, any] = (*Message[key.Key8, any])(nil)
)

func TestRequest(t *testing.T) {
	target := kadtest.StringID("target")
	msg := NewRequest[key.Key256, any](target.Key())

	require.Equal(t, &Message[key.Key256, any]{}, msg.EmptyResponse())

	b := key.Equal(msg.Target(), target.Key())
	require.True(t, b)
	require.Nil(t, msg.CloserNodes())
}

func TestResponse(t *testing.T) {
	closerPeers := []kad.NodeInfo[key.Key256, any]{
		kadtest.NewAddr[key.Key256, any](kadtest.NewID(kadtest.StringID("peer1").Key()), nil),
		kadtest.NewAddr[key.Key256, any](kadtest.NewID(kadtest.StringID("peer2").Key()), nil),
	}
	msg := NewResponse(closerPeers)

	// require.Nil(t, msg.Target())
	require.Equal(t, len(closerPeers), len(msg.CloserNodes()))
	for i, peer := range closerPeers {
		require.Equal(t, closerPeers[i], peer)
	}
}
