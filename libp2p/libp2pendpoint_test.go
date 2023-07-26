package libp2p

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/plprobelab/go-kademlia/sim"

	"github.com/plprobelab/go-kademlia/kad"

	"github.com/plprobelab/go-kademlia/internal/kadtest"

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
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/stretchr/testify/require"
)

var (
	peerstoreTTL = 10 * time.Minute
	protoID      = address.ProtocolID("/test/1.0.0")
)

func createEndpoints(t *testing.T, ctx context.Context, nPeers int) (
	[]*Libp2pEndpoint, []*AddrInfo, []*PeerID,
	[]scheduler.AwareScheduler,
) {
	clk := clock.New()

	scheds := make([]scheduler.AwareScheduler, nPeers)
	ids := make([]*PeerID, nPeers)
	addrinfos := make([]*AddrInfo, nPeers)
	endpoints := make([]*Libp2pEndpoint, nPeers)
	for i := 0; i < nPeers; i++ {
		scheds[i] = simplescheduler.NewSimpleScheduler(clk)
		host, err := libp2p.New()
		require.NoError(t, err)
		ids[i] = NewPeerID(host.ID())
		addrinfos[i] = NewAddrInfo(peer.AddrInfo{
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
	addrInfos []*AddrInfo,
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

	invalidID := kadtest.NewInfo[key.Key256, ma.Multiaddr](kadtest.NewID(kadtest.NewStringID("invalid").Key()), nil)

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
	connectedness, err = endpoints[1].Connectedness(invalidID.ID())
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	require.Equal(t, network.NotConnected, connectedness)
	// verify peerinfo for invalid peerid
	peerinfo, err := endpoints[1].PeerInfo(invalidID.ID())
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	require.Len(t, peerinfo.Addrs, 0)
	// verify network address for valid peerid
	netAddr, err := endpoints[0].NetworkAddress(ids[1])
	require.NoError(t, err)
	ai, ok := netAddr.(*AddrInfo)
	require.True(t, ok)
	require.Equal(t, ids[1].ID, ai.PeerID().ID)
	require.Len(t, ai.Addrs, len(addrs[1].Addrs))
	for _, addr := range ai.Addrs {
		require.Contains(t, addrs[1].Addrs, addr)
	}
	// verify network address for invalid peerid
	netAddr, err = endpoints[0].NetworkAddress(invalidID.ID())
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
	err = endpoints[1].DialPeer(ctx, invalidID.ID())
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
	err = endpoints[0].AsyncDialAndReport(ctx, kadtest.NewStringID("invalid"), nil)
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
	requestHandler := func(ctx context.Context, id kad.NodeID[key.Key256],
		req kad.MinKadMessage,
	) (kad.MinKadMessage, error) {
		// request handler returning the received message
		return req, nil
	}
	err := endpoints[0].AddRequestHandler(protoID, &Message{}, requestHandler)
	require.NoError(t, err)
	err = endpoints[0].AddRequestHandler(protoID, &Message{}, nil)
	require.Equal(t, endpoint.ErrNilRequestHandler, err)

	// invalid message format for handler
	err = endpoints[0].AddRequestHandler("/fail/1.0.0", &sim.SimMessage[key.Key256, ma.Multiaddr]{}, requestHandler)
	require.Equal(t, ErrRequireProtoKadMessage, err)

	// remove request handler
	endpoints[0].RemoveRequestHandler(protoID)
}

func TestReqFailFast(t *testing.T) {
	ctx := context.Background()

	endpoints, addrs, ids, _ := createEndpoints(t, ctx, 3)
	connectEndpoints(t, ctx, endpoints[:2], addrs[:2])

	req := FindPeerRequest(ids[1])
	// invalid response format (not protobuf)
	err := endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&sim.SimMessage[key.Key256, ma.Multiaddr]{}, time.Second, nil)
	require.Equal(t, ErrRequireProtoKadResponse, err)

	// invalid request format (not protobuf)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1],
		&sim.SimMessage[key.Key256, ma.Multiaddr]{}, &Message{}, time.Second, nil)
	require.Equal(t, ErrRequireProtoKadMessage, err)

	// invalid recipient (not a peerid)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID,
		kadtest.NewStringID("invalid"), req, &Message{}, time.Second, nil)
	require.Equal(t, ErrRequirePeerID, err)

	// nil response handler
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&Message{}, time.Second, nil)
	require.Equal(t, endpoint.ErrNilResponseHandler, err)

	// non nil response handler that should not be called
	responseHandler := func(_ context.Context, _ kad.MinKadResponseMessage[key.Key256, ma.Multiaddr], err error) {
		require.Fail(t, "response handler shouldn't be called")
	}

	// peer 0 isn't connected to peer 2, so it should fail fast
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[2], req,
		&Message{}, time.Second, responseHandler)
	require.Equal(t, endpoint.ErrUnknownPeer, err)
}

