# Simple Routing Table

Author: [Guillaume Michel](https://github.com/guillaumemichel)

`SimpleRT` is a very simple Kademlia Routing Table implementation. It is not meant to be used in production.

## Disclaimers

- `NearestPeers` isn't optimized at all, but it will work.
- `SimpleRT` never refreshes its peers. Hence it will eventually contain a lot of unreachable peers if used outside of a simulation.
