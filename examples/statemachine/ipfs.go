package main

import (
	"context"
	"net"

	"github.com/plprobelab/go-kademlia/coord"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/util"
)

type IpfsDht struct {
	coordinator  *coord.Coordinator[key.Key256, net.IP]
	queryWaiters map[query.QueryID]chan<- kad.Response[key.Key256, net.IP]
}

func NewIpfsDht(c *coord.Coordinator[key.Key256, net.IP]) *IpfsDht {
	return &IpfsDht{
		coordinator:  c,
		queryWaiters: make(map[query.QueryID]chan<- kad.Response[key.Key256, net.IP]),
	}
}

func (d *IpfsDht) Start(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "IpfsDht.Start")
	defer span.End()
	go d.mainloop(ctx)
}

func (d *IpfsDht) mainloop(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "IpfsDht.mainloop")
	defer span.End()

	kadEvents := d.coordinator.Start(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-kadEvents:
			switch tev := ev.(type) {
			case *coord.KademliaOutboundQueryProgressedEvent[key.Key256, net.IP]:
				// TODO: locking
				ch, ok := d.queryWaiters[tev.QueryID]
				if !ok {
					// we have lost the query waiter somehow
					d.coordinator.StopQuery(ctx, tev.QueryID)
					continue
				}

				// notify the waiter
				ch <- tev.Response
			}
		}
	}
}

func (d *IpfsDht) registerQueryWaiter(queryID query.QueryID, ch chan<- kad.Response[key.Key256, net.IP]) {
	// TODO: locking
	d.queryWaiters[queryID] = ch
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
