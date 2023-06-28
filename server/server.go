package server

import (
	"context"

	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

// Server is the interface for handling requests from remote nodes.
type Server interface {
	// HandleRequest handles a request from a remote peer.
	HandleRequest(context.Context, address.NodeID,
		message.MinKadMessage) (message.MinKadMessage, error)
}
