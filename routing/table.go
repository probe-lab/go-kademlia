package routing

import (
	"context"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

// Table is the interface for Kademlia Routing Tables
type Table interface {
	// Self returns the local node's Kademlia key
	Self() key.KadKey
	// AddPeer tries to add a peer to the routing table
	AddPeer(context.Context, address.NodeID) (bool, error)
	// RemoveKey tries to remove a peer identified by its Kademlia key from the
	// routing table
	RemoveKey(context.Context, key.KadKey) (bool, error)
	// NearestPeers returns the closest peers to a given key
	NearestPeers(context.Context, key.KadKey, int) ([]address.NodeID, error)
}

// RemovePeer removes a peer from the routing table
func RemovePeer(ctx context.Context, rt Table, k address.NodeID) (bool, error) {
	return rt.RemoveKey(ctx, k.Key())
}
