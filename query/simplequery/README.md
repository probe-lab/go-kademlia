# Simple Query

Author: [Guillaume Michel](https://github.com/guillaumemichel)

`SimpleQuery` is a simple multi threaded single worker query mechanism.

It takes as input the target Kademlia Key, the `ProtocolID` of the request, the request message, an empty response message (to parse the reponse once received), a concurrency paramter, a default message timeout value, the message endpoint used to communicate with remote peers, a routing table to select the closest peers and add newly discovered peers, a scheduler to give actions to the single worker, and a `handleResultFn` function that is defined by the called.

The `handleResultFn` is used by the caller to interact with the received responses, and to save a state for the query. It is through this function that the caller decides when the query terminates.

```go
func NewSimpleQuery(ctx context.Context, kadid key.KadKey, proto address.ProtocolID,
	req message.MinKadMessage, resp message.MinKadResponseMessage, concurrency int,
	timeout time.Duration, msgEndpoint endpoint.Endpoint, rt routingtable.RoutingTable,
	sched scheduler.Scheduler, handleResultFn HandleResultFn) *SimpleQuery
```
