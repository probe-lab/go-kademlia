package routingtable

import (
	"context"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
)

// RoutingTable is the interface for Kademlia Routing Tables
type RoutingTable interface {
	// Self returns the local node's Kademlia key
	Self() key.KadKey
	// AddPeer tries to add a peer to the routing table
	AddPeer(context.Context, address.NodeID) (bool, error)
	// RemovePeer tries to remove a peer identified by its Kademlia key from the
	// routing table
	RemoveKey(context.Context, key.KadKey) (bool, error)
	// NearestPeers returns the closest peers to a given key
	NearestPeers(context.Context, key.KadKey, int) ([]address.NodeID, error)
}

// RemovePeer removes a peer from the routing table
func RemovePeer(ctx context.Context, rt RoutingTable, k address.NodeID) (bool, error) {
	return rt.RemoveKey(ctx, k.Key())
}
