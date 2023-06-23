package endpoint

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p/core/network"
)

// RequestHandlerFn defines a function that handles a request from a remote peer
type RequestHandlerFn func(context.Context, address.NodeID,
	message.MinKadMessage) (message.MinKadMessage, error)

// ResponseHandlerFn defines a function that deals with the response to a
// request previously sent to a remote peer.
type ResponseHandlerFn func(context.Context, message.MinKadResponseMessage, error)

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

// NetworkedEndpoint is an endpoint keeping track of the connectedness with
// known remote peers.
type NetworkedEndpoint interface {
	Endpoint
	// Connectedness returns the connectedness of the given peer.
	Connectedness(address.NodeID) network.Connectedness
}

// StreamID is a unique identifier for a stream.
type StreamID uint64

// SimEndpoint is a simulated endpoint that doesn't operate on real network
type SimEndpoint interface {
	ServerEndpoint
	// HandleMessage handles a message from the given peer.
	HandleMessage(context.Context, address.NodeID, address.ProtocolID,
		StreamID, message.MinKadMessage)
}
