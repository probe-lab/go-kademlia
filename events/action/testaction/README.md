# TestAction

Author: [Guillaume Michel](https://github.com/guillaumemichel)

## IntAction

An `IntAction` is a very elementary `Action` that does nothing when run. It is a type alias for `int`, so `IntAction(0) != IntAction(1)`. It is used mostly for testing `Queue` and `Planner` implementations.

## FuncAction

A `FuncAction` is also an elementary `Action` implementation. It behaves as `IntAction`, but keeps track of whether it has been run yet or not. It is convenient to use when testing `Queue` and `Planner` implementation, for we can test whether the action has been run.