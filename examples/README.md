# Examples

This folder contains Kademlia examples built with this repository.

## Featured examples

- ### [`connect`](./connect/)
    IPFS DHT client example, where a Kademlia node configured with a `libp2pendpoint` connects to a bootstrapper node from the IPFS DHT and performs a `FIND_PEER` request in the live IPFS network.
- ### [`fullsim`](./fullsim/)
    Simulation example where 4 simple nodes are created, and a node performs a multi hop lookup request. All nodes run in server mode since they have to answer each other's queries.