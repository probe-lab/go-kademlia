package libp2pendpoint

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/events/planner"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	stringid "github.com/plprobelab/go-kademlia/network/address/stringid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/ipfsv1"
	"github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/stretchr/testify/require"
)

var (
	peerstoreTTL = 10 * time.Minute
	protoID      = address.ProtocolID("/test/1.0.0")
)

func createEndpoints(t *testing.T, ctx context.Context, nPeers int) (
	[]*Libp2pEndpoint, []*addrinfo.AddrInfo, []*peerid.PeerID,
	[]scheduler.AwareScheduler,
) {
	clk := clock.New()

	scheds := make([]scheduler.AwareScheduler, nPeers)
	ids := make([]*peerid.PeerID, nPeers)
	addrinfos := make([]*addrinfo.AddrInfo, nPeers)
	endpoints := make([]*Libp2pEndpoint, nPeers)
	for i := 0; i < nPeers; i++ {
		scheds[i] = simplescheduler.NewSimpleScheduler(clk)
		host, err := libp2p.New()
		require.NoError(t, err)
		ids[i] = peerid.NewPeerID(host.ID())
		addrinfos[i] = addrinfo.NewAddrInfo(peer.AddrInfo{
			ID:    ids[i].ID,
			Addrs: host.Addrs(),
		})
		endpoints[i] = NewLibp2pEndpoint(ctx, host, scheds[i])
		na, err := endpoints[i].NetworkAddress(ids[i])
		require.NoError(t, err)
		for _, a := range na.Addresses() {
			ma, ok := a.(ma.Multiaddr)
			require.True(t, ok)
			require.Contains(t, host.Addrs(), ma)
		}
	}
	return endpoints, addrinfos, ids, scheds
}

func connectEndpoints(t *testing.T, ctx context.Context, endpoints []*Libp2pEndpoint,
	addrInfos []*addrinfo.AddrInfo,
) {
	require.Len(t, endpoints, len(addrInfos))
	for i, ep := range endpoints {
		for j, ai := range addrInfos {
			if i == j {
				continue
			}
			err := ep.MaybeAddToPeerstore(ctx, ai, peerstoreTTL)
			require.NoError(t, err)
		}
	}
}

func TestConnections(t *testing.T) {
	ctx := context.Background()

	// create endpoints
	endpoints, addrs, ids, _ := createEndpoints(t, ctx, 4)

	invalidID := stringid.NewStringID("invalid")

	// test that the endpoint's kademlia key is as expected
	for i, ep := range endpoints {
		require.Equal(t, ids[i].Key(), ep.KadKey())
	}

	// add peer 1 to peer 0's peerstore
	err := endpoints[0].MaybeAddToPeerstore(ctx, addrs[1], peerstoreTTL)
	require.NoError(t, err)
	// add invalid peer to peerstore
	err = endpoints[0].MaybeAddToPeerstore(ctx, invalidID, peerstoreTTL)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	// should add no information for self
	err = endpoints[0].MaybeAddToPeerstore(ctx, addrs[0], peerstoreTTL)
	require.NoError(t, err)
	// add peer 1 to peer 0's peerstore
	err = endpoints[0].MaybeAddToPeerstore(ctx, addrs[3], peerstoreTTL)
	require.NoError(t, err)

	// test connectedness, not connected but known address -> NotConnected
	connectedness, err := endpoints[0].Connectedness(ids[1])
	require.NoError(t, err)
	require.Equal(t, network.NotConnected, connectedness)
	// not connected, unknown address -> NotConnected
	connectedness, err = endpoints[0].Connectedness(ids[2])
	require.NoError(t, err)
	require.Equal(t, network.NotConnected, connectedness)
	// invalid peerid -> error
	connectedness, err = endpoints[1].Connectedness(invalidID)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	require.Equal(t, network.NotConnected, connectedness)
	// verify peerinfo for invalid peerid
	peerinfo, err := endpoints[1].PeerInfo(invalidID)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	require.Len(t, peerinfo.Addrs, 0)
	// verify network address for valid peerid
	netAddr, err := endpoints[0].NetworkAddress(ids[1])
	require.NoError(t, err)
	ai, ok := netAddr.(*addrinfo.AddrInfo)
	require.True(t, ok)
	require.Equal(t, ids[1].ID, ai.PeerID().ID)
	require.Len(t, ai.Addrs, len(addrs[1].Addrs))
	for _, addr := range ai.Addrs {
		require.Contains(t, addrs[1].Addrs, addr)
	}
	// verify network address for invalid peerid
	netAddr, err = endpoints[0].NetworkAddress(invalidID)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	require.Nil(t, netAddr)
	// dial from 0 to 1
	err = endpoints[0].DialPeer(ctx, ids[1])
	require.NoError(t, err)
	// test connectedness
	connectedness, err = endpoints[0].Connectedness(ids[1])
	require.NoError(t, err)
	require.Equal(t, network.Connected, connectedness)
	// test peerinfo
	peerinfo, err = endpoints[0].PeerInfo(ids[1])
	require.NoError(t, err)
	require.Len(t, peerinfo.Addrs, len(addrs[1].Addrs))
	for _, addr := range peerinfo.Addrs {
		require.Contains(t, addrs[1].Addrs, addr)
	}
	peerinfo, err = endpoints[0].PeerInfo(ids[2])
	require.NoError(t, err)
	require.Len(t, peerinfo.Addrs, 0)

	// dial from 1 to invalid peerid
	err = endpoints[1].DialPeer(ctx, invalidID)
	require.Error(t, err)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	// dial again from 0 to 1, no is already connected
	err = endpoints[0].DialPeer(ctx, ids[1])
	require.NoError(t, err)
	// dial from 1 to 0, works because 1 is already connected to 0
	err = endpoints[1].DialPeer(ctx, ids[0])
	require.NoError(t, err)
	// dial from 0 to 2, fails because 0 doesn't know 2's addresses
	err = endpoints[0].DialPeer(ctx, ids[2])
	require.Error(t, err)
}

