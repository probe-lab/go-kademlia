package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/plprobelab/go-kademlia/coord"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	tutil "github.com/plprobelab/go-kademlia/examples/util"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/libp2p"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/util"
)

var protocolID address.ProtocolID = "/ipfs/kad/1.0.0" // IPFS DHT network protocol ID

func FindPeer(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "FindPeer Test")
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	sched := simplescheduler.NewSimpleScheduler(clk)
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

	// target is the peer we want to find 12D3KooWK5eSiSMwvQk6v96JfXbuTFhuYYoCJaUUGXqJWj47HQVU
	target, err := peer.Decode("12D3KooWK5eSiSMwvQk6v96JfXbuTFhuYYoCJaUUGXqJWj47HQVU")
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

	cfg := coord.DefaultConfig()
	cfg.RequestConcurrency = 1
	cfg.RequestTimeout = 5 * time.Second

	c, err := coord.NewCoordinator[key.Key256, multiaddr.Multiaddr](pid, msgEndpoint, rt, cfg)
	if err != nil {
		log.Fatal(err)
	}

	span.AddEvent("start request execution")

	queryID := query.QueryID("query1")

	err = c.StartQuery(ctx, queryID, protocolID, req)
	if err != nil {
		log.Fatalf("failed to start query: %v", err)
	}

	// run the coordinator and endpoint scheduler until the query is done
	go func(ctx context.Context) {
		stepper := clk.Ticker(10 * time.Millisecond)
		defer stepper.Stop()
		for {
			select {
			case <-stepper.C:
				sched.RunOne(ctx)
				c.RunOne(ctx)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Fatalf("context cancelled: %v", ctx.Err())
		case ev := <-c.Events():
			// read an event from the coordinator

			switch tev := ev.(type) {
			case *coord.KademliaOutboundQueryProgressedEvent[key.Key256, multiaddr.Multiaddr]:
				fmt.Printf("query %q progressed\n", tev.QueryID)
				fmt.Printf("  running: %s\n", clk.Since(tev.Stats.Start))
				fmt.Printf("  requests made: %d\n", tev.Stats.Requests)
				fmt.Printf("  requests succeeded: %d\n", tev.Stats.Success)
				fmt.Printf("  requests failed: %d\n", tev.Stats.Failure)

				for _, found := range tev.Response.CloserNodes() {
					if key.Equal(found.ID().Key(), targetID.Key()) {
						fmt.Printf("found the node we were looking for: %v\n", found.ID())
						fmt.Println("stopping query")
						c.StopQuery(ctx, queryID)
					}
				}

			case *coord.KademliaOutboundQueryFinishedEvent:
				fmt.Printf("query %q finished\n", tev.QueryID)
				fmt.Printf("  duration: %s\n", tev.Stats.End.Sub(tev.Stats.Start))
				fmt.Printf("  requests made: %d\n", tev.Stats.Requests)
				fmt.Printf("  requests succeeded: %d\n", tev.Stats.Success)
				fmt.Printf("  requests failed: %d\n", tev.Stats.Failure)
				return
			case *coord.KademliaRoutingUpdatedEvent[key.Key256, multiaddr.Multiaddr]:
				fmt.Printf("routing updated for node %v\n", tev.NodeInfo.ID())
			default:
				fmt.Printf("got event: %#v\n", ev)
			}

		}
	}
}
