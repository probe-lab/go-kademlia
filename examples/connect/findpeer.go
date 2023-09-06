package main

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/plprobelab/go-libdht/event"
	tutil "github.com/plprobelab/go-libdht/examples/util"
	"github.com/plprobelab/go-libdht/kad"
	"github.com/plprobelab/go-libdht/key"
	"github.com/plprobelab/go-libdht/libp2p"
	"github.com/plprobelab/go-libdht/network/address"
	"github.com/plprobelab/go-libdht/query/simplequery"
	"github.com/plprobelab/go-libdht/routing/simplert"
	"github.com/plprobelab/go-libdht/util"
)

var protocolID address.ProtocolID = "/ipfs/kad/1.0.0" // IPFS DHT network protocol ID

func FindPeer(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "FindPeer Test")
	defer span.End()

	// this example is using real time
	clk := clock.New()

	// create a libp2p host
	h, err := tutil.Libp2pHost(ctx, "8888")
	if err != nil {
		panic(err)
	}

	pid := libp2p.NewPeerID(h.ID())

	// create a simple routing table, with bucket size 20
	rt := simplert.New[key.Key256, kad.NodeID[key.Key256]](pid, 20)
	// create a scheduler using real time
	sched := event.NewSimpleScheduler(clk)
	// create a message endpoint is used to communicate with other peers
	msgEndpoint := libp2p.NewLibp2pEndpoint(ctx, h, sched)

	// friend is the first peer we know in the IPFS DHT network (bootstrap node)
	friend, err := peer.Decode("12D3KooWGjgvfDkpuVAoNhd7PRRvMTEG4ZgzHBFURqDe1mqEzAMS")
	if err != nil {
		panic(err)
	}
	friendID := libp2p.NewPeerID(friend)

	// multiaddress of friend
	a, err := multiaddr.NewMultiaddr("/ip4/45.32.75.236/udp/4001/quic")
	if err != nil {
		panic(err)
	}
	// connect to friend
	friendAddr := peer.AddrInfo{ID: friend, Addrs: []multiaddr.Multiaddr{a}}
	if err := h.Connect(ctx, friendAddr); err != nil {
		panic(err)
	}
	fmt.Println("connected to friend")

	// target is the peer we want to find QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb
	target, err := peer.Decode("QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb")
	if err != nil {
		panic(err)
	}
	targetID := libp2p.NewPeerID(target)

	// create a find peer request message
	req := libp2p.FindPeerRequest(targetID)
	// add friend to routing table
	success := rt.AddNode(friendID)
	if !success {
		panic("failed to add friend to rt")
	}

	// endCond is used to terminate the simulation once the query is done
	endCond := false
	handleResultsFn := func(ctx context.Context, id kad.NodeID[key.Key256],
		resp kad.Response[key.Key256, multiaddr.Multiaddr],
	) (bool, []kad.NodeID[key.Key256]) {
		// parse response to ipfs dht message
		msg, ok := resp.(*libp2p.Message)
		if !ok {
			fmt.Println("invalid response!")
			return false, nil
		}
		var targetAddrs *libp2p.AddrInfo
		peers := make([]kad.NodeID[key.Key256], 0, len(msg.CloserPeers))
		for _, p := range msg.CloserPeers {
			addrInfo, err := libp2p.PBPeerToPeerInfo(p)
			if err != nil {
				fmt.Println("invalid peer info format")
				continue
			}
			peers = append(peers, addrInfo.PeerID())
			if addrInfo.PeerID().ID == target {
				endCond = true
				targetAddrs = addrInfo
			}
		}
		fmt.Println("---\nResponse from", id, "with", peers)
		if endCond {
			fmt.Println("\n  - target found!", target, targetAddrs.Addrs)
		}
		// return peers and not msg.CloserPeers because we want to return the
		// PeerIDs and not AddrInfos. The returned NodeID is used to update the
		// query. The AddrInfo is only useful for the message endpoint.
		return endCond, peers
	}

	// create the query, the IPFS DHT protocol ID, the IPFS DHT request message,
	// a concurrency parameter of 1, a timeout of 5 seconds, the libp2p message
	// endpoint, the node's routing table and scheduler, and the response
	// handler function.
	// The query will be executed only once actions are run on the scheduler.
	// For now, it is only scheduled to be run.
	queryOpts := []simplequery.Option[key.Key256, multiaddr.Multiaddr]{
		simplequery.WithProtocolID[key.Key256, multiaddr.Multiaddr](protocolID),
		simplequery.WithConcurrency[key.Key256, multiaddr.Multiaddr](1),
		simplequery.WithRequestTimeout[key.Key256, multiaddr.Multiaddr](2 * time.Second),
		simplequery.WithHandleResultsFunc[key.Key256, multiaddr.Multiaddr](handleResultsFn),
		simplequery.WithRoutingTable[key.Key256, multiaddr.Multiaddr](rt),
		simplequery.WithEndpoint[key.Key256, multiaddr.Multiaddr](msgEndpoint),
		simplequery.WithScheduler[key.Key256, multiaddr.Multiaddr](sched),
	}
	_, err = simplequery.NewSimpleQuery[key.Key256, multiaddr.Multiaddr](ctx, pid.NodeID(), req, queryOpts...)
	if err != nil {
		panic(err)
	}

	span.AddEvent("start request execution")

	// run the actions from the scheduler until the query is done
	for i := 0; i < 1000 && !endCond; i++ {
		for sched.RunOne(ctx) {
		}
		time.Sleep(10 * time.Millisecond)
	}
}
