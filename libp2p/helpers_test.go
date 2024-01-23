package libp2p

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
	"github.com/probe-lab/go-kademlia/sim"
)

var (
	_ ProtoKadRequestMessage[key.Key256, multiaddr.Multiaddr]  = (*Message)(nil)
	_ ProtoKadResponseMessage[key.Key256, multiaddr.Multiaddr] = (*Message)(nil)
)

var testPeerstoreTTL = 10 * time.Minute

func TestFindPeerRequest(t *testing.T) {
	p, err := peer.Decode("12D3KooWH6Qd1EW75ANiCtYfD51D6M7MiZwLQ4g8wEBpoEUnVYNz")
	require.NoError(t, err)

	pid := NewPeerID(p)
	msg := FindPeerRequest(pid)

	require.Equal(t, msg.GetKey(), []byte(p))

	b := key.Equal(msg.Target(), pid.Key())
	require.True(t, b)

	require.Equal(t, 0, len(msg.CloserNodes()))
}

func createDummyPeerInfo(id, addr string) (*AddrInfo, error) {
	p, err := peer.Decode(id)
	if err != nil {
		return nil, err
	}
	a, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	return NewAddrInfo(peer.AddrInfo{
		ID:    p,
		Addrs: []multiaddr.Multiaddr{a},
	}), nil
}

func TestFindPeerResponse(t *testing.T) {
	ctx := context.Background()
	selfAddr, err := createDummyPeerInfo("12BoooooSELF", "/ip4/1.1.1.1")
	require.NoError(t, err)

	fakeEndpoint := sim.NewEndpoint[key.Key256, multiaddr.Multiaddr](selfAddr, nil, nil)

	nPeers := 5
	closerPeers := make([]kad.NodeInfo[key.Key256, multiaddr.Multiaddr], nPeers)
	closerIds := make([]kad.NodeID[key.Key256], nPeers)
	for i := 0; i < nPeers; i++ {
		s := strconv.Itoa(2 + i)
		closerPeers[i], err = createDummyPeerInfo("12BooooPEER"+s, "/ip4/"+s+"."+s+"."+s+"."+s)
		require.NoError(t, err)

		closerIds[i] = closerPeers[i].(*AddrInfo).PeerID()
		fakeEndpoint.MaybeAddToPeerstore(ctx, closerPeers[i], testPeerstoreTTL)
	}

	resp := FindPeerResponse(closerIds, fakeEndpoint)

	// require.Nil(t, resp.Target())
	require.Equal(t, closerPeers, resp.CloserNodes())
}

func TestCornerCases(t *testing.T) {
	resp := FindPeerResponse(nil, nil)
	// require.Nil(t, resp.Target())
	require.Equal(t, 0, len(resp.CloserNodes()))

	require.Equal(t, &Message{}, resp.EmptyResponse())

	ids := make([]kad.NodeID[key.Key256], 0)
	resp = FindPeerResponse(ids, nil)

	// require.Nil(t, resp.Target())
	require.Equal(t, 0, len(resp.CloserNodes()))

	fakeEndpoint := sim.NewEndpoint[key.Key256, multiaddr.Multiaddr](AddrInfo{}, nil, nil)
	n0, err := peer.Decode("1D3oooUnknownPeer")
	require.NoError(t, err)
	ids = append(ids, &PeerID{ID: n0})

	resp = FindPeerResponse(ids, fakeEndpoint)
	require.Equal(t, 0, len(resp.CloserNodes()))

	pbp := Message_Peer{
		Id:         []byte(n0),
		Addrs:      [][]byte{},
		Connection: 0,
	}

	ai, err := PBPeerToPeerInfo(&pbp)
	require.Equal(t, err, ErrNoValidAddresses)
	require.Nil(t, ai)
}
