package main

import (
	"context"
	"fmt"
	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/examples/util"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/endpoint/libp2pendpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/ipfsv1"
	"github.com/plprobelab/go-kademlia/query/simplequery"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"time"
)

func main() {
	ctx := context.Background()

	// friend is the first peer we know in the IPFS DHT network (bootstrap node)
	friend, err := peer.Decode("12D3KooWGjgvfDkpuVAoNhd7PRRvMTEG4ZgzHBFURqDe1mqEzAMS")
	if err != nil {
		panic(err)
	}
	friendID := peerid.NewPeerID(friend)

	// multiaddress of friend
	a, err := multiaddr.NewMultiaddr("/ip4/45.32.75.236/udp/4001/quic")
	if err != nil {
		panic(err)
	}
	// connect to friend

	// target is the peer we want to find QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb
	target, err := peer.Decode("12D3KooWK5eSiSMwvQk6v96JfXbuTFhuYYoCJaUUGXqJWj47HQVU")
	if err != nil {
		panic(err)
	}
	targetID := peerid.NewPeerID(target)

	h, err := util.Libp2pHost(ctx, "3234")
	if err != nil {
		panic(err)
	}

	friendAddr := peer.AddrInfo{ID: friend, Addrs: []multiaddr.Multiaddr{a}}
	if err := h.Connect(ctx, friendAddr); err != nil {
		panic(err)
	}

	fmt.Println("Connected")
	dht := New(ctx, h)

	fmt.Println("Add Peer")
	success, err := dht.rt.AddPeer(ctx, friendID)
	if err != nil {
		panic(err)
	}
	fmt.Println("Added peer", success)

	fmt.Println("Find Peers")
	go func() {
		p, err := dht.FindPeers(ctx, targetID)
		if err != nil {
			panic(err)
		}
		fmt.Println("FOUND", p)
	}()

	// run the actions from the scheduler until the query is done
	for i := 0; i < 1000; i++ {
		for dht.scheduler.RunOne(ctx) {
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type DHT struct {
	ep        endpoint.Endpoint
	scheduler *simplescheduler.SimpleScheduler
	rt        *simplert.SimpleRT
	h         host.Host
}

func New(ctx context.Context, h host.Host) *DHT {
	pid := peerid.NewPeerID(h.ID())

	sched := simplescheduler.NewSimpleScheduler(clock.New())
	ep := libp2pendpoint.NewLibp2pEndpoint(ctx, h, sched)
	rt := simplert.New(pid.Key(), 20)
	d := &DHT{
		h:         h,
		rt:        rt,
		ep:        ep,
		scheduler: sched,
	}

	return d
}

func (d *DHT) FindPeers(ctx context.Context, pid *peerid.PeerID) (*peer.AddrInfo, error) {
	results := make(chan *peer.AddrInfo)

	handleResultsFn := func(ctx context.Context, id address.NodeID, resp message.MinKadResponseMessage) (bool, []address.NodeID) {

		fmt.Println("handleResults")

		// parse response to ipfs dht message
		msg, ok := resp.(*ipfsv1.Message)
		if !ok {
			fmt.Println("invalid response!")
			return false, nil
		}

		peers := make([]address.NodeID, 0, len(msg.CloserPeers))
		for _, p := range msg.CloserPeers {
			addrInfo, err := ipfsv1.PBPeerToPeerInfo(p)
			if err != nil {
				fmt.Println("invalid peer info format")
				continue
			}
			peers = append(peers, addrInfo.PeerID())
			if addrInfo.PeerID().ID == pid.ID {
				results <- &addrInfo.AddrInfo
				return true, peers
			}
		}

		return true, peers
	}

	opts := []simplequery.Option{
		simplequery.WithEndpoint(d.ep),
		simplequery.WithScheduler(d.scheduler),
		simplequery.WithRoutingTable(d.rt),
		simplequery.WithProtocolID("/ipfs/kad/1.0.0"),
		simplequery.WithHandleResultsFunc(handleResultsFn),
	}

	req := ipfsv1.FindPeerRequest(pid)
	_, err := simplequery.NewSimpleQuery(ctx, req, opts...)
	if err != nil {
		return nil, err
	}

	return <-results, nil
}
