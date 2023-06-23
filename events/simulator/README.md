# Simulator

A `Simulator` executes `Action`s from a set of provided `Scheduler`s. It is used to simulate interactions between multiple peers, represented by their respective `Scheduler`s. Some `Action` run from a scheduler can cause more `Action`s to be enqueued or scheduled on the same scheduler or on other schedulers.

When `Run` is called, the simulation will run until there are no more `Action` to run in any of the `Scheduler`s.

```go
// Simulator is an interface for simulating a set of schedulers.
type Simulator interface {
	// AddPeer adds a scheduler to the simulator
	AddPeer(scheduler.AwareScheduler)
	// RemovePeer removes a scheduler from the simulator
	RemovePeer(scheduler.AwareScheduler)
	// Run runs the simulator until there are no more Actions to run
	Run(context.Context)
}
```