func TestAsyncDial(t *testing.T) {
	ctx := context.Background()
	wg := sync.WaitGroup{}

	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 4)

	// test async dial and report from 0 to 1
	err := endpoints[0].MaybeAddToPeerstore(ctx, addrs[1], peerstoreTTL)
	require.NoError(t, err)
	wg.Add(1)
	err = endpoints[0].AsyncDialAndReport(ctx, ids[1], func(ctx context.Context, success bool) {
		require.True(t, success)
		wg.Done()
	})
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		// AsyncDialAndReport adds the dial action to the event queue, so we
		// need to run the scheduler
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		wg.Done()
	}()
	wg.Wait()
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}

	// test async dial and report from 0 to 2 (already connected)
	err = endpoints[0].MaybeAddToPeerstore(ctx, addrs[2], peerstoreTTL)
	require.NoError(t, err)
	err = endpoints[0].DialPeer(ctx, ids[2])
	require.NoError(t, err)
	err = endpoints[0].AsyncDialAndReport(ctx, ids[1], func(ctx context.Context, success bool) {
		require.True(t, success)
	})
	require.NoError(t, err)
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}

	// test async dial and report from 0 to 3 (unknown address)
	wg.Add(1)
	endpoints[0].AsyncDialAndReport(ctx, ids[3], func(ctx context.Context, success bool) {
		require.False(t, success)
		wg.Done()
	})
	wg.Add(1)
	go func() {
		// AsyncDialAndReport adds the dial action to the event queue, so we
		// need to run the scheduler
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		wg.Done()
	}()
	wg.Wait()
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, s.NextActionTime(ctx), planner.MaxTime)
	}

	// test asyc dial with invalid peerid
	err = endpoints[0].AsyncDialAndReport(ctx, stringid.NewStringID("invalid"), nil)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}
}

