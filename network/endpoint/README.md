# Message Endpoint

A message `Endpoint` handles everything about communications with remote Kademlia peers. They implement the abstraction of an address book to keep track of remote peers, and handle sending and receiving message.

In order to have a `Server` node (responding to requests from other peers), a node must add `RequestHandler`s for specific `ProtocolID`s, and it will only answer requests for the supported protocols. A node is said to be in `Client` mode if it doesn't have request handlers for any `ProtocolID`.

```go
// Endpoint defines how Kademlia nodes interacts with each other.
type Endpoint interface {
	// MaybeAddToPeerstore adds the given address to the peerstore if it is
	// valid and if it is not already there.
	MaybeAddToPeerstore(context.Context, address.NodeID, time.Duration) error
	// SendRequestHandleResponse sends a request to the given peer and handles
	// the response with the given handler.
	SendRequestHandleResponse(context.Context, address.ProtocolID, address.NodeID,
		message.MinKadMessage, message.MinKadMessage, time.Duration, ResponseHandlerFn)

	// KadKey returns the KadKey of the local node.
	KadKey() key.KadKey
	// NetworkAddress returns the network address of the given peer (if known).
	NetworkAddress(address.NodeID) (address.NodeID, error)
}

// ServerEndpoint is a Kademlia endpoint that can handle requests from remote
// peers.
type ServerEndpoint interface {
	Endpoint
	// AddRequestHandler registers a handler for a given protocol ID.
	AddRequestHandler(address.ProtocolID, RequestHandlerFn)
	// RemoveRequestHandler removes a handler for a given protocol ID.
	RemoveRequestHandler(address.ProtocolID)
}
```

## Implementations

- **`Libp2pEndpoint`** is a message endpoint implementation based on Libp2p.
- **`FakeEndpoint`** is a simulated message endpoint, mostly used for tests and simulations.