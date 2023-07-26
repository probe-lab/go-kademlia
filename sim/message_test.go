package sim

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

var (
	_ kad.Request[key.Key8, net.IP]  = (*SimMessage[key.Key8, net.IP])(nil)
	_ kad.Response[key.Key8, net.IP] = (*SimMessage[key.Key8, net.IP])(nil)
)

func TestRequest(t *testing.T) {
	target := kadtest.StringID("target")
	msg := NewRequest[key.Key256, net.IP](target.Key())

	require.Equal(t, &SimMessage[key.Key256, net.IP]{}, msg.EmptyResponse())

	b := key.Equal(msg.Target(), target.Key())
	require.True(t, b)
	require.Nil(t, msg.CloserNodes())
}

func TestResponse(t *testing.T) {
	closerPeers := []kad.NodeInfo[key.Key256, net.IP]{
		kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.StringID("peer1").Key()), nil),
		kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.StringID("peer2").Key()), nil),
	}
	msg := NewResponse(closerPeers)

	// require.Nil(t, msg.Target())
	require.Equal(t, len(closerPeers), len(msg.CloserNodes()))
	for i, peer := range closerPeers {
		require.Equal(t, closerPeers[i], peer)
	}
}
