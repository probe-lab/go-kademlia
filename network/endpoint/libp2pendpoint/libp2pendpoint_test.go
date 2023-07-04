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
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
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

func TestLibp2pEndpoint(t *testing.T) {
	ctx := context.Background()
	clk := clock.New()

	// create endpoints
	nPeers := 4
	scheds := make([]scheduler.Scheduler, nPeers)
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
	}

	invalidID := stringid.NewStringID("invalid")

	// test that the endpoint's kademlia key is as expected
	for i, ep := range endpoints {
		require.Equal(t, ids[i].Key(), ep.KadKey())
	}

	// add peer 1 to peer 0's peerstore
	err := endpoints[0].MaybeAddToPeerstore(ctx, addrinfos[1], peerstoreTTL)
	require.NoError(t, err)
	// add invalid peer to peerstore
	err = endpoints[0].MaybeAddToPeerstore(ctx, invalidID, peerstoreTTL)
	require.Equal(t, endpoint.ErrInvalidPeer, err)
	// should add no information for self
	err = endpoints[0].MaybeAddToPeerstore(ctx, addrinfos[0], peerstoreTTL)
	require.NoError(t, err)
	// add peer 1 to peer 0's peerstore
	err = endpoints[0].MaybeAddToPeerstore(ctx, addrinfos[3], peerstoreTTL)
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
	require.Len(t, ai.Addrs, len(addrinfos[1].Addrs))
	for _, addr := range ai.Addrs {
		require.Contains(t, addrinfos[1].Addrs, addr)
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
	require.Len(t, peerinfo.Addrs, len(addrinfos[1].Addrs))
	for _, addr := range peerinfo.Addrs {
		require.Contains(t, addrinfos[1].Addrs, addr)
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

	wg := sync.WaitGroup{}
	// test async dial and report from 0 to 3
	wg.Add(1)
	err = endpoints[0].AsyncDialAndReport(ctx, ids[3], func(ctx context.Context, success bool) {
		require.True(t, success)
		wg.Done()
	})
	require.NoError(t, err)
	go func() {
		// AsyncDialAndReport adds the dial action to the event queue, so we
		// need to run the scheduler
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()
	// test async dial and report from 0 to 1 (already connected)
	err = endpoints[0].AsyncDialAndReport(ctx, ids[1], func(ctx context.Context, success bool) {
		require.True(t, success)
	})
	require.NoError(t, err)
	// test async dial and report from 0 to 2 (unknown address)
	wg.Add(1)
	endpoints[0].AsyncDialAndReport(ctx, ids[2], func(ctx context.Context, success bool) {
		require.False(t, success)
		wg.Done()
	})
	go func() {
		// AsyncDialAndReport adds the dial action to the event queue, so we
		// need to run the scheduler
		for !scheds[0].RunOne(ctx) {
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()
	// test asyc dial with invalid peerid
	err = endpoints[0].AsyncDialAndReport(ctx, invalidID, nil)
	require.Equal(t, endpoint.ErrInvalidPeer, err)

	// set node 1 to server mode
	requestHandler := func(ctx context.Context, id address.NodeID,
		req message.MinKadMessage) (message.MinKadMessage, error) {
		// request handler returning the received message
		return req, nil
	}
	err = endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, requestHandler)
	require.NoError(t, err)
	err = endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, nil)
	require.Equal(t, endpoint.ErrNilRequestHandler, err)

	// send request from 0 to 1
	wg.Add(1)
	responseHandler := func(ctx context.Context,
		resp message.MinKadResponseMessage, err error) {
		wg.Done()
	}
	req := ipfsv1.FindPeerRequest(ids[1])
	resp := &ipfsv1.Message{}
	endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req, resp,
		time.Second, responseHandler)

	// run the schedulers
	for !scheds[1].RunOne(ctx) {
		time.Sleep(time.Millisecond)
	}
	require.False(t, scheds[1].RunOne(ctx)) // only 1 action should run on server
	wg.Wait()

	// invalid response format (not protobuf)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		&simmessage.SimMessage{}, time.Second, nil)
	require.Equal(t, ErrRequireProtoKadResponse, err)

	// invalid request format (not protobuf)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1],
		&simmessage.SimMessage{}, resp, time.Second, nil)
	require.Equal(t, ErrRequireProtoKadMessage, err)

	// invalid recipient (not a peerid)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, invalidID, req,
		resp, time.Second, nil)
	require.Equal(t, ErrRequirePeerID, err)

	// nil response handler
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		resp, time.Second, nil)
	require.Equal(t, endpoint.ErrNilResponseHandler, err)

	// builds response handler checking that the response handler error
	// matches the expected error
	invalidRespHandlerBuilder := func(err0 error) func(context.Context,
		message.MinKadResponseMessage, error) {
		return func(ctx context.Context, resp message.MinKadResponseMessage,
			err1 error) {
			wg.Done()
			require.Equal(t, err0, err1, err0)
		}
	}

	// unknown valid peerid (address not stored in peerstore)
	wg.Add(1)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[2], req,
		resp, time.Second, invalidRespHandlerBuilder(swarm.ErrNoAddresses))
	require.NoError(t, err)
	wg.Wait()

	// test timeout
	err = endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, func(ctx context.Context,
		id address.NodeID, req message.MinKadMessage) (message.MinKadMessage, error) {
		// request handler wait 2ms before returning the received message
		time.Sleep(10 * time.Millisecond)
		return req, nil
	})
	require.NoError(t, err)
	wg.Add(1)
	// timeout after 1 ms
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		resp, time.Millisecond, invalidRespHandlerBuilder(endpoint.ErrTimeout))
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
	// response arrives after timeout
	require.True(t, scheds[1].RunOne(ctx))
	require.False(t, scheds[1].RunOne(ctx))

	// server request handler error
	err = endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, func(ctx context.Context,
		id address.NodeID, req message.MinKadMessage) (message.MinKadMessage, error) {
		// request handler returns error
		return nil, errors.New("server error")
	})
	require.NoError(t, err)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		resp, 0, responseHandler)
	require.NoError(t, err)
	wg.Add(1)
	for !scheds[1].RunOne(ctx) {
		time.Sleep(time.Millisecond)
	}
	require.False(t, scheds[1].RunOne(ctx))
	// no response to be handled by 0
	require.False(t, scheds[0].RunOne(ctx))

	// server request handler returns wrong message type
	err = endpoints[1].AddRequestHandler(protoID, &ipfsv1.Message{}, func(ctx context.Context,
		id address.NodeID, req message.MinKadMessage) (message.MinKadMessage, error) {
		// request handler returns error
		return &simmessage.SimMessage{}, nil
	})
	require.NoError(t, err)
	err = endpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1], req,
		resp, 0, responseHandler)
	require.NoError(t, err)
	wg.Add(1)
	for !scheds[1].RunOne(ctx) {
		time.Sleep(time.Millisecond)
	}
	require.False(t, scheds[1].RunOne(ctx))
	// no response to be handled by 0
	require.False(t, scheds[0].RunOne(ctx))

	// invalid message format for handler
	err = endpoints[0].AddRequestHandler("/fail/1.0.0", &simmessage.SimMessage{}, requestHandler)
	require.Equal(t, ErrRequireProtoKadMessage, err)

	// remove request handler
	endpoints[1].RemoveRequestHandler(protoID)
}
