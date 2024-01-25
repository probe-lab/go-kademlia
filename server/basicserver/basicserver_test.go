package basicserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/probe-lab/go-kademlia/libp2p"
	"github.com/probe-lab/go-kademlia/network/address"
	"github.com/probe-lab/go-kademlia/network/endpoint"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/probe-lab/go-kademlia/event"
	"github.com/probe-lab/go-kademlia/internal/kadtest"
	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
	"github.com/probe-lab/go-kademlia/routing/simplert"
	"github.com/probe-lab/go-kademlia/sim"
	"github.com/stretchr/testify/require"
)

// remotePeers with bucket assignments wrt to self
var kadRemotePeers = []kad.NodeInfo[key.Key256, net.IP]{
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b10001000})), nil), // 1000 1000 (bucket 0)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b11010010})), nil), // 1101 0010 (bucket 0)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b01001011})), nil), // 0100 1011 (bucket 1)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b01010011})), nil), // 0101 0011 (bucket 1)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00101110})), nil), // 0010 1110 (bucket 2)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00110110})), nil), // 0011 0110 (bucket 2)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00011111})), nil), // 0001 1111 (bucket 3)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00010001})), nil), // 0001 0001 (bucket 3)
	kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00001000})), nil), // 0000 1000 (bucket 4)
}

func TestSimMessageHandling(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	self := kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(key.ZeroKey256()), nil) // 0000 0000

	router := sim.NewRouter[key.Key256, net.IP]()
	sched := event.NewSimpleScheduler(clk)
	fakeEndpoint := sim.NewEndpoint[key.Key256, net.IP](self.ID(), sched, router)
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](self.ID(), 2)

	// add peers to routing table and peerstore
	for _, p := range kadRemotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success := rt.AddNode(p.ID())
		require.True(t, success)
	}

	s0 := NewBasicServer[net.IP](rt, fakeEndpoint, WithPeerstoreTTL(peerstoreTTL),
		WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requester := kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00000001})), nil) // 0000 0001
	fakeEndpoint.MaybeAddToPeerstore(ctx, requester, peerstoreTTL)

	req0 := sim.NewRequest[key.Key256, net.IP](kadtest.Key256WithLeadingBytes([]byte{0b00000000}))
	msg, err := s0.HandleRequest(ctx, requester.ID(), req0)
	require.NoError(t, err)

	resp, ok := msg.(kad.Response[key.Key256, net.IP])
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0000 0000
	// [8] 0000 1000, [7] 0001 0001, [6] 0001 1111, [4] 0010 1110
	order := []kad.NodeInfo[key.Key256, net.IP]{
		kadRemotePeers[8], kadRemotePeers[7],
		kadRemotePeers[6], kadRemotePeers[4],
	}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	req1 := sim.NewRequest[key.Key256, net.IP](kadtest.Key256WithLeadingBytes([]byte{0b11111111}))
	msg, err = s0.HandleRequest(ctx, requester.ID(), req1)
	require.NoError(t, err)
	resp, ok = msg.(kad.Response[key.Key256, net.IP])
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 1111 1111
	// [1] 1101 0010, [0] 1000 1000, [3] 0101 0011, [2] 0100 1011
	order = []kad.NodeInfo[key.Key256, net.IP]{
		kadRemotePeers[1], kadRemotePeers[0],
		kadRemotePeers[3], kadRemotePeers[2],
	}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	numberOfCloserPeersToSend = 3
	s1 := NewBasicServer[net.IP](rt, fakeEndpoint, WithNumberUsefulCloserPeers(3))

	req2 := sim.NewRequest[key.Key256, net.IP](kadtest.Key256WithLeadingBytes([]byte{0b01100000}))
	msg, err = s1.HandleRequest(ctx, requester.ID(), req2)
	require.NoError(t, err)
	resp, ok = msg.(kad.Response[key.Key256, net.IP])
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0110 0000
	// [2] 0100 1011, [3] 0101 0011, [4] 0010 1110
	order = []kad.NodeInfo[key.Key256, net.IP]{kadRemotePeers[2], kadRemotePeers[3], kadRemotePeers[4]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}
}

