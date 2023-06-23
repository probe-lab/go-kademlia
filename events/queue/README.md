# Event Queue

A `Queue` keeps track of `Action`s that will be run. It is useful to allow a single worker to run multithread programs. The worker will pick the `Action`s from the `Queue` one by one, and execute them until the `Queue` is empty.

```go
type EventQueue interface {
	Enqueue(context.Context, action.Action)
	Dequeue(context.Context) action.Action

	Size() uint
	Close()
}
```

`Queue` implementations **MUST** be thread safe if they are used in multi threads environments. However, it is also possible to use a `Queue` that is not thread safe, if the caller is also single threaded, e.g in testing or Kademlia simulations.

[`ChanQueue`](./chanqueue/) is a thread safe `Queue` implementation.

## Priority queue

A priority queue sorts events by priority e.g the first element of the queue is always the most urgent one. There are currently no priority queue implementation. Priority queues are useful if the event queue is getting filled quickly and prioritary events get stuck.

In the context of Kademlia, here is a draft list of action priorities:
- ctx cancel
- IO (read from provider store)
- query cancel
- server requests (from remote peers)
- handle response to sent requests
- sending the first messages of a query (first in terms of concurrency)
- new client requests (find/provide)
- request timeout
- ...
- sending the last messages of a query
- bucket/node refresh
- reprovide