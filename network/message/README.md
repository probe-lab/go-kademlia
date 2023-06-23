# Message

This package defines message interfaces.

A Kademlia Request message (`MinKadRequestMessage`) should contain the `Target` Kademlia key.

A Kademlia Response message (`MinKadResponseMessage`) should contain nodes that are closer (`CloserNodes`) to the `Target` key in XOR distance.

The `ProtoKadMessage` defines a message that can be marshalled by Protobuf.

## Implementations

- `SimMessage` is a minimal message implementation
- `IPFSv1` is a Protobuf message implementation. It is the format used in the IPFS DHT (2023-06-23).