package fakeendpoint

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadaddr"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	si "github.com/plprobelab/go-kademlia/network/address/stringid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/server/basicserver"
	"github.com/stretchr/testify/require"
)

var (
	protoID      = address.ProtocolID("/test/1.0.0")
	peerstoreTTL = time.Minute
)

func TestFakeEndpoint(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	selfID := si.StringID("self")

	router := NewFakeRouter()
	sched := simplescheduler.NewSimpleScheduler(clk)

	fakeEndpoint := NewFakeEndpoint(selfID, sched, router)

	b, err := selfID.Key().Equal(fakeEndpoint.KadKey())
	require.NoError(t, err)
	require.True(t, b)

	node0 := si.StringID("node0")
	err = fakeEndpoint.DialPeer(ctx, node0)
	require.Equal(t, endpoint.ErrUnknownPeer, err)

	connectedness, err := fakeEndpoint.Connectedness(node0)
	require.NoError(t, err)
	require.Equal(t, network.NotConnected, connectedness)

	na, err := fakeEndpoint.NetworkAddress(node0)
	require.NoError(t, err)
	require.Equal(t, na, node0)

	parsed, err := peer.Decode("1EooooPEER")
	require.NoError(t, err)
	pid := peerid.NewPeerID(parsed)
	_, err = fakeEndpoint.NetworkAddress(pid)
	require.Equal(t, endpoint.ErrUnknownPeer, err)

	req := simmessage.NewSimRequest(selfID.Key())
	resp := &simmessage.SimMessage{}

	var runCheck bool
	respHandler := func(ctx context.Context, msg message.MinKadResponseMessage, err error) {
		require.Equal(t, endpoint.ErrUnknownPeer, err)
		runCheck = true
	}
	fakeEndpoint.SendRequestHandleResponse(ctx, protoID, node0, req, resp, 0, respHandler)
	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
	require.True(t, runCheck)

	err = fakeEndpoint.MaybeAddToPeerstore(ctx, node0, peerstoreTTL)
	require.NoError(t, err)

	connectedness, err = fakeEndpoint.Connectedness(node0)
	require.NoError(t, err)
	require.Equal(t, network.CanConnect, connectedness)

	na, err = fakeEndpoint.NetworkAddress(node0)
	require.NoError(t, err)
	require.Equal(t, node0, na)

	// it will still be an ErrUnknownPeer because we haven't added node0 to the router
	err = fakeEndpoint.SendRequestHandleResponse(ctx, protoID, node0, req, resp, 0, respHandler)
	require.NoError(t, err)
	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))

	sched0 := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint0 := NewFakeEndpoint(node0, sched0, router)
	rt0 := simplert.New(node0.Key(), 2)
	serv0 := basicserver.NewBasicServer(rt0, fakeEndpoint0)
	err = fakeEndpoint0.AddRequestHandler(protoID, nil, serv0.HandleRequest)
	require.NoError(t, err)
	err = fakeEndpoint0.AddRequestHandler(protoID, nil, nil)
	require.Equal(t, endpoint.ErrNilRequestHandler, err)
	// remove a request handler that doesn't exist
	fakeEndpoint0.RemoveRequestHandler("/test/0.0.1")

	runCheck = false
	respHandler = func(ctx context.Context, msg message.MinKadResponseMessage, err error) {
		require.NoError(t, err)
		runCheck = true
	}

	err = fakeEndpoint.SendRequestHandleResponse(ctx, protoID, node0, req, resp, 0, respHandler)
	require.NoError(t, err)

	require.True(t, sched0.RunOne(ctx))
	require.False(t, sched0.RunOne(ctx))

	require.False(t, runCheck)

	require.True(t, sched.RunOne(ctx))
	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))

	require.True(t, runCheck)

	// test response to a sent request that is not a valid MinKadResponseMessage
	var sid endpoint.StreamID = 1000
	fakeEndpoint.streamFollowup[sid] = func(ctx context.Context,
		msg message.MinKadResponseMessage, err error,
	) {
		require.Equal(t, ErrInvalidResponseType, err)
	}
	var msg message.MinKadMessage
	fakeEndpoint.HandleMessage(ctx, node0, protoID, sid, msg)

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))

	// test request whose handler returns an error
	errHandler := func(ctx context.Context, id address.NodeID,
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		return nil, endpoint.ErrUnknownPeer
	}
	errProtoID := address.ProtocolID("/err/0.0.1")
	fakeEndpoint.AddRequestHandler(errProtoID, nil, errHandler)
	fakeEndpoint.HandleMessage(ctx, node0, errProtoID, 1001, msg)
	// no message should have been sent to node0, so nothing to run
	require.False(t, sched0.RunOne(ctx))

	var followupRan bool
	// test that HandleMessage adds the closer peers to the peerstore
	followup := func(ctx context.Context, resp message.MinKadResponseMessage,
		err error) {
		require.NoError(t, err)
		followupRan = true
	}
	// add followup function for the stream and make sure it runs
	fakeEndpoint.streamFollowup[1000] = followup
	addrs := []address.NodeAddr{
		kadaddr.NewKadAddr(kadid.NewKadID([]byte{0}), []string{"0"}),
		kadaddr.NewKadAddr(kadid.NewKadID([]byte{1}), []string{"1"}),
		kadaddr.NewKadAddr(kadid.NewKadID([]byte{2}), []string{"2"}),
	}
	msg = simmessage.NewSimResponse(addrs)
	fakeEndpoint.HandleMessage(ctx, node0, protoID, 1000, msg)

	a, err := fakeEndpoint.NetworkAddress(addrs[0].NodeID())
	require.NoError(t, err)
	require.Equal(t, addrs[0], a)

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
	require.True(t, followupRan)
}

