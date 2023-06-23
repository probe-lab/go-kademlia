# Kademlia Key

This package contains the logic about Kademlia Keys and the basic XOR distance arithmetic. 

## Definitions

A Kademlia key is defined as a bit string of arbitrary size. In this repository, Kademlia keys are represented as `[]byte`, and hence only key sizes that are multiples of 8 bytes are possible.

In practice different Kademlia implementations use different key sizes. For instance, the [Kademlia Paper](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf) defines keys as 160-bits long and IPFS uses 256-bits keys.

Keys are usually generated using cryptographic hash functions, however key generation doesn't matter for key operations.

## Implementations

- ### [`sha256key256`](./sha256key256/)
    Key generation algorithm used in IPFS. IPFS uses the digest of the SHA256 hash function as Kademlia keys (2023-06-23).