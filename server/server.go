package server

import (
	"context"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

// Server is the interface for handling requests from remote nodes.
type Server[K kad.Key[K]] interface {
	// HandleRequest handles a request from a remote peer.
	HandleRequest(context.Context, address.NodeID[K],
		message.MinKadMessage) (message.MinKadMessage, error)
}
