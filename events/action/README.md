# Action

An `Action` is essentially the unit that is dealt with by `Queue` and `Planner`. An `Action` only needs to be runnable. Most of the times, an `Action` represents a pointer on a function.

```go
type Action interface {
	Run(context.Context)
}
```

[`basicaction`](./basicaction/) and [`testaction`](./testaction/) contain implementations of `Action`. [`basicaction`](./basicaction/) is the preferred implementation for real world applications. [`testaction`](./testaction/) was built mostly to test event scheduling.