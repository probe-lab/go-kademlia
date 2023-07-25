package endpoint

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

// RequestHandlerFn defines a function that handles a request from a remote peer
type RequestHandlerFn[K kad.Key[K]] func(context.Context, kad.NodeID[K],
	message.MinKadMessage) (message.MinKadMessage, error)

// ResponseHandlerFn defines a function that deals with the response to a
// request previously sent to a remote peer.
type ResponseHandlerFn[K kad.Key[K], A any] func(context.Context, message.MinKadResponseMessage[K, A], error)

// Endpoint defines how Kademlia nodes interacts with each other.
type Endpoint[K kad.Key[K], A any] interface {
	// MaybeAddToPeerstore adds the given address to the peerstore if it is
	// valid and if it is not already there.
	MaybeAddToPeerstore(context.Context, kad.NodeInfo[K, A], time.Duration) error
	// SendRequestHandleResponse sends a request to the given peer and handles
	// the response with the given handler.
	SendRequestHandleResponse(context.Context, address.ProtocolID, kad.NodeID[K],
		message.MinKadMessage, message.MinKadMessage, time.Duration,
		ResponseHandlerFn[K, A]) error

	// KadKey returns the KadKey of the local node.
	KadKey() K
	// NetworkAddress returns the network address of the given peer (if known).
	NetworkAddress(kad.NodeID[K]) (kad.NodeInfo[K, A], error)
}

// ServerEndpoint is a Kademlia endpoint that can handle requests from remote
// peers.
type ServerEndpoint[K kad.Key[K], A any] interface {
	Endpoint[K, A]
	// AddRequestHandler registers a handler for a given protocol ID.
	AddRequestHandler(address.ProtocolID, message.MinKadMessage, RequestHandlerFn[K]) error
	// RemoveRequestHandler removes a handler for a given protocol ID.
	RemoveRequestHandler(address.ProtocolID)
}

// NetworkedEndpoint is an endpoint keeping track of the connectedness with
// known remote peers.
type NetworkedEndpoint[K kad.Key[K], A any] interface {
	Endpoint[K, A]
	// Connectedness returns the connectedness of the given peer.
	Connectedness(kad.NodeID[K]) (network.Connectedness, error)
}

// StreamID is a unique identifier for a stream.
type StreamID uint64

// SimEndpoint is a simulated endpoint that doesn't operate on real network
type SimEndpoint[K kad.Key[K], A any] interface {
	ServerEndpoint[K, A]
	// HandleMessage handles a message from the given peer.
	HandleMessage(context.Context, kad.NodeID[K], address.ProtocolID,
		StreamID, message.MinKadMessage)
}
