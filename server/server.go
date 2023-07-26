package server

import (
	"context"

	"github.com/plprobelab/go-kademlia/kad"
)

// Server is the interface for handling requests from remote nodes.
type Server[K kad.Key[K]] interface {
	// HandleRequest handles a request from a remote peer.
	HandleRequest(context.Context, kad.NodeID[K], kad.MinKadMessage) (kad.MinKadMessage, error)
}
