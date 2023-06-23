# Libp2p Endpoint

Author: [Guillaume Michel](https://github.com/guillaumemichel)

`Libp2pEndpoint` is a message endpoint using Libp2p to exchange messages between Kademlia nodes over a network. It makes use of the Libp2p peerstore to record peers. The implementation is multi thread, as it is a requirement from Libp2p.

When sending a Kademlia request, a new go routine is created to send the request and wait for the response. Once the response is received, the go routine will add a new `Action` to handle the received response to the `Scheduler`'s event queue and dies. The single worker will pick the response handling `Action` from the `Scheduler` once it is available.

When in `Server` mode, Libp2p stream handlers are added to the Libp2p `host`. Once a new request is caught by the Libp2p stream handler, it is sent to the `Scheduler`'s event queue, and handled by the single worker.