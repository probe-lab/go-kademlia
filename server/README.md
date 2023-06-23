# Server

A Kademlia `Server` is responsible for answering requests from remote peers. It implements a single `HandleRequest` function producing a response from the provided request message.

```go
// Server is the interface for handling requests from remote nodes.
type Server interface {
	// HandleRequest handles a request from a remote peer.
	HandleRequest(context.Context, address.NodeID,
		message.MinKadMessage) (message.MinKadMessage, error)
}
```