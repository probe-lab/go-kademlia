package basicserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler/simplescheduler"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/kadid"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint/fakeendpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p-kad-dht/network/message/simmessage"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable/simplert"
	"github.com/stretchr/testify/require"
)

var self = kadid.KadID{KadKey: []byte{0x00}} // 0000 0000

// remotePeers with bucket assignments wrt to self
var remotePeers = []*kadid.KadID{
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

func TestKadSimServer(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint
	numberOfCloserPeersToSend := 4

	router := fakeendpoint.NewFakeRouter()
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := fakeendpoint.NewFakeEndpoint(self, sched, router)
	rt := simplert.NewSimpleRT(self.Key(), 2)

	// add peers to routing table and peerstore
	for _, p := range remotePeers {
		err := fakeEndpoint.MaybeAddToPeerstore(ctx, p, peerstoreTTL)
		require.NoError(t, err)
		success, err := rt.AddPeer(ctx, p)
		require.NoError(t, err)
		require.True(t, success)
	}

	s0 := NewBasicServer(rt, fakeEndpoint, WithPeerstoreTTL(peerstoreTTL),
		WithNumberOfCloserPeersToSend(numberOfCloserPeersToSend))

	requester := kadid.KadID{KadKey: []byte{0b00000001}} // 0000 0001

	req0 := simmessage.NewSimRequest([]byte{0b00000000})
	msg, err := s0.HandleFindNodeRequest(ctx, requester, req0)
	require.NoError(t, err)

	resp, ok := msg.(message.MinKadResponseMessage)
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0000 0000
	// [8] 0000 1000, [7] 0001 0001, [6] 0001 1111, [4] 0010 1110
	order := []*kadid.KadID{remotePeers[8], remotePeers[7], remotePeers[6], remotePeers[4]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	req1 := simmessage.NewSimRequest([]byte{0b11111111})
	msg, err = s0.HandleFindNodeRequest(ctx, requester, req1)
	require.NoError(t, err)
	resp, ok = msg.(message.MinKadResponseMessage)
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 1111 1111
	// [1] 1101 0010, [0] 1000 1000, [3] 0101 0011, [2] 0100 1011
	order = []*kadid.KadID{remotePeers[1], remotePeers[0], remotePeers[3], remotePeers[2]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

	numberOfCloserPeersToSend = 3
	s1 := NewBasicServer(rt, fakeEndpoint, WithNumberOfCloserPeersToSend(3))

	req2 := simmessage.NewSimRequest([]byte{0b01100000})
	msg, err = s1.HandleFindNodeRequest(ctx, requester, req2)
	require.NoError(t, err)
	resp, ok = msg.(message.MinKadResponseMessage)
	require.True(t, ok)
	require.Len(t, resp.CloserNodes(), numberOfCloserPeersToSend)
	// closer peers should be ordered by distance to 0110 0000
	// [2] 0100 1011, [3] 0101 0011, [4] 0010 1110
	order = []*kadid.KadID{remotePeers[2], remotePeers[3], remotePeers[4]}
	for i, p := range resp.CloserNodes() {
		require.Equal(t, order[i], p)
	}

}

func TestInvalidRequests(t *testing.T) {
	ctx := context.Background()
	// invalid option
	s := NewBasicServer(nil, nil, func(*Config) error {
		return errors.New("invalid option")
	})
	require.Nil(t, s)

	peerstoreTTL := time.Second // doesn't matter as we use fakeendpoint

	clk := clock.New()
	router := fakeendpoint.NewFakeRouter()

	// create a valid server
	sched := simplescheduler.NewSimpleScheduler(clk)
	fakeEndpoint := fakeendpoint.NewFakeEndpoint(self, sched, router)
	rt := simplert.NewSimpleRT(self.Key(), 2)

	// add peers to routing table and peerstore
	for _, p := range remotePeers {
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