func TestRequestHandler(t *testing.T) {
	ctx := context.Background()
	endpoints, _, _, _ := createEndpoints(t, ctx, 1)

	// set node 1 to server mode
	requestHandler := func(ctx context.Context, id address.NodeID[key.Key256],
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		// request handler returning the received message
		return req, nil
	}
	err := endpoints[0].AddRequestHandler(protoID, &ipfsv1.Message{}, requestHandler)
	require.NoError(t, err)
	err = endpoints[0].AddRequestHandler(protoID, &ipfsv1.Message{}, nil)
	require.Equal(t, endpoint.ErrNilRequestHandler, err)

	// invalid message format for handler
	err = endpoints[0].AddRequestHandler("/fail/1.0.0", &simmessage.SimMessage[key.Key256]{}, requestHandler)
	require.Equal(t, ErrRequireProtoKadMessage, err)

	// remove request handler
	endpoints[0].RemoveRequestHandler(protoID)
}

func TestReqFailFast(t *testing.T) {
	ctx := context.Background()

	endpoints, addrs, ids, _ := createEndpoints(t, ctx, 3)
	connectEndpoints(t, ctx, endpoints[:2], addrs[:2])

	req := ipfsv1.FindPeerRequest(ids[1])
	// invalid response format (not protobuf)
	err := endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&simmessage.SimMessage[key.Key256]{}, time.Second, nil)
	require.Equal(t, ErrRequireProtoKadResponse, err)

	// invalid request format (not protobuf)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1],
		&simmessage.SimMessage[key.Key256]{}, &ipfsv1.Message{}, time.Second, nil)
	require.Equal(t, ErrRequireProtoKadMessage, err)

	// invalid recipient (not a peerid)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID,
		stringid.NewStringID("invalid"), req, &ipfsv1.Message{}, time.Second, nil)
	require.Equal(t, ErrRequirePeerID, err)

	// nil response handler
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&ipfsv1.Message{}, time.Second, nil)
	require.Equal(t, endpoint.ErrNilResponseHandler, err)

	// non nil response handler that should not be called
	responseHandler := func(_ context.Context, _ message.MinKadResponseMessage[key.Key256], err error) {
		require.Fail(t, "response handler shouldn't be called")
	}

	// peer 0 isn't connected to peer 2, so it should fail fast
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[2], req,
		&ipfsv1.Message{}, time.Second, responseHandler)
	require.Equal(t, endpoint.ErrUnknownPeer, err)
}

func TestSuccessfulRequest(t *testing.T) {
	ctx := context.Background()

	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 2)
	connectEndpoints(t, ctx, endpoints, addrs)

	// set node 1 to server mode
	requestHandler := func(ctx context.Context, id address.NodeID[key.Key256],
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		// request handler returning the received message
		return req, nil
	}
	err := endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, requestHandler)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	responseHandler := func(ctx context.Context,
		resp message.MinKadResponseMessage[key.Key256], err error,
	) {
		wg.Done()
	}
	req := ipfsv1.FindPeerRequest(ids[1])
	wg.Add(1)
	endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&ipfsv1.Message{}, time.Second, responseHandler)
	wg.Add(2)
	go func() {
		// run server 1
		for !scheds[1].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[1].RunOne(ctx)) // only 1 action should run on server
		wg.Done()
	}()
	go func() {
		// timeout is queued in the scheduler 0
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[0].RunOne(ctx))
		wg.Done()
	}()
	wg.Wait()
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}
}

func TestReqUnknownPeer(t *testing.T) {
	ctx := context.Background()

	// create endpoints
	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 2)
	// replace address of peer 1 with an invalid address
	addrs[1] = addrinfo.NewAddrInfo(peer.AddrInfo{
		ID:    addrs[1].ID,
		Addrs: []ma.Multiaddr{ma.StringCast("/ip4/1.2.3.4")},
	})
	// "connect" the endpoints. 0 will store an invalid address for 1, so it
	// won't fail fast, because it believes that it knows how to reach 1
	connectEndpoints(t, ctx, endpoints, addrs)

	// verfiy that the wrong address is stored for peer 1 in peer 0
	na, err := endpoints[0].NetworkAddress(ids[1])
	require.NoError(t, err)
	for _, addr := range na.Addresses() {
		a, ok := addr.(ma.Multiaddr)
		require.True(t, ok)
		require.Contains(t, addrs[1].Addrs, a)
	}

	req := ipfsv1.FindPeerRequest(ids[1])
	wg := sync.WaitGroup{}
	responseHandler := func(_ context.Context, _ message.MinKadResponseMessage[key.Key256], err error) {
		wg.Done()
		require.Equal(t, swarm.ErrNoGoodAddresses, err)
	}

	// unknown valid peerid (address not stored in peerstore)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&ipfsv1.Message{}, time.Second, responseHandler)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		// timeout is queued in the scheduler 0
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[0].RunOne(ctx))
		wg.Done()
	}()
	wg.Wait()
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}
}

