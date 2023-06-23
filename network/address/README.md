# Addressing

The `NodeID` is the Kademlia node identifier. It must implement a `Key()` method mapping the `NodeID` to a Kademlia Key. It can also contain additional information such as the node's network addresses.

```go
// NodeID is a generic node identifier. It is used to identify a node and can
// also include extra information about the node, such as its network addresses.
type NodeID interface {
	// Key returns the KadKey of the NodeID.
	Key() key.KadKey
	// String returns the string representation of the NodeID. String
	// representation should be unique for each NodeID.
	String() string
}
```

## `NodeID` implementation

### `StringID`

`StringID` uses a simple `string` as `NodeID`, and the `Key()` is derived using [SHA256](../../key/sha256key256/).

### `KadID`

`KadID` is a `KadKey` wrapper. It is useful in order to test different routing scenarios, because the `KadID` controls where the `Key` lies in the keyspace. The node's location in the keyspace is not random.

### `PeerID`

`PeerID` is a wrapper of the libp2p `peer.ID`.

### `AddrInfo`

`AddrInfo` is a wrapper of the libp2p `peer.AddrInfo`, containing a `peer.ID` and `[]multiaddr.multiaddr`.