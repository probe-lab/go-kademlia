# Simple Planner

Author: [Guillaume Michel](https://github.com/guillaumemichel)

`SimplePlanner` is a simple implementation of `ActionPlanner`. The `Clock` provided to the builder `NewSimplePlanner` can be a mock clock, allowing time manipulation for tests and simulations.

`simpleTimedAction` implements `PlannedAction`.

`SimplePlanner` also implements `AwareActionPlanner`, offering the additional function `NextActionTime`, returning the time of the next action. This is required for tests and simulations, where the time can be controlled, so that time can be changed to the next `Action`'s time.
