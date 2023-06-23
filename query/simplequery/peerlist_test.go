package simplequery

import (
	"testing"

	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/peerid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestAddPeers(t *testing.T) {
	// create empty peer list
	pl := newPeerList(make([]byte, 32))

	require.Nil(t, pl.closest)
	require.Nil(t, pl.closestQueued)

	// add initial peers
	nPeers := 3
	peerids := make([]address.NodeID, nPeers+1)
	for i := 0; i < nPeers; i++ {
		peerids[i] = peerid.PeerID{ID: peer.ID(byte(i))}
	}
	peerids[nPeers] = peerid.PeerID{ID: peer.ID(byte(0))} // duplicate with peerids[0]

	// distances
	// peerids[0]: 6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d
	// peerids[1]: 4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a
	// peerids[2]: dbc1b4c900ffe48d575b5da5c638040125f65db0fe3e24494b76ea986457d986
	// peerids[3]: 6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d

	// add 4 peers (incl. 1 duplicate)
	pl.addToPeerlist(peerids)

	require.Equal(t, nPeers, pl.queuedCount)

	curr := pl.closest
	// verify that closest peer is peerids[1]
	require.Equal(t, peerids[1], curr.id)
	curr = curr.next
	// second closest peer should be peerids[0]
	require.Equal(t, peerids[0], curr.id)
	curr = curr.next
	// third closest peer should be peerids[2]
	require.Equal(t, peerids[2], curr.id)

	// end of the list
	require.Nil(t, curr.next)

	// verify that closestQueued peer is peerids[0]
	require.Equal(t, peerids[1], pl.closestQueued.id)

	// add more peers
	nPeers = 5
	newPeerids := make([]address.NodeID, nPeers+2)
	for i := 0; i < nPeers; i++ {
		newPeerids[i] = peerid.PeerID{ID: peer.ID(byte(10 + i))}
	}
	newPeerids[nPeers] = peerid.PeerID{ID: peer.ID(byte(10))}  // duplicate with newPeerids[0]
	newPeerids[nPeers+1] = peerid.PeerID{ID: peer.ID(byte(1))} // duplicate with peerids[1]

	// distances
	// newPeerids[0]: 01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b
	// newPeerids[1]: e7cf46a078fed4fafd0b5e3aff144802b853f8ae459a4f0c14add3314b7cc3a6
	// newPeerids[2]: ef6cbd2161eaea7943ce8693b9824d23d1793ffb1c0fca05b600d3899b44c977
	// newPeerids[3]: 9d1e0e2d9459d06523ad13e28a4093c2316baafe7aec5b25f30eba2e113599c4
	// newPeerids[4]: 4d7b3ef7300acf70c892d8327db8272f54434adbc61a4e130a563cb59a0d0f47
	// newPeerids[5]: 01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b
	// newPeerids[6]: 4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a

	// add 7 peers (incl. 2 duplicates)
	pl.addToPeerlist(newPeerids)

	require.Equal(t, 8, pl.queuedCount)

	// order is now as follows:
	order := []address.NodeID{newPeerids[0], peerids[1], newPeerids[4], peerids[0], newPeerids[3],
		peerids[2], newPeerids[1], newPeerids[2]}

	curr = pl.closest
	for _, p := range order {
		require.Equal(t, p, curr.id)
		curr = curr.next
	}
	require.Nil(t, curr)

	// verify that closestQueued peer is peerids[0]
	require.Equal(t, newPeerids[0], pl.closestQueued.id)

	// add a single peer that isn't the closest one
	newPeer := peerid.PeerID{ID: peer.ID(byte(20))}

	pl.addToPeerlist([]address.NodeID{newPeer})
	order = append(order[:5], order[4:]...)
	order[4] = newPeer

	curr = pl.closest
	for _, p := range order {
		require.Equal(t, p, curr.id)
		curr = curr.next
	}

	require.Nil(t, curr)
}

func TestPeerlistCornerCases(t *testing.T) {
	// different empty peerid lists
	emptyPeeridLists := [][]address.NodeID{
		{},
		{peerid.PeerID{ID: peer.ID("")}},
		make([]address.NodeID, 3),
	}

	for _, peerids := range emptyPeeridLists {
		// adding them to a peerlist should result in an empty list
		require.Nil(t, sliceToPeerInfos(make([]byte, 32), peerids))

		pl := newPeerList(make([]byte, 32))
		pl.addToPeerlist(peerids)
		require.Nil(t, pl.closest)
		require.Nil(t, pl.closestQueued)
		require.Equal(t, 0, pl.queuedCount)
	}

	pl := newPeerList(make([]byte, 32))

	singlePeerList0 := []address.NodeID{peerid.PeerID{ID: peer.ID(byte(0))}}
	// 6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d
	pl.addToPeerlist(singlePeerList0)
	require.Equal(t, singlePeerList0[0], pl.closest.id)
	require.Equal(t, singlePeerList0[0], pl.closestQueued.id)
	require.Equal(t, 1, pl.queuedCount)

	singlePeerList1 := []address.NodeID{peerid.PeerID{ID: peer.ID(byte(1))}}
	// 4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a
	pl.addToPeerlist(singlePeerList1)
	require.Equal(t, singlePeerList1[0], pl.closest.id)
	require.Equal(t, singlePeerList1[0], pl.closest.id)
	require.Equal(t, 2, pl.queuedCount)

	singlePeerList2 := []address.NodeID{peerid.PeerID{ID: peer.ID(byte(2))}}
	// dbc1b4c900ffe48d575b5da5c638040125f65db0fe3e24494b76ea986457d986
	pl.addToPeerlist(singlePeerList2)
	require.Equal(t, 3, pl.queuedCount)

	curr := pl.closest
	require.Equal(t, singlePeerList1[0], curr.id)
	curr = curr.next
	require.Equal(t, singlePeerList0[0], curr.id)
	curr = curr.next
	require.Equal(t, singlePeerList2[0], curr.id)
	curr = curr.next
	require.Nil(t, curr)
}

func TestUpdatePeerStatusInPeerlist(t *testing.T) {
	// create empty peer list
	pl := newPeerList(make([]byte, 32))

	// add initial peers
	nPeers := 3
	peerids := make([]address.NodeID, nPeers)
	for i := 0; i < nPeers; i++ {
		peerids[i] = peerid.PeerID{ID: peer.ID(byte(i))}
	}

	pl.addToPeerlist(peerids)

	require.Equal(t, 3, pl.queuedCount)

	// initial queue state
	// peerids[1], queued, 4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a
	// peerids[0], queued, 6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d
	// peerids[2], queued, dbc1b4c900ffe48d575b5da5c638040125f65db0fe3e24494b76ea986457d986

	pl.updatePeerStatusInPeerlist(peerids[0], waiting)
	require.Equal(t, 2, pl.queuedCount)
	pl.updatePeerStatusInPeerlist(peerids[1], unreachable)
	require.Equal(t, 1, pl.queuedCount)

	curr := pl.closest
	require.Equal(t, curr.status, unreachable)
	curr = curr.next
	require.Equal(t, curr.status, waiting)
	curr = curr.next
	require.Equal(t, curr.status, queued)
	curr = curr.next
	require.Nil(t, curr)
}

func TestPopClosestQueued(t *testing.T) {
	// create empty peer list
	pl := newPeerList(make([]byte, 32))

	// add initial peers
	nPeers := 3
	peerids := make([]address.NodeID, nPeers)
	for i := 0; i < nPeers; i++ {
		peerids[i] = peerid.PeerID{ID: peer.ID(byte(i))}
	}

	pl.addToPeerlist(peerids)
	require.Equal(t, 3, pl.queuedCount)

	// initial queue state
	// peerids[1], queued, 4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a
	// peerids[0], queued, 6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d
	// peerids[2], queued, dbc1b4c900ffe48d575b5da5c638040125f65db0fe3e24494b76ea986457d986

	require.Equal(t, peerids[1], pl.popClosestQueued())
	require.Equal(t, peerids[1], pl.closest.id)
	require.Equal(t, peerids[0], pl.closestQueued.id)
	require.Equal(t, 2, pl.queuedCount)
	require.Equal(t, peerids[0], pl.popClosestQueued())
	require.Equal(t, peerids[1], pl.closest.id)
	require.Equal(t, peerids[2], pl.closestQueued.id)
	require.Equal(t, 1, pl.queuedCount)
	require.Equal(t, peerids[2], pl.popClosestQueued())
	require.Equal(t, 0, pl.queuedCount)
	require.Equal(t, nil, pl.popClosestQueued())
	require.Equal(t, 0, pl.queuedCount)

	pl = newPeerList(make([]byte, 32))

	pl.addToPeerlist(peerids)
	require.Equal(t, 3, pl.queuedCount)

	// mark second item (peerids[0]) as waiting
	pl.updatePeerStatusInPeerlist(peerids[0], waiting)
	require.Equal(t, peerids[1], pl.closest.id)
	require.Equal(t, peerids[1], pl.closestQueued.id)
	require.Equal(t, 2, pl.queuedCount)

	// pop closest queued (peerids[1])
	require.Equal(t, peerids[1], pl.popClosestQueued())
	require.Equal(t, peerids[1], pl.closest.id)
	// peerids[2] is now closestQueued
	require.Equal(t, peerids[2], pl.closestQueued.id)
	require.Equal(t, 1, pl.queuedCount)

	pl.updatePeerStatusInPeerlist(peerids[2], unreachable)
	require.Equal(t, peerids[1], pl.closest.id)
	require.Equal(t, 0, pl.queuedCount)
	require.Nil(t, pl.closestQueued)
	require.Equal(t, 0, pl.queuedCount)

	pl.updatePeerStatusInPeerlist(peerids[1], queued)
	require.Equal(t, peerids[1], pl.closestQueued.id)
	require.Equal(t, 1, pl.queuedCount)
	pl.updatePeerStatusInPeerlist(peerids[2], queued)
	require.Equal(t, peerids[1], pl.closestQueued.id)
	require.Equal(t, 2, pl.queuedCount)
}
