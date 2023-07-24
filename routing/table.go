package routing

import (
	"context"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
)

// Table is the interface for Kademlia Routing Tables
type Table[K kad.Key[K]] interface {
	// Self returns the local node's Kademlia key
	Self() K
	// AddPeer tries to add a peer to the routing table
	AddPeer(context.Context, address.NodeID[K]) (bool, error)
	// RemoveKey tries to remove a peer identified by its Kademlia key from the
	// routing table
	RemoveKey(context.Context, K) (bool, error)
	// NearestPeers returns the closest peers to a given key
	NearestPeers(context.Context, K, int) ([]address.NodeID[K], error)
}

// RemovePeer removes a peer from the routing table
func RemovePeer[K kad.Key[K]](ctx context.Context, rt Table[K], a address.NodeID[K]) (bool, error) {
	return rt.RemoveKey(ctx, a.Key())
}
