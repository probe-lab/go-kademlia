package main

import (
	"context"
	"fmt"
	"net"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type IpfsDht struct {
	kad          *KademliaHandler[key.Key256, net.IP]
	queryWaiters map[QueryID]chan<- kad.MinKadResponseMessage[key.Key256, net.IP]
}

func NewIpfsDht(kh *KademliaHandler[key.Key256, net.IP]) *IpfsDht {
	return &IpfsDht{
		kad:          kh,
		queryWaiters: make(map[QueryID]chan<- kad.MinKadResponseMessage[key.Key256, net.IP]),
	}
}

func (d *IpfsDht) Start(ctx context.Context) {
	go d.mainloop(ctx)
}

func (d *IpfsDht) mainloop(ctx context.Context) {
	kadEvents := d.kad.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-kadEvents:
			switch tev := ev.(type) {
			case *KademliaOutboundQueryProgressedEvent[key.Key256, net.IP]:
				// TODO: locking
				ch, ok := d.queryWaiters[tev.QueryID]
				if !ok {
					// we have lost the query waiter somehow
					d.kad.StopQuery(ctx, tev.QueryID)
					continue
				}

				// notify the waiter
				ch <- tev.Response

			default:
				panic(fmt.Sprintf("unexpected event: %T", tev))
			}
		}
	}
}

func (d *IpfsDht) registerQueryWaiter(queryID QueryID, ch chan<- kad.MinKadResponseMessage[key.Key256, net.IP]) {
	// TODO: locking
	d.queryWaiters[queryID] = ch
}

// Initiates an iterative query for the the address of the given peer.
// FindNode is a fundamental Kademlia operation so this logic should be on KademliaHandler
func (d *IpfsDht) FindNode(ctx context.Context, node kad.NodeID[key.Key256]) (kad.NodeInfo[key.Key256, net.IP], error) {
	trace("IpfsHandler.FindNode")
	// TODO: look in local peer store first

	// If not in peer store then query the Kademlia dht
	queryID, err := d.kad.StartQuery(ctx, &FindNodeRequest[key.Key256, net.IP]{NodeID: node})
	if err != nil {
		return nil, fmt.Errorf("failed to start query: %w", err)
	}
	trace("Query id is %d", queryID)

	ch := make(chan kad.MinKadResponseMessage[key.Key256, net.IP])
	d.registerQueryWaiter(queryID, ch)

	// wait for query to finish
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp, ok := <-ch:
			if !ok {
				// channel was closed, so query can't progress
				d.kad.StopQuery(ctx, queryID)
				return nil, fmt.Errorf("query was unexpectedly stopped")
			}
			trace("IpfsHandler.FindNode: got event from kademlia")
			// we got a response from a message sent by query
			switch tresp := resp.(type) {
			case *FindNodeResponse[key.Key256, net.IP]:
				// interpret the response
				for _, found := range tresp.CloserPeers {
					// TODO: is this the best way to test for node equality?
					if key.Equal(found.ID().Key(), node.Key()) {
						// found the node we were looking for
						d.kad.StopQuery(ctx, queryID)
						return found, nil
					}
				}
				trace("IpfsHandler.FindNode: desired node not found yet")
			default:
				return nil, fmt.Errorf("unknown response: %v", resp)
			}
		}
	}
}

// Initiates an iterative query for the closest peers to the given key.
// TODO: function signature
func (*IpfsDht) ClosestPeers(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}

// Performs a lookup for a record in the DHT.
// TODO: function signature
func (*IpfsDht) GetRecord(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}

// Stores a record in the DHT, locally as well as at the nodes
// closest to the key as per the xor distance metric.
// TODO: function signature
func (*IpfsDht) PutRecord(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}

// Stores a record at specific peers, without storing it locally.
// TODO: function signature
func (*IpfsDht) PutRecordTo(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}

// Removes the record with the given key from _local_ storage,
// if the local node is the publisher of the record.
// TODO: function signature
func (*IpfsDht) RemoveRecord(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}

// Bootstraps the local node to join the DHT.
// TODO: function signature
func (*IpfsDht) Bootstrap(ctx context.Context) {
	panic("not implemented")
}

// Establishes the local node as a provider of a value for the given key.
// TODO: function signature
func (*IpfsDht) StartProviding(ctx context.Context) {
	panic("not implemented")
}

// Stops the local node from announcing that it is a provider for the given key.
// TODO: function signature
func (*IpfsDht) StopProviding(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}

// Performs a lookup for providers of a value to the given key.
// TODO: function signature
func (*IpfsDht) GetProviders(ctx context.Context, kk key.Key256) {
	panic("not implemented")
}
