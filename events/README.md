# Events Management

The goal of this folder is to provide an abstraction for single worker multi threaded applications. Some applications are multi threaded by design (e.g Kademlia lookup), but having a sequential execution brings many benefits such as deterministic testing, easier debugging, sequential tracing, and sometimes even increased performance. 

- ## [`action`](./action/)
    Contains the logic of actions, that are going to be run one by one by the single worker.
- ## [`queue`](./queue/)
    Contains the logic of an action queue, where actions that have to be run wait to be executed one at a time by the single worker.
- ## [`planner`](./planner/)
    Contains the logic of planning Actions to be executed at a specific time.
- ## [`scheduler`](./scheduler/)
    Contains the combined logic of both [`queue`](./queue/) and [`planner`](./planner/). It exposes all required functions to enqueue, schedule and run actions.
- ## [`simulator`](./simulator/)
    Contains the logic of executing actions that are queued or scheduled on multiple schedulers.