func TestReqTimeout(t *testing.T) {
	ctx := context.Background()

	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 2)
	connectEndpoints(t, ctx, endpoints, addrs)

	req := ipfsv1.FindPeerRequest(ids[1])

	wg := sync.WaitGroup{}

	err := endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, func(ctx context.Context,
		id address.NodeID[key.Key256], req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		return req, nil
	})
	require.NoError(t, err)

	responseHandler := func(_ context.Context, _ message.MinKadResponseMessage[key.Key256], err error) {
		require.Error(t, err)
		wg.Done()
	}
	// timeout after 100 ms, will fail immediately
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&ipfsv1.Message{}, 100*time.Millisecond, responseHandler)
	require.NoError(t, err)
	wg.Add(2)
	go func() {
		// timeout is queued in the scheduler 0
		for !scheds[0].RunOne(ctx) {
			time.Sleep(1 * time.Millisecond)
		}
		require.False(t, scheds[0].RunOne(ctx))
		wg.Done()
	}()
	wg.Wait()
	// now that the client has timed out, the server will send back the reply
	require.True(t, scheds[1].RunOne(ctx))

	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}
}

func TestReqHandlerError(t *testing.T) {
	// server request handler error
	ctx := context.Background()

	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 2)
	connectEndpoints(t, ctx, endpoints, addrs)

	req := ipfsv1.FindPeerRequest(ids[1])

	wg := sync.WaitGroup{}

	err := endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, func(ctx context.Context,
		id address.NodeID[key.Key256], req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		// request handler returns error
		return nil, errors.New("server error")
	})
	require.NoError(t, err)
	// responseHandler is run after context is cancelled
	responseHandler := func(ctx context.Context,
		resp message.MinKadResponseMessage[key.Key256], err error,
	) {
		wg.Done()
		require.Error(t, err)
	}
	noResponseCtx, cancel := context.WithCancel(ctx)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(noResponseCtx, protoID, ids[1], req,
		&ipfsv1.Message{}, 0, responseHandler)
	require.NoError(t, err)
	wg.Add(2)
	go func() {
		for !scheds[1].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[1].RunOne(ctx))
		cancel()
		wg.Done()
	}()
	go func() {
		// timeout is queued in the scheduler 0
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[0].RunOne(ctx))
		wg.Done()
	}()
	wg.Wait()
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}
}

func TestReqHandlerReturnsWrongType(t *testing.T) {
	// server request handler returns wrong message type, no response is
	// returned by the server
	ctx := context.Background()

	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 2)
	connectEndpoints(t, ctx, endpoints, addrs)

	req := ipfsv1.FindPeerRequest(ids[1])

	wg := sync.WaitGroup{}
	err := endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, func(ctx context.Context,
		id address.NodeID[key.Key256], req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		// request handler returns error
		return &simmessage.SimMessage[key.Key256]{}, nil
	})
	require.NoError(t, err)
	// responseHandler is run after context is cancelled
	responseHandler := func(ctx context.Context,
		resp message.MinKadResponseMessage[key.Key256], err error,
	) {
		wg.Done()
		require.Error(t, err)
	}
	noResponseCtx, cancel := context.WithCancel(ctx)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(noResponseCtx, protoID, ids[1], req,
		&ipfsv1.Message{}, 0, responseHandler)
	require.NoError(t, err)
	wg.Add(2)
	go func() {
		for !scheds[1].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[1].RunOne(ctx))
		cancel()
		wg.Done()
	}()
	go func() {
		// timeout is queued in the scheduler 0
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
		require.False(t, scheds[0].RunOne(ctx))
		wg.Done()
	}()
	wg.Wait()
	// nothing to run for both schedulers
	for _, s := range scheds {
		require.Equal(t, planner.MaxTime, s.NextActionTime(ctx))
	}
}
