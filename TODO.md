# TODOs:

Major missing features:
- Libp2p notifee (add peers that connect to us, and support the right protocol)
- Provider Record/IPNS store
- Refreshing routing table

Improvements:
- Event priority queue
- better routing table (see [description](./routingtable/))

New features (non-breaking):
- Use rcmgr
- Use signed peer records
- Better Provider/IPNS Record Store (only store what the server can read, not the full protobuf)
- Move reprovide interface in the Kademlia implementation
- Provider Record TTL