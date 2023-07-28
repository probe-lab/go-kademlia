Author: [Ian Davis](https://github.com/iand)

Last Updated: 2023-07-28

This document descriibes the design and rationale of the state machine approach to implementing the Kademlia protocol.

## Rationale

The main non-functional design goal of this new Kademlia implementation is to create a more predictable, maintainable and testable codebase that
can provide an extensible platform for new DHT capabilities. As OSS it is important that new contributors are attracted to the project so ease of 
understanding, straightforward debugging and consistency of design are also high priority.

The hierarchical state machine approach addresses these goals in the following ways:

 - **Determinism** - The state machine follows a deterministic flow, meaning that given a particular input or set of conditions, it will always produce the same output. This predictability makes it easier to isolate and reproduce issues, as bugs are more likely to occur consistently within specific states or transitions. The events in the state machines can be linearised, in the sense that the same sequence of events will produce the same sequence of state transitions, regardless of the timing of event arrival. When combined with a determistic clock implementation this enables accurate simulation of time-dependent behaviours, such as timeouts.

 - **Transparency** - Each state in the machine is assigned a specific responsibility, focusing in on a particular aspect of the protocol's behaviour.
This focused scope narrows down the search for potential problems to specific areas of the codebase, reducing the debugging space and making it more manageable. The states are represented as simple data values that can be easily inspected or logged which provides developers with clear visibility into the state transitions and the current state of the protocol during its execution. Developers can effectively track the sequence of events leading to an issue, enabling them to identify root causes and simplifying resolution.

 - **Testability** - The state machines can be unit tested by testing each state and transition separately. This approach helps ensure that each component functions correctly, leading to a more reliable and robust protocol implementation. When debugging a specific state, developers can isolate that state's context, ignoring the complexities of other parts of the code. This isolation allows for focused testing and simplifies the understanding of how individual states contribute to the overall behavior. A state machine can be placed in a specific initial state and tested for expected behaviour under various valid and invalid inputs.

 - **Extensibility** - A state machine is inherently easier to extend due to its modular and hierarchical structure. Each state in the machine represents a specific behaviour or functionality, making it simpler to add new states and transitions to accommodate additional features or enhancements. The clear separation of concerns, where each state focuses on a specific aspect of the software's behaviour, makes it easier to identify the appropriate location for incorporating new features, reducing the risk of unintended interactions with existing states.

- **Consistency and Understandability** - The hierarchical nature of the state machines promotes readability and understandability of the code. New states are logically integrated into the existing structure, making it easier for developers to comprehend the software's behaviour as a whole. The transitions between states can be drawn top help visualise behaviours. The consistent use of the state machine pattern across components ensures uniformity in behavior and interface, allowing developers to understand the system more quickly and reducing ambiguity.

In addition, the state machine model enforces a structured and controlled execution flow, making it easier to impose limits on resource consumption:

 - **Memory** - As the state machines transition between states they can release memory allocated for states that are no longer active.
 - **Concurrency** - The state machines provide bounds on concurrency, offering full control over the number of in-flight operations. 
 - **CPU** - The finite number of states and combinations ensure that the work to be undertaken is bounded and known ahead of time.
 - **Network** - Network connections are opened, closed, and reused appropriately based on the state transitions. This controlled socket management can help prevent leaks or exhaustion, leading to more efficient utilization of network resources.

## Components

### The Coordinator (`coord` package)

Kademlia is inherently an asynchronous protocol dependent on variability of network latency and responsiveness of peers.
We shouldn't hide this complexity from users of go-kademlia but we can confine its API surface.

The `Coordinator` plays this role. Users of this type subscribe to a channel of Kademlia events to receive notification
of the progress of queries as well as other things including updates to routing tables. Users call methods on the
Coordinator to initiate operations such a starting a query. They receive an error response immediately if the operation
could not be started, perhaps due to invalid arguments passed by the caller. Otherwise, if no error is returned,
the caller must wait for events on the channel to monitor the outcome of their request.

A pattern for using this is in the `statemachine` example. The `IpfsDht` type in that example implements a
synchronous `FindNode` method that starts a query via the coordinator and then monitors the event channel
for updates as the query progresses, until it finally completes.

### The Query state machines (`query` package)

Queries are operated via two state machines organised hierarchically. The `Pool` state machine manages the lifetimes of
any number of `Query` state machines. Each user-submitted query is added to the `Pool` and is executed by a
dedicated `Query`.


## Operation







