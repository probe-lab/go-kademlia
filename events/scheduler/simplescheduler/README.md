# Simple Scheduler

Author: [Guillaume Michel](https://github.com/guillaumemichel)

`SimpleScheduler` is a simple implementation of `Scheduler`. It is composed of a [`ChanQueue`](../../queue/chanqueue/) with a capacity of `1024`, and of a [`SimplePlanner`](../../planner/).

`SimpleScheduler` also implements `AwareScheduler`, offering the additional function `NextActionTime`, returning the time of the next action. This is required for tests and simulations, where the time can be controlled, so that time can be changed to the next `Action`'s time.

Eventually, the goal is to replace the limited [`ChanQueue`](../../queue/chanqueue/) by a priority queue.