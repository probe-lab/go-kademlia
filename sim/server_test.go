package sim

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing/simplert"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
)

// remotePeers with bucket assignments wrt to self
var kadRemotePeers = []kad.NodeInfo[key.Key8, any]{
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b10001000)), nil), // 1000 1000 (bucket 0)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b11010010)), nil), // 1101 0010 (bucket 0)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b01001011)), nil), // 0100 1011 (bucket 1)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b01010011)), nil), // 0101 0011 (bucket 1)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b00101110)), nil), // 0010 1110 (bucket 2)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b00110110)), nil), // 0011 0110 (bucket 2)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b00011111)), nil), // 0001 1111 (bucket 3)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b00010001)), nil), // 0001 0001 (bucket 3)
	kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b00001000)), nil), // 0000 1000 (bucket 4)
}

func TestMessageHandling(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second
	numberOfCloserPeersToSend := 4

	self := kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0)), nil) // 0000 0000

	router := NewRouter[key.Key8, any]()
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := NewEndpoint[key.Key8, any](self.ID(), sched, router)
	rt := simplert.New(self.ID().Key(), 2)

	// add peers to routing table and peerstore
	for _, p := range kadRemotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddNode(ctx, p.ID())
		require.NoError(t, err)
		require.True(t, success)
	}

	s0 := NewServer[key.Key8, any](rt, fakeEndpoint, &ServerConfig{
		PeerstoreTTL:            peerstoreTTL,
		NumberUsefulCloserPeers: numberOfCloserPeersToSend,
	})

	requester := kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0b00000001)), nil) // 0000 0001
	fakeEndpoint.MaybeAddToPeerstore(ctx, requester, peerstoreTTL)

	req0 := NewRequest[key.Key8, any](key.Key8(0b00000000))
	msg, err := s0.HandleRequest(ctx, requester.ID(), req0)
	require.NoError(t, err)

	resp, ok := msg.(message.MinKadResponseMessage[key.Key8, any])
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0000 0000
	// [8] 0000 1000, [7] 0001 0001, [6] 0001 1111, [4] 0010 1110
	order := []kad.NodeInfo[key.Key8, any]{
		kadRemotePeers[8], kadRemotePeers[7],
		kadRemotePeers[6], kadRemotePeers[4],
	}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	req1 := NewRequest[key.Key8, any](key.Key8(0b11111111))
	msg, err = s0.HandleRequest(ctx, requester.ID(), req1)
	require.NoError(t, err)
	resp, ok = msg.(message.MinKadResponseMessage[key.Key8, any])
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 1111 1111
	// [1] 1101 0010, [0] 1000 1000, [3] 0101 0011, [2] 0100 1011
	order = []kad.NodeInfo[key.Key8, any]{
		kadRemotePeers[1], kadRemotePeers[0],
		kadRemotePeers[3], kadRemotePeers[2],
	}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	numberOfCloserPeersToSend = 3
	s1 := NewServer[key.Key8, any](rt, fakeEndpoint, &ServerConfig{
		PeerstoreTTL:            peerstoreTTL,
		NumberUsefulCloserPeers: numberOfCloserPeersToSend,
	})

	req2 := NewRequest[key.Key8, any](key.Key8(0b01100000))
	msg, err = s1.HandleRequest(ctx, requester.ID(), req2)
	require.NoError(t, err)
	resp, ok = msg.(message.MinKadResponseMessage[key.Key8, any])
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0110 0000
	// [2] 0100 1011, [3] 0101 0011, [4] 0010 1110
	order = []kad.NodeInfo[key.Key8, any]{kadRemotePeers[2], kadRemotePeers[3], kadRemotePeers[4]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}
}

func TestInvalidSimRequests(t *testing.T) {
	ctx := context.Background()
	// invalid option
	s := (*Server[key.Key8, any])(nil)
	require.Nil(t, s)

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint

	clk := clock.New()
	router := NewRouter[key.Key8, any]()

	self := kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0)), nil) // 0000 0000

	// create a valid server
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := NewEndpoint[key.Key8, any](self.ID(), sched, router)
	rt := simplert.New(self.ID().Key(), 2)

	// add peers to routing table and peerstore
	for _, p := range kadRemotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddNode(ctx, p.ID())
		require.NoError(t, err)
		require.True(t, success)
	}

	s = NewServer[key.Key8, any](rt, fakeEndpoint, DefaultServerConfig())
	require.NotNil(t, s)

	requester := kadtest.NewID(key.Key8(0b00000001)) // 0000 0001

	// invalid message format (not a SimMessage)
	req0 := struct{}{}
	_, err := s.HandleFindNodeRequest(ctx, requester, req0)
	require.Error(t, err)

	// empty request
	req1 := &Message[key.Key8, any]{}
	s.HandleFindNodeRequest(ctx, requester, req1)

	// request with invalid key (not matching the expected length)
	req2 := NewRequest[key.Key32, any](key.Key32(0b00000000000000010000000000000000))
	s.HandleFindNodeRequest(ctx, requester, req2)
}

func TestRequestNoNetworkAddress(t *testing.T) {
	ctx := context.Background()

	clk := clock.New()
	router := NewRouter[key.Key8, any]()

	self := kadtest.NewInfo[key.Key8, any](kadtest.NewID(key.Key8(0)), nil) // 0000 0000

	// create a valid server
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := NewEndpoint[key.Key8, any](self.ID(), sched, router)
	rt := simplert.New(self.ID().Key(), 2)

	node := kadtest.NewID(key.Key8(0xf6))

	// add peer to routing table, but NOT to peerstore
	success, err := rt.AddNode(ctx, node)
	require.NoError(t, err)
	require.True(t, success)

	s := NewServer[key.Key8, any](rt, fakeEndpoint, DefaultServerConfig())
	require.NotNil(t, s)

	requester := kadtest.NewID(key.Key8(0x80))

	// sim request message (for any key)
	req := NewRequest[key.Key8, any](requester.Key())
	msg, err := s.HandleFindNodeRequest(ctx, requester, req)
	require.NoError(t, err)
	resp, ok := msg.(message.MinKadResponseMessage[key.Key8, any])
	require.True(t, ok)
	fmt.Println(resp.CloserNodes())
	require.Len(t, resp.CloserNodes(), 0)
}