func TestInvalidSimRequests(t *testing.T) {
	ctx := context.Background()
	// invalid option
	s := NewBasicServer[net.IP](nil, nil, func(*Config) error {
		return errors.New("invalid option")
	})
	require.Nil(t, s)

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint

	clk := clock.New()
	router := sim.NewRouter[key.Key256, net.IP]()

	self := kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(key.ZeroKey256()), nil) // 0000 0000

	// create a valid server
	sched := event.NewSimpleScheduler(clk)
	fakeEndpoint := sim.NewEndpoint[key.Key256, net.IP](self.ID(), sched, router)
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](self.ID(), 2)

	// add peers to routing table and peerstore
	for _, p := range kadRemotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success := rt.AddNode(p.ID())
		require.True(t, success)
	}

	s = NewBasicServer[net.IP](rt, fakeEndpoint)
	require.NotNil(t, s)

	requester := kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0b00000001})) // 0000 0001

	// invalid message format (not a Message)
	req0 := struct{}{}
	_, err := s.HandleFindNodeRequest(ctx, requester, req0)
	require.Error(t, err)

	// empty request
	req1 := &sim.Message[key.Key256, net.IP]{}
	s.HandleFindNodeRequest(ctx, requester, req1)

	// request with invalid key (not matching the expected length)
	req2 := sim.NewRequest[key.Key32, net.IP](key.Key32(0b00000000000000010000000000000000))
	s.HandleFindNodeRequest(ctx, requester, req2)
}

func TestSimRequestNoNetworkAddress(t *testing.T) {
	ctx := context.Background()
	// invalid option
	s := NewBasicServer[net.IP](nil, nil, func(*Config) error {
		return errors.New("invalid option")
	})
	require.Nil(t, s)

	clk := clock.New()
	router := sim.NewRouter[key.Key256, net.IP]()

	self := kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0})), nil) // 0000 0000

	// create a valid server
	sched := event.NewSimpleScheduler(clk)
	fakeEndpoint := sim.NewEndpoint(self.ID(), sched, router)
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](self.ID(), 2)

	parsed, err := peer.Decode("1EooooPEER")
	require.NoError(t, err)
	addrInfo := libp2p.NewAddrInfo(peer.AddrInfo{
		ID:    parsed,
		Addrs: nil,
	})

	// add peer to routing table, but NOT to peerstore
	success := rt.AddNode(addrInfo.ID())
	require.True(t, success)

	s = NewBasicServer[net.IP](rt, fakeEndpoint)
	require.NotNil(t, s)

	require.NotNil(t, s)

	requester := kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0x80}))

	// sim request message (for net.IP key)
	req := sim.NewRequest[key.Key256, net.IP](requester.Key())
	msg, err := s.HandleFindNodeRequest(ctx, requester, req)
	require.NoError(t, err)
	resp, ok := msg.(kad.Response[key.Key256, net.IP])
	require.True(t, ok)
	fmt.Println(resp.CloserNodes())
	require.Len(t, resp.CloserNodes(), 0)
}

type invalidEndpoint[K kad.Key[K], A kad.Address[A]] struct{}

// var _ endpoint.Endpoint = (*invalidEndpoint)(nil)

func (e *invalidEndpoint[K, A]) MaybeAddToPeerstore(context.Context, kad.NodeInfo[K, A],
	time.Duration,
) error {
	return nil
}

func (e *invalidEndpoint[K, A]) SendRequestHandleResponse(context.Context,
	address.ProtocolID, kad.NodeID[K], kad.Message,
	kad.Message, time.Duration, endpoint.ResponseHandlerFn[K, A],
) error {
	return nil
}

func (e *invalidEndpoint[K, A]) SendMessage(context.Context, address.ProtocolID, kad.NodeID[K], kad.Request[K, A]) (kad.Response[K, A], error) {
	return nil, nil
}

