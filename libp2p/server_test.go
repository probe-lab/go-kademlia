package libp2p

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/server/basicserver"
	"github.com/plprobelab/go-kademlia/sim"
)

type invalidEndpoint[K kad.Key[K], A kad.Address[A]] struct{}

// var _ endpoint.Endpoint = (*invalidEndpoint)(nil)

func (e *invalidEndpoint[K, A]) MaybeAddToPeerstore(context.Context, kad.NodeInfo[K, A],
	time.Duration,
) error {
	return nil
}

func (e *invalidEndpoint[K, A]) SendRequestHandleResponse(context.Context,
	address.ProtocolID, kad.NodeID[K], message.MinKadMessage,
	message.MinKadMessage, time.Duration, endpoint.ResponseHandlerFn[K, A],
) error {
	return nil
}

func (e *invalidEndpoint[K, A]) KadKey() K {
	var v K
	return v
}

func (e *invalidEndpoint[K, A]) NetworkAddress(kad.NodeID[K]) (kad.NodeInfo[K, A], error) {
	return nil, nil
}

func TestInvalidIpfsv1Requests(t *testing.T) {
	ctx := context.Background()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	selfPid, err := peer.Decode("1EooooSELF")
	require.NoError(t, err)
	self := NewPeerID(selfPid)

	invalidEP := &invalidEndpoint[key.Key256, multiaddr.Multiaddr]{}
	rt := simplert.New(self.Key(), 4)

	nPeers := 6
	peerids := make([]kad.NodeID[key.Key256], nPeers)

	for i := 0; i < nPeers; i++ {
		// create peer.ID "1EoooPEER2" until "1EoooPEER7"
		p, err := peer.Decode("1EoooPEER" + fmt.Sprint(i+2))
		require.NoError(t, err)
		peerids[i] = NewPeerID(p)

		// ipfsv1 needs to have addrinfo.AddrInfo stored in the endpoint
		addr := multiaddr.StringCast("/ip4/" + fmt.Sprint(i+2) + "." +
			fmt.Sprint(i+2) + "." + fmt.Sprint(i+2) + "." + fmt.Sprint(i+2))
		addrInfo := NewAddrInfo(peer.AddrInfo{
			ID:    p,
			Addrs: []multiaddr.Multiaddr{addr},
		})
		// add peers to routing table and peerstore
		err = invalidEP.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
		require.NoError(t, err)
		success := rt.AddNode(peerids[i])
		require.True(t, success)
	}

	s0 := basicserver.NewBasicServer[multiaddr.Multiaddr](rt, invalidEP, basicserver.WithPeerstoreTTL(peerstoreTTL),
		basicserver.WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requesterPid, err := peer.Decode("1WoooREQUESTER")
	require.NoError(t, err)
	requester := NewPeerID(requesterPid)

	// request will fail as endpoint is not a networked endpoint
	req0 := FindPeerRequest(self)
	msg, err := s0.HandleRequest(ctx, requester, req0)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, basicserver.ErrNotNetworkedEndpoint, err)

	// replace the key with an invalid peerid
	req0.Key = []byte("invalid key")
	msg, err = s0.HandleRequest(ctx, requester, req0)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, basicserver.ErrIpfsV1InvalidPeerID, err)

	req0.Type = -1 // invalid request type
	msg, err = s0.HandleRequest(ctx, requester, req0)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, basicserver.ErrIpfsV1InvalidRequest, err)

	req1 := struct{}{} // invalid message format
	msg, err = s0.HandleRequest(ctx, requester, req1)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, basicserver.ErrUnknownMessageFormat, err)
}

func TestIPFSv1Handling(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	selfPid, err := peer.Decode("1EooooSELF")
	require.NoError(t, err)
	self := NewPeerID(selfPid)

	router := sim.NewRouter[key.Key256, multiaddr.Multiaddr]()
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := sim.NewEndpoint[key.Key256, multiaddr.Multiaddr](self.NodeID(), sched, router)
	rt := simplert.New(self.Key(), 4)

	nPeers := 6
	peerids := make([]kad.NodeID[key.Key256], nPeers)

	for i := 0; i < nPeers; i++ {
		// create peer.ID "1EoooPEER2" until "1EoooPEER7"
		p, err := peer.Decode("1EoooPEER" + fmt.Sprint(i+2))
		require.NoError(t, err)
		peerids[i] = NewPeerID(p)

		// ipfsv1 needs to have addrinfo.AddrInfo stored in the endpoint
		addr := multiaddr.StringCast("/ip4/" + fmt.Sprint(i+2) + "." +
			fmt.Sprint(i+2) + "." + fmt.Sprint(i+2) + "." + fmt.Sprint(i+2))
		addrInfo := NewAddrInfo(peer.AddrInfo{
			ID:    p,
			Addrs: []multiaddr.Multiaddr{addr},
		})
		if i == 1 {
			// no addresses for peer 1, it should not be returned even though
			// it is among the numberOfCloserPeersToSend closer peers
			addrInfo = NewAddrInfo(peer.AddrInfo{
				ID:    p,
				Addrs: nil,
			})
		}
		// add peers to routing table and peerstore
		err = fakeEndpoint.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
		require.NoError(t, err)
		success := rt.AddNode(peerids[i])
		require.True(t, success)
	}

	// self:       f3a2eb191b47b031e317e39776d09938c6d97a50b52e71acc4c4173275b998bd
	// peerids[0]: e69614c5fcb92e8fbf2aa5785904fec5a67524ac7cd513f32bc7ab38621b4b7b (bucket 3)
	// peerids[1]: 6ab9cb73bbd52ad2bb6ac4048e988478bf076df9b39e072f30b4722639382683 (bucket 0)
	// peerids[2]: 69b9104f74ca05073a1bb658155fa4549fcc8db470947915a6e2750185dc1f81 (bucket 0)
	// peerids[3]: 4eaafc67b177fa53ee6de27d1646f7862fb2957878bcc8d60dfa67b7832bb28b (bucket 0)
	// peerids[4]: ab6c9fe862d32ff3170ed43600742b2abbb52f09216afa139cb89842e083ce4e (bucket 1)
	// peerids[5]: 00ca8d64555add66790c4fb3e62075911a02a3577622fa69279731e82c135b8a (bucket 0)

	s0 := basicserver.NewBasicServer[multiaddr.Multiaddr](rt, fakeEndpoint, basicserver.WithPeerstoreTTL(peerstoreTTL),
		basicserver.WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requesterPid, err := peer.Decode("1WoooREQUESTER")
	require.NoError(t, err)
	requester := NewPeerID(requesterPid)
	fakeEndpoint.MaybeAddToPeerstore(ctx, NewAddrInfo(peer.AddrInfo{
		ID:    requesterPid,
		Addrs: nil,
	}), peerstoreTTL)

	req0 := FindPeerRequest(self)
	msg, err := s0.HandleRequest(ctx, requester, req0)
	require.NoError(t, err)

	resp, ok := msg.(*Message)
	require.True(t, ok)

	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend-1)
	// -1 because peerids[1] has no addresses, so it is counted as one of the
	// numberOfCloserPeersToSend closer peers, but it is not returned

	order := []kad.NodeID[key.Key256]{peerids[0], peerids[4], peerids[2]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p.ID())
	}
}
