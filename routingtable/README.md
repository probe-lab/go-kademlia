# Routing Table

`RoutingTable` defines a generic Kademlia routing table interface.

```go
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
```

In `go-libp2p-kad-dht`, the Routing Table periodically refreshes. This operation consists in looking for its own Kademlia key, to be aware of one's closest neighbors at all time, and making sure that the buckets are _as full as possible_ with reachable peers. So a node will make sure that all peers that are in its routing table are still online, and will replace the offline peers with fresh ones.

## Implementations

- `SimpleRT` a very simple routing table implementation that should NOT be used in production.
- `ClientRT` (doesn't exist yet) a routing table implementation that is optimized for nodes in client mode only
- `TrieRT` (doesn't exist yet) a routing table implementation based on a binary trie to store Kademlia keys and optimize distance computations.
- `FullRT` (not migrated yet) a routing table implementation that periodically crawls the network and stores all nodes.
- `LazyRT` (doesn't exist yet) a routing table implementation keeping all peers it has heard of in its routing table, but only refreshes a subset of them periodically. Some peers may be unreachable.

## Challenges

2023-05-23: We want to keep track of the remote Clients that are close to us. So we want to add them in our routing table. However, we don't want to give them as _closer peers_ when answering a `FIND_NODE` request. They should remain in the RT (as long as there is space in the buckets), but not be shared. They should be kept in the routing table, but they aren't prioritary compared with other DHT servers.