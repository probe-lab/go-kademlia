package sim

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/plprobelab/go-kademlia/kad"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/libp2p"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing/simplert"
)

var (
	_ endpoint.NetworkedEndpoint[key.Key8, net.IP] = (*Endpoint[key.Key8, net.IP])(nil)
	_ endpoint.SimEndpoint[key.Key8, net.IP]       = (*Endpoint[key.Key8, net.IP])(nil)
	_ endpoint.Endpoint[key.Key8, net.IP]          = (*Endpoint[key.Key8, net.IP])(nil)
)

var (
	protoID      = address.ProtocolID("/test/1.0.0")
	peerstoreTTL = time.Minute
)

func TestEndpoint(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	selfID := kadtest.StringID("self")

	router := NewRouter[key.Key256, net.IP]()
	sched := simplescheduler.NewSimpleScheduler(clk)

	fakeEndpoint := NewEndpoint[key.Key256, net.IP](selfID.NodeID(), sched, router)

	b := key.Equal(selfID.Key(), fakeEndpoint.KadKey())
	require.True(t, b)

	node0 := kadtest.StringID("node0")
	err := fakeEndpoint.DialPeer(ctx, node0)
	require.Equal(t, endpoint.ErrUnknownPeer, err)

	connectedness, err := fakeEndpoint.Connectedness(node0)
	require.NoError(t, err)
	require.Equal(t, network.NotConnected, connectedness)

	na, err := fakeEndpoint.NetworkAddress(node0)
	require.NoError(t, err)
	require.Equal(t, na, node0)

	parsed, err := peer.Decode("1EooooPEER")
	require.NoError(t, err)
	pid := libp2p.NewPeerID(parsed)
	_, err = fakeEndpoint.NetworkAddress(pid)
	require.Equal(t, endpoint.ErrUnknownPeer, err)

	req := NewRequest[key.Key256, net.IP](selfID.Key())
	resp := &Message[key.Key256, net.IP]{}

	var runCheck bool
	respHandler := func(ctx context.Context, msg message.MinKadResponseMessage[key.Key256, net.IP], err error) {
		require.Equal(t, endpoint.ErrUnknownPeer, err)
		runCheck = true
	}
	fakeEndpoint.SendRequestHandleResponse(ctx, protoID, node0, req, resp, 0, respHandler)
	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
	require.True(t, runCheck)

	err = fakeEndpoint.MaybeAddToPeerstore(ctx, kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(node0.Key()), nil), peerstoreTTL)
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
	fakeEndpoint0 := NewEndpoint[key.Key256, net.IP](node0, sched0, router)
	rt0 := simplert.New(node0.Key(), 2)
	serv0 := NewServer[key.Key256, net.IP](rt0, fakeEndpoint0, DefaultServerConfig())
	err = fakeEndpoint0.AddRequestHandler(protoID, nil, serv0.HandleRequest)
	require.NoError(t, err)
	err = fakeEndpoint0.AddRequestHandler(protoID, nil, nil)
	require.Equal(t, endpoint.ErrNilRequestHandler, err)
	// remove a request handler that doesn't exist
	fakeEndpoint0.RemoveRequestHandler("/test/0.0.1")

	runCheck = false
	respHandler = func(ctx context.Context, msg message.MinKadResponseMessage[key.Key256, net.IP], err error) {
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
		msg message.MinKadResponseMessage[key.Key256, net.IP], err error,
	) {
		require.Equal(t, ErrInvalidResponseType, err)
	}
	var msg message.MinKadMessage
	fakeEndpoint.HandleMessage(ctx, node0, protoID, sid, msg)

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))

	// test request whose handler returns an error
	errHandler := func(ctx context.Context, id kad.NodeID[key.Key256],
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
	followup := func(ctx context.Context, resp message.MinKadResponseMessage[key.Key256, net.IP],
		err error,
	) {
		require.NoError(t, err)
		followupRan = true
	}
	// add followup function for the stream and make sure it runs
	fakeEndpoint.streamFollowup[1000] = followup
	addrs := []kad.NodeInfo[key.Key256, net.IP]{
		kadtest.NewInfo(kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0})), []net.IP{net.ParseIP("127.0.0.1")}),
		kadtest.NewInfo(kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{1})), []net.IP{net.ParseIP("127.0.0.2")}),
		kadtest.NewInfo(kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{2})), []net.IP{net.ParseIP("127.0.0.3")}),
	}
	msg = NewResponse(addrs)
	fakeEndpoint.HandleMessage(ctx, node0, protoID, 1000, msg)

	a, err := fakeEndpoint.NetworkAddress(addrs[0].ID())
	require.NoError(t, err)
	require.Equal(t, addrs[0], a)

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
	require.True(t, followupRan)
}

func TestRequestTimeout(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	router := NewRouter[key.Key256, net.IP]()

	nPeers := 2
	scheds := make([]scheduler.AwareScheduler, nPeers)
	ids := make([]kad.NodeInfo[key.Key256, net.IP], nPeers)
	fakeEndpoints := make([]*Endpoint[key.Key256, net.IP], nPeers)
	for i := 0; i < nPeers; i++ {
		ids[i] = kadtest.NewInfo[key.Key256, net.IP](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{byte(i)})), nil)
		scheds[i] = simplescheduler.NewSimpleScheduler(clk)
		fakeEndpoints[i] = NewEndpoint[key.Key256, net.IP](ids[i].ID(), scheds[i], router)
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
	dropRequestHandler := func(ctx context.Context, id kad.NodeID[key.Key256],
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		return nil, endpoint.ErrUnknownPeer
	}
	fakeEndpoints[1].AddRequestHandler(protoID, nil, dropRequestHandler)
	// fakeEndpoints[0] will send a request to fakeEndpoints[1], but the request
	// will timeout (because fakeEndpoints[1] will not respond)
	fakeEndpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1].ID(), nil, nil,
		time.Second, func(ctx context.Context,
			msg message.MinKadResponseMessage[key.Key256, net.IP], err error,
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
	fakeEndpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1].ID(), nil, nil,
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
	dumbResponseHandler := func(ctx context.Context, id kad.NodeID[key.Key256],
		req message.MinKadMessage,
	) (message.MinKadMessage, error) {
		clk.Sleep(100 * time.Millisecond)
		return req, nil
	}
	// create valid message
	msg := NewResponse([]kad.NodeInfo[key.Key256, net.IP]{})
	// overwrite request handler
	fakeEndpoints[1].AddRequestHandler(protoID, nil, dumbResponseHandler)
	fakeEndpoints[0].SendRequestHandleResponse(ctx, protoID, ids[1].ID(), msg, nil,
		time.Second, func(ctx context.Context,
			msg message.MinKadResponseMessage[key.Key256, net.IP], err error,
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
