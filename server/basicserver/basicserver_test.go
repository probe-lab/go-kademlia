package basicserver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/endpoint/fakeendpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/ipfsv1"
	"github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routingtable/simplert"
	"github.com/stretchr/testify/require"
)

// remotePeers with bucket assignments wrt to self
var kadRemotePeers = []*kadid.KadID{
	{KadKey: []byte{0b10001000}}, // 1000 1000 (bucket 0)
	{KadKey: []byte{0b11010010}}, // 1101 0010 (bucket 0)
	{KadKey: []byte{0b01001011}}, // 0100 1011 (bucket 1)
	{KadKey: []byte{0b01010011}}, // 0101 0011 (bucket 1)
	{KadKey: []byte{0b00101110}}, // 0010 1110 (bucket 2)
	{KadKey: []byte{0b00110110}}, // 0011 0110 (bucket 2)
	{KadKey: []byte{0b00011111}}, // 0001 1111 (bucket 3)
	{KadKey: []byte{0b00010001}}, // 0001 0001 (bucket 3)
	{KadKey: []byte{0b00001000}}, // 0000 1000 (bucket 4)
}

func TestSimMessageHandling(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	var self = kadid.KadID{KadKey: []byte{0x00}} // 0000 0000

	router := fakeendpoint.NewFakeRouter()
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := fakeendpoint.NewFakeEndpoint(self, sched, router)
	rt := simplert.NewSimpleRT(self.Key(), 2)

	// add peers to routing table and peerstore
	for _, p := range kadRemotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddPeer(ctx, p)
		require.NoError(t, err)
		require.True(t, success)
	}

	s0 := NewBasicServer(rt, fakeEndpoint, WithPeerstoreTTL(peerstoreTTL),
		WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requester := kadid.KadID{KadKey: []byte{0b00000001}} // 0000 0001
	fakeEndpoint.MaybeAddToPeerstore(ctx, requester, peerstoreTTL)

	req0 := simmessage.NewSimRequest([]byte{0b00000000})
	msg, err := s0.HandleRequest(ctx, requester, req0)
	require.NoError(t, err)

	resp, ok := msg.(message.MinKadResponseMessage)
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0000 0000
	// [8] 0000 1000, [7] 0001 0001, [6] 0001 1111, [4] 0010 1110
	order := []*kadid.KadID{kadRemotePeers[8], kadRemotePeers[7],
		kadRemotePeers[6], kadRemotePeers[4]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	req1 := simmessage.NewSimRequest([]byte{0b11111111})
	msg, err = s0.HandleRequest(ctx, requester, req1)
	require.NoError(t, err)
	resp, ok = msg.(message.MinKadResponseMessage)
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 1111 1111
	// [1] 1101 0010, [0] 1000 1000, [3] 0101 0011, [2] 0100 1011
	order = []*kadid.KadID{kadRemotePeers[1], kadRemotePeers[0],
		kadRemotePeers[3], kadRemotePeers[2]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	numberOfCloserPeersToSend = 3
	s1 := NewBasicServer(rt, fakeEndpoint, WithNumberUsefulCloserPeers(3))

	req2 := simmessage.NewSimRequest([]byte{0b01100000})
	msg, err = s1.HandleRequest(ctx, requester, req2)
	require.NoError(t, err)
	resp, ok = msg.(message.MinKadResponseMessage)
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0110 0000
	// [2] 0100 1011, [3] 0101 0011, [4] 0010 1110
	order = []*kadid.KadID{kadRemotePeers[2], kadRemotePeers[3], kadRemotePeers[4]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}
}

func TestInvalidSimRequests(t *testing.T) {
	ctx := context.Background()
	// invalid option
	s := NewBasicServer(nil, nil, func(*Config) error {
		return errors.New("invalid option")
	})
	require.Nil(t, s)

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint

	clk := clock.New()
	router := fakeendpoint.NewFakeRouter()

	var self = kadid.KadID{KadKey: []byte{0x00}} // 0000 0000

	// create a valid server
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := fakeendpoint.NewFakeEndpoint(self, sched, router)
	rt := simplert.NewSimpleRT(self.Key(), 2)

	// add peers to routing table and peerstore
	for _, p := range kadRemotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddPeer(ctx, p)
		require.NoError(t, err)
		require.True(t, success)
	}

	s = NewBasicServer(rt, fakeEndpoint)
	require.NotNil(t, s)

	requester := kadid.KadID{KadKey: []byte{0b00000001}} // 0000 0001

	// invalid message format (not a SimMessage)
	req0 := struct{}{}
	_, err := s.HandleFindNodeRequest(ctx, requester, req0)
	require.Error(t, err)

	// empty request
	req1 := &simmessage.SimMessage{}
	s.HandleFindNodeRequest(ctx, requester, req1)

	// request with invalid key (not matching the expected length)
	req2 := simmessage.NewSimRequest([]byte{0b00000000, 0b00000001})
	s.HandleFindNodeRequest(ctx, requester, req2)
}

func TestIPFSv1Handling(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	selfPid, err := peer.Decode("1EooooSELF")
	require.NoError(t, err)
	self := peerid.NewPeerID(selfPid)

	router := fakeendpoint.NewFakeRouter()
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := fakeendpoint.NewFakeEndpoint(self, sched, router)
	rt := simplert.NewSimpleRT(self.Key(), 4)

	nPeers := 6
	peerids := make([]address.NodeID, nPeers)

	for i := 0; i < nPeers; i++ {
		// create peer.ID "1EoooPEER2" until "1EoooPEER7"
		p, err := peer.Decode("1EoooPEER" + fmt.Sprint(i+2))
		require.NoError(t, err)
		peerids[i] = peerid.NewPeerID(p)

		// ipfsv1 needs to have addrinfo.AddrInfo stored in the endpoint
		addr := multiaddr.StringCast("/ip4/" + fmt.Sprint(i+2) + "." +
			fmt.Sprint(i+2) + "." + fmt.Sprint(i+2) + "." + fmt.Sprint(i+2))
		addrInfo := addrinfo.NewAddrInfo(peer.AddrInfo{
			ID:    p,
			Addrs: []multiaddr.Multiaddr{addr},
		})
		if i == 1 {
			// no addresses for peer 1, it should not be returned even though
			// it is among the numberOfCloserPeersToSend closer peers
			addrInfo = addrinfo.NewAddrInfo(peer.AddrInfo{
				ID:    p,
				Addrs: []multiaddr.Multiaddr{},
			})
		}
		// add peers to routing table and peerstore
		err = fakeEndpoint.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddPeer(ctx, peerids[i])
		require.NoError(t, err)
		require.True(t, success)
	}

	// self:       f3a2eb191b47b031e317e39776d09938c6d97a50b52e71acc4c4173275b998bd
	// peerids[0]: e69614c5fcb92e8fbf2aa5785904fec5a67524ac7cd513f32bc7ab38621b4b7b (bucket 3)
	// peerids[1]: 6ab9cb73bbd52ad2bb6ac4048e988478bf076df9b39e072f30b4722639382683 (bucket 0)
	// peerids[2]: 69b9104f74ca05073a1bb658155fa4549fcc8db470947915a6e2750185dc1f81 (bucket 0)
	// peerids[3]: 4eaafc67b177fa53ee6de27d1646f7862fb2957878bcc8d60dfa67b7832bb28b (bucket 0)
	// peerids[4]: ab6c9fe862d32ff3170ed43600742b2abbb52f09216afa139cb89842e083ce4e (bucket 1)
	// peerids[5]: 00ca8d64555add66790c4fb3e62075911a02a3577622fa69279731e82c135b8a (bucket 0)

	s0 := NewBasicServer(rt, fakeEndpoint, WithPeerstoreTTL(peerstoreTTL),
		WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requesterPid, err := peer.Decode("1WoooREQUESTER")
	require.NoError(t, err)
	requester := peerid.NewPeerID(requesterPid)
	fakeEndpoint.MaybeAddToPeerstore(ctx, addrinfo.NewAddrInfo(peer.AddrInfo{
		ID:    requesterPid,
		Addrs: nil,
	}), peerstoreTTL)

	req0 := ipfsv1.FindPeerRequest(self)
	msg, err := s0.HandleRequest(ctx, requester, req0)
	require.NoError(t, err)

	resp, ok := msg.(*ipfsv1.Message)
	require.True(t, ok)

	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend-1)
	// -1 because peerids[1] has no addresses, so it is counted as one of the
	// numberOfCloserPeersToSend closer peers, but it is not returned

	order := []address.NodeID{peerids[0], peerids[4], peerids[2]}
	for i, p := range resp.CloserNodes() {
		ai, ok := p.(*addrinfo.AddrInfo)
		require.True(t, ok)
		require.Equal(t, order[i], ai.PeerID())
	}
}

type invalidEndpoint struct{}

var _ endpoint.Endpoint = (*invalidEndpoint)(nil)

func (e *invalidEndpoint) MaybeAddToPeerstore(context.Context, address.NodeAddr,
	time.Duration) error {
	return nil
}

func (e *invalidEndpoint) SendRequestHandleResponse(context.Context,
	address.ProtocolID, address.NodeID, message.MinKadMessage,
	message.MinKadMessage, time.Duration, endpoint.ResponseHandlerFn) error {
	return nil
}

func (e *invalidEndpoint) KadKey() key.KadKey {
	return make([]byte, 0)
}

func (e *invalidEndpoint) NetworkAddress(address.NodeID) (address.NodeAddr, error) {
	return nil, nil
}

func TestInvalidIpfsv1Requests(t *testing.T) {
	ctx := context.Background()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	selfPid, err := peer.Decode("1EooooSELF")
	require.NoError(t, err)
	self := peerid.NewPeerID(selfPid)

	invalidEP := &invalidEndpoint{}
	rt := simplert.NewSimpleRT(self.Key(), 4)

	nPeers := 6
	peerids := make([]address.NodeID, nPeers)

	for i := 0; i < nPeers; i++ {
		// create peer.ID "1EoooPEER2" until "1EoooPEER7"
		p, err := peer.Decode("1EoooPEER" + fmt.Sprint(i+2))
		require.NoError(t, err)
		peerids[i] = peerid.NewPeerID(p)

		// ipfsv1 needs to have addrinfo.AddrInfo stored in the endpoint
		addr := multiaddr.StringCast("/ip4/" + fmt.Sprint(i+2) + "." +
			fmt.Sprint(i+2) + "." + fmt.Sprint(i+2) + "." + fmt.Sprint(i+2))
		addrInfo := addrinfo.NewAddrInfo(peer.AddrInfo{
			ID:    p,
			Addrs: []multiaddr.Multiaddr{addr},
		})
		// add peers to routing table and peerstore
		err = invalidEP.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddPeer(ctx, peerids[i])
		require.NoError(t, err)
		require.True(t, success)
	}

	s0 := NewBasicServer(rt, invalidEP, WithPeerstoreTTL(peerstoreTTL),
		WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requesterPid, err := peer.Decode("1WoooREQUESTER")
	require.NoError(t, err)
	requester := peerid.NewPeerID(requesterPid)

	// request will fail as endpoint is not a networked endpoint
	req0 := ipfsv1.FindPeerRequest(self)
	msg, err := s0.HandleRequest(ctx, requester, req0)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, ErrNotNetworkedEndpoint, err)

	// replace the key with an invalid peerid
	req0.Key = []byte("invalid key")
	msg, err = s0.HandleRequest(ctx, requester, req0)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, ErrIpfsV1InvalidPeerID, err)

	req0.Type = -1 // invalid request type
	msg, err = s0.HandleRequest(ctx, requester, req0)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, ErrIpfsV1InvalidRequest, err)

	req1 := struct{}{} // invalid message format
	msg, err = s0.HandleRequest(ctx, requester, req1)

	require.Nil(t, msg)
	require.Error(t, err)
	require.Equal(t, ErrUnknownMessageFormat, err)
}