func (e *invalidEndpoint[K, A]) Key() K {
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
	self := libp2p.NewPeerID(selfPid)

	invalidEP := &invalidEndpoint[key.Key256, multiaddr.Multiaddr]{}
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](self, 4)

	nPeers := 6
	peerids := make([]kad.NodeID[key.Key256], nPeers)

	for i := 0; i < nPeers; i++ {
		// create peer.ID "1EoooPEER2" until "1EoooPEER7"
		p, err := peer.Decode("1EoooPEER" + fmt.Sprint(i+2))
		require.NoError(t, err)
		peerids[i] = libp2p.NewPeerID(p)

		// ipfsv1 needs to have addrinfo.AddrInfo stored in the endpoint
		addr := multiaddr.StringCast("/ip4/" + fmt.Sprint(i+2) + "." +
			fmt.Sprint(i+2) + "." + fmt.Sprint(i+2) + "." + fmt.Sprint(i+2))
		addrInfo := libp2p.NewAddrInfo(peer.AddrInfo{
			ID:    p,
			Addrs: []multiaddr.Multiaddr{addr},
		})
		// add peers to routing table and peerstore
		err = invalidEP.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
		require.NoError(t, err)
		success := rt.AddNode(peerids[i])
		require.True(t, success)
	}

	s0 := NewBasicServer[multiaddr.Multiaddr](rt, invalidEP, WithPeerstoreTTL(peerstoreTTL),
		WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requesterPid, err := peer.Decode("1WoooREQUESTER")
	require.NoError(t, err)
	requester := libp2p.NewPeerID(requesterPid)

	// request will fail as endpoint is not a networked endpoint
	req0 := libp2p.FindPeerRequest(self)
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

func TestIPFSv1Handling(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	selfPid, err := peer.Decode("1EooooSELF")
	require.NoError(t, err)
	self := libp2p.NewPeerID(selfPid)

	router := sim.NewRouter[key.Key256, multiaddr.Multiaddr]()
	sched := event.NewSimpleScheduler(clk)
	fakeEndpoint := sim.NewEndpoint[key.Key256, multiaddr.Multiaddr](self.NodeID(), sched, router)
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](self, 4)

	nPeers := 6
	peerids := make([]kad.NodeID[key.Key256], nPeers)

	for i := 0; i < nPeers; i++ {
		// create peer.ID "1EoooPEER2" until "1EoooPEER7"
		p, err := peer.Decode("1EoooPEER" + fmt.Sprint(i+2))
		require.NoError(t, err)
		peerids[i] = libp2p.NewPeerID(p)

		// ipfsv1 needs to have addrinfo.AddrInfo stored in the endpoint
		addr := multiaddr.StringCast("/ip4/" + fmt.Sprint(i+2) + "." +
			fmt.Sprint(i+2) + "." + fmt.Sprint(i+2) + "." + fmt.Sprint(i+2))
		addrInfo := libp2p.NewAddrInfo(peer.AddrInfo{
			ID:    p,
			Addrs: []multiaddr.Multiaddr{addr},
		})
		if i == 1 {
			// no addresses for peer 1, it should not be returned even though
			// it is among the numberOfCloserPeersToSend closer peers
			addrInfo = libp2p.NewAddrInfo(peer.AddrInfo{
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

	s0 := NewBasicServer[multiaddr.Multiaddr](rt, fakeEndpoint, WithPeerstoreTTL(peerstoreTTL),
		WithNumberUsefulCloserPeers(numberOfCloserPeersToSend))

	requesterPid, err := peer.Decode("1WoooREQUESTER")
	require.NoError(t, err)
	requester := libp2p.NewPeerID(requesterPid)
	fakeEndpoint.MaybeAddToPeerstore(ctx, libp2p.NewAddrInfo(peer.AddrInfo{
		ID:    requesterPid,
		Addrs: nil,
	}), peerstoreTTL)

	req0 := libp2p.FindPeerRequest(self)
	msg, err := s0.HandleRequest(ctx, requester, req0)
	require.NoError(t, err)

	resp, ok := msg.(*libp2p.Message)
	require.True(t, ok)

	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend-1)
	// -1 because peerids[1] has no addresses, so it is counted as one of the
	// numberOfCloserPeersToSend closer peers, but it is not returned

	order := []kad.NodeID[key.Key256]{peerids[0], peerids[4], peerids[2]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p.ID())
	}
}
