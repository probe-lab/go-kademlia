package sim

import (
	"context"
	"net"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
)

func TestRouter(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	router := NewRouter[key.Key256, net.IP]()

	nPeers := 5
	scheds := make([]scheduler.AwareScheduler, nPeers)
	ids := make([]kad.NodeID[key.Key256], nPeers)
	fakeEndpoints := make([]*Endpoint[key.Key256, net.IP], nPeers)
	for i := 0; i < nPeers; i++ {
		ids[i] = kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{byte(i)}))
		scheds[i] = simplescheduler.NewSimpleScheduler(clk)
		fakeEndpoints[i] = NewEndpoint[key.Key256, net.IP](ids[i], scheds[i], router)
	}

	protoID := address.ProtocolID("/test/proto")
	sid, err := router.SendMessage(ctx, ids[0], ids[1], protoID, 0, nil)
	require.NoError(t, err)
	require.Equal(t, endpoint.StreamID(1), sid)
	require.True(t, scheds[1].RunOne(ctx))
	require.False(t, scheds[1].RunOne(ctx))

	newSid := endpoint.StreamID(100)
	sid, err = router.SendMessage(ctx, ids[4], ids[2], protoID, newSid, nil)
	require.NoError(t, err)
	require.Equal(t, newSid, sid)
	require.True(t, scheds[2].RunOne(ctx))
	require.False(t, scheds[2].RunOne(ctx))

	sid, err = router.SendMessage(ctx, ids[2], ids[3], protoID, 0, nil)
	require.NoError(t, err)
	require.Equal(t, endpoint.StreamID(2), sid)
	require.True(t, scheds[3].RunOne(ctx))
	require.False(t, scheds[3].RunOne(ctx))

	notRegisteredID := kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{byte(100)}))
	sid, err = router.SendMessage(ctx, ids[3], notRegisteredID, protoID, 0, nil)
	require.Error(t, err)
	require.Equal(t, endpoint.ErrUnknownPeer, err)
	require.Equal(t, endpoint.StreamID(0), sid)

	router.RemovePeer(ids[1])
	sid, err = router.SendMessage(ctx, ids[0], ids[1], protoID, 0, nil)
	require.Error(t, err)
	require.Equal(t, endpoint.ErrUnknownPeer, err)
	require.Equal(t, endpoint.StreamID(0), sid)
	require.False(t, scheds[1].RunOne(ctx))
}
