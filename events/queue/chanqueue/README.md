# ChanQueue

Author: [Guillaume Michel](https://github.com/guillaumemichel)

`ChanQueue` is a simple Event Queue implementation using a channel. The channel capacity must be provided when creating a new `ChanQueue`. Note that when the queue is full, the new `Action`s to be enqueued are discarded.

This `Queue` implementation is safe to use in multithreaded environments as it is relies on a channel. Concurrent `Enqueue` are allowed.