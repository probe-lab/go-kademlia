# IPFS v1 Message Format

Author: [Guillaume Michel](https://github.com/guillaumemichel)

This package contains the `.proto` file for IPFS DHT messages. [`helpers.go`](./helpers.go) implements the missing methods to make sure that `Message` implements `ProtoKadRequestMessage` and `ProtoKadResponseMessage`. It also contain helpers methods to help build IPFS DHT requests and responses.