func TestSuccessfulRequest(t *testing.T) {
	ctx := context.Background()

	endpoints, addrs, ids, scheds := createEndpoints(t, ctx, 2)
	connectEndpoints(t, ctx, endpoints, addrs)

	// set node 1 to server mode
	requestHandler := func(ctx context.Context, id kad.NodeID[key.Key256],
		req kad.MinKadMessage,
	) (kad.MinKadMessage, error) {
		// request handler returning the received message
		return req, nil
	}
	err := endpoints[1].AddRequestHandler(protoID, &Message{}, requestHandler)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	responseHandler := func(ctx context.Context,
		resp kad.MinKadResponseMessage[key.Key256, ma.Multiaddr], err error,
	) {
		wg.Done()
	}
	req := FindPeerRequest(ids[1])
	wg.Add(1)
	endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&Message{}, time.Second, responseHandler)
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
	addrs[1] = NewAddrInfo(peer.AddrInfo{
		ID:    addrs[1].PeerID().ID,
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

	req := FindPeerRequest(ids[1])
	wg := sync.WaitGroup{}
	responseHandler := func(_ context.Context, _ kad.MinKadResponseMessage[key.Key256, ma.Multiaddr], err error) {
		wg.Done()
		require.Equal(t, swarm.ErrNoGoodAddresses, err)
	}

	// unknown valid peerid (address not stored in peerstore)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&Message{}, time.Second, responseHandler)
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

	req := FindPeerRequest(ids[1])

	wg := sync.WaitGroup{}

	err := endpoints[1].AddRequestHandler(protoID, &Message{}, func(ctx context.Context,
		id kad.NodeID[key.Key256], req kad.MinKadMessage,
	) (kad.MinKadMessage, error) {
		return req, nil
	})
	require.NoError(t, err)

	responseHandler := func(_ context.Context, _ kad.MinKadResponseMessage[key.Key256, ma.Multiaddr], err error) {
		require.Error(t, err)
		wg.Done()
	}
	// timeout after 100 ms, will fail immediately
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&Message{}, 100*time.Millisecond, responseHandler)
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

	req := FindPeerRequest(ids[1])

	wg := sync.WaitGroup{}

	err := endpoints[1].AddRequestHandler(protoID, &Message{}, func(ctx context.Context,
		id kad.NodeID[key.Key256], req kad.MinKadMessage,
	) (kad.MinKadMessage, error) {
		// request handler returns error
		return nil, errors.New("server error")
	})
	require.NoError(t, err)
	// responseHandler is run after context is cancelled
	responseHandler := func(ctx context.Context,
		resp kad.MinKadResponseMessage[key.Key256, ma.Multiaddr], err error,
	) {
		wg.Done()
		require.Error(t, err)
	}
	noResponseCtx, cancel := context.WithCancel(ctx)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(noResponseCtx, protoID, ids[1], req,
		&Message{}, 0, responseHandler)
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

	req := FindPeerRequest(ids[1])

	wg := sync.WaitGroup{}
	err := endpoints[1].AddRequestHandler(protoID, &Message{}, func(ctx context.Context,
		id kad.NodeID[key.Key256], req kad.MinKadMessage,
	) (kad.MinKadMessage, error) {
		// request handler returns error
		return &sim.SimMessage[key.Key256, ma.Multiaddr]{}, nil
	})
	require.NoError(t, err)
	// responseHandler is run after context is cancelled
	responseHandler := func(ctx context.Context,
		resp kad.MinKadResponseMessage[key.Key256, ma.Multiaddr], err error,
	) {
		wg.Done()
		require.Error(t, err)
	}
	noResponseCtx, cancel := context.WithCancel(ctx)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(noResponseCtx, protoID, ids[1], req,
		&Message{}, 0, responseHandler)
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