func TestRequestTimeout(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	router := NewFakeRouter()

	nPeers := 2
	scheds := make([]scheduler.AwareScheduler, nPeers)
	ids := make([]address.NodeAddr, nPeers)
	fakeEndpoints := make([]*FakeEndpoint, nPeers)
	for i := 0; i < nPeers; i++ {
		ids[i] = kadid.NewKadID([]byte{byte(i)})
		scheds[i] = simplescheduler.NewSimpleScheduler(clk)
		fakeEndpoints[i] = NewFakeEndpoint(ids[i].NodeID(), scheds[i], router)
	}

	// connect the peers to each other
	for i, fe := range fakeEndpoints {
		for j := range fakeEndpoints {
			if i != j {
				fe.MaybeAddToPeerstore(ctx, ids[j], peerstoreTTL)
			}
		}
	}

	var timeoutExecuted bool
	// fakeEndpoints[1]'s request handler will not respond
	dropRequestHandler := func(ctx context.Context, id address.NodeID,
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		return nil, endpoint.ErrUnknownPeer
	}
	fakeEndpoints[1].AddRequestHandler(protoID, nil, dropRequestHandler)
	// fakeEndpoints[0] will send a request to fakeEndpoints[1], but the request
	// will timeout (because fakeEndpoints[1] will not respond)
	fakeEndpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1].NodeID(), nil, nil,
		time.Second, func(ctx context.Context,
			msg message.MinKadResponseMessage, err error,
		) {
			timeoutExecuted = true
		})

	// peer[1] has 1 action to run: message handling and not responding
	require.True(t, scheds[1].RunOne(ctx))
	require.False(t, scheds[1].RunOne(ctx))
	// peer[0] has no action to run: waiting for response
	require.False(t, scheds[0].RunOne(ctx))

	// advance the clock to timeout
	clk.Add(time.Second)

	// peer[0] is running timeout action
	require.True(t, scheds[0].RunOne(ctx))
	require.False(t, scheds[0].RunOne(ctx))
	require.True(t, timeoutExecuted)

	// timeout without followup action
	fakeEndpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1].NodeID(), nil, nil,
		time.Second, nil)
	// peer[1] has 1 action to run: message handling and not responding
	require.True(t, scheds[1].RunOne(ctx))
	require.False(t, scheds[1].RunOne(ctx))
	// peer[0] has no action to run: waiting for response
	require.False(t, scheds[0].RunOne(ctx))
	// advance the clock to timeout
	clk.Add(time.Second)
	// peer[0] is running timeout action
	require.True(t, scheds[0].RunOne(ctx))
	require.False(t, scheds[0].RunOne(ctx))

	// response coming back before timeout
	var handlerHasResponded bool
	dumbResponseHandler := func(ctx context.Context, id address.NodeID,
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		clk.Sleep(100 * time.Millisecond)
		return req, nil
	}
	// create valid message
	msg := simmessage.NewSimResponse([]address.NodeAddr{})
	// overwrite request handler
	fakeEndpoints[1].AddRequestHandler(protoID, nil, dumbResponseHandler)
	fakeEndpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1].NodeID(), msg, nil,
		time.Second, func(ctx context.Context,
			msg message.MinKadResponseMessage, err error,
		) {
			require.NoError(t, err)
		})

	go func() {
		for !handlerHasResponded {
			clk.Add(100 * time.Millisecond)
		}
	}()
	// peer[0] is waiting for response
	require.False(t, scheds[0].RunOne(ctx))
	// peer[1] has 1 action to run: message handling and responding
	require.True(t, scheds[1].RunOne(ctx))
	require.False(t, scheds[1].RunOne(ctx))
	// peer[0] is running followup action
	require.True(t, scheds[0].RunOne(ctx))
	require.True(t, scheds[0].RunOne(ctx))
	require.False(t, scheds[0].RunOne(ctx))
}
