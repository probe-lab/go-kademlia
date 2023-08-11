package libp2p

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/event"
	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/query/simplequery"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/sim"
)

// TestLibp2pCornerCase tests that the newRequest(ctx) can fail fast if the
// selected peer has an invalid format (e.g for libp2p something that isn't a
// peerid.PeerID). This test should be performed using the libp2p endpoint
// because the fakeendpoint can never fail fast.
func TestLibp2pCornerCase(t *testing.T) {
	t.Skip()

	ctx := context.Background()
	clk := clock.New()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 1
	peerstoreTTL := time.Minute

	h, err := libp2p.New()
	require.NoError(t, err)
	id := NewPeerID(h.ID())
	sched := event.NewSimpleScheduler(clk)
	libp2pEndpoint := NewLibp2pEndpoint(ctx, h, sched)
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](id, bucketSize)

	parsed, err := peer.Decode("1D3oooUnknownPeer")
	require.NoError(t, err)
	addrInfo := NewAddrInfo(peer.AddrInfo{
		ID:    parsed,
		Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/1.1.1.1/tcp/1")},
	})
	success := rt.AddNode(addrInfo.ID())
	require.True(t, success)
	err = libp2pEndpoint.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
	require.NoError(t, err)

	queryOpts := []simplequery.Option[key.Key256, multiaddr.Multiaddr]{
		simplequery.WithProtocolID[key.Key256, multiaddr.Multiaddr](protoID),
		simplequery.WithConcurrency[key.Key256, multiaddr.Multiaddr](1),
		simplequery.WithNumberUsefulCloserPeers[key.Key256, multiaddr.Multiaddr](bucketSize),
		simplequery.WithRequestTimeout[key.Key256, multiaddr.Multiaddr](time.Millisecond),
		simplequery.WithEndpoint[key.Key256, multiaddr.Multiaddr](libp2pEndpoint),
		simplequery.WithRoutingTable[key.Key256, multiaddr.Multiaddr](rt),
		simplequery.WithScheduler[key.Key256, multiaddr.Multiaddr](sched),
	}

	req := sim.NewRequest[key.Key256, multiaddr.Multiaddr](kadtest.NewStringID("RandomKey").Key())

	q2, err := simplequery.NewSimpleQuery[key.Key256, multiaddr.Multiaddr](ctx, NewPeerID(h.ID()), req, queryOpts...)
	require.NoError(t, err)

	// TODO: moved this test here from simplequery to remove libp2p dependency from there
	// this means we can't set the peerlist endpoint to nil...
	// skipping this test for now.
	_ = q2
	//// set the peerlist endpoint to nil, to allow invalid NodeIDs in peerlist
	//q2.peerlist.endpoint = nil
	//// change the node id of the queued peer. sending the message with
	//// SendRequestHandleResponse will fail fast (no new go routine created)
	//q2.peerlist.closest.id = kadtest.NewID(key.ZeroKey256())

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
}
