package simplequery

import (
	"sort"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
)

type peerStatus uint8

const (
	queued peerStatus = iota
	waiting
	queried
	unreachable
)

type peerInfo struct {
	distance key.KadKey
	status   peerStatus
	id       address.NodeID

	next *peerInfo
}

type peerList struct {
	target key.KadKey

	closest       *peerInfo
	closestQueued *peerInfo

	queuedCount int
}

func newPeerList(target key.KadKey) *peerList {
	return &peerList{
		target:      target,
		queuedCount: 0,
	}
}

// normally peers should already be ordered with distance to target, but we
// sort them just in case
func (pl *peerList) addToPeerlist(ids []address.NodeID) {

	// linked list of new peers sorted by distance to target
	newHead := sliceToPeerInfos(pl.target, ids)
	if newHead == nil {
		return
	}

	oldHead := pl.closest

	// if the list is empty, define first new peer as closest
	if oldHead == nil {
		pl.closest = newHead
		pl.closestQueued = newHead

		for curr := newHead; curr != nil; curr = curr.next {
			pl.queuedCount++
		}
		return
	}

	// merge the new sorted list into the existing sorted list
	var prev *peerInfo
	currOld := true
	closestQueuedReached := false

	r, _ := oldHead.distance.Compare(newHead.distance)
	if r > 0 {
		pl.closest = newHead
		pl.closestQueued = newHead
		currOld = false
	}

	for {
		if r > 0 {
			// newHead is closer than oldHead

			if !closestQueuedReached {
				// newHead is closer than closestQueued, update closestQueued
				pl.closestQueued = newHead
				closestQueuedReached = true
			}
			if currOld && prev != nil {
				prev.next = newHead
				currOld = false
			}
			prev = newHead
			newHead = newHead.next

			// increased queued count as all new peers are queued
			pl.queuedCount++
		} else {
			// oldHead is closer than newHead

			if !closestQueuedReached && oldHead == pl.closestQueued {
				// old closestQueued is closer than newHead,
				// don't update closestQueued
				closestQueuedReached = true
			}

			if !currOld && prev != nil {
				prev.next = oldHead
				currOld = true
			}
			prev = oldHead
			oldHead = oldHead.next
			if r == 0 {
				// newHead is a duplicate of oldHead, discard newHead
				newHead = newHead.next
			}
		}
		// we are done when we reach the end of either list
		if oldHead == nil || newHead == nil {
			break
		}
		r, _ = oldHead.distance.Compare(newHead.distance)
	}

	// append the remaining list to the end
	if oldHead == nil {
		prev.next = newHead

		// if there are still new peers to be appended, increase queued count
		for curr := newHead; curr != nil; curr = curr.next {
			pl.queuedCount++
		}
	} else {
		prev.next = oldHead
	}
}

func sliceToPeerInfos(target key.KadKey, ids []address.NodeID) *peerInfo {

	// create a new list of peerInfo
	newPeers := make([]*peerInfo, 0, len(ids))
	for _, id := range ids {
		newPeer := addrInfoToPeerInfo(target, id)
		if newPeer != nil {
			newPeers = append(newPeers, newPeer)
		}
	}

	if len(newPeers) == 0 {
		return nil
	}

	// sort the new list
	sort.Slice(newPeers, func(i, j int) bool {
		r, _ := newPeers[i].distance.Compare(newPeers[j].distance)
		return r < 0
	})

	// convert slice to linked list and remove duplicates
	curr := newPeers[0]
	for i := 1; i < len(newPeers); i++ {
		r, _ := curr.distance.Compare(newPeers[i].distance)
		if r != 0 {
			curr.next = newPeers[i]
			curr = curr.next
		}
	}
	// return head of linked list
	return newPeers[0]
}

func addrInfoToPeerInfo(target key.KadKey, id address.NodeID) *peerInfo {
	if id == nil || id.String() == "" || target.Size() != id.Key().Size() {
		return nil
	}
	dist, _ := target.Xor(id.Key())
	return &peerInfo{
		distance: dist,
		status:   queued,
		id:       id,
	}
}

func (pl *peerList) updatePeerStatusInPeerlist(id address.NodeID, newStatus peerStatus) {
	curr := pl.closest
	for curr != nil && curr.id.String() != id.String() {
		curr = curr.next
	}
	if curr != nil {
		if curr.status == queued && newStatus != queued {
			pl.queuedCount--
		} else if curr.status != queued && newStatus == queued {
			pl.queuedCount++

			for curr := pl.closest; curr != nil; curr = curr.next {
				// if a peer is set to queued, we may need to update closestQueued
				if curr.id.String() == id.String() {
					pl.closestQueued = curr
					break
				} else if curr == pl.closestQueued {
					break
				}
			}
		}

		curr.status = newStatus

		if curr == pl.closestQueued && newStatus != queued {
			pl.closestQueued = findNextQueued(curr)
		}
	}
}

func (pl *peerList) popClosestQueued() address.NodeID {
	if pl.closestQueued == nil {
		return nil
	}
	pi := pl.closestQueued
	pi.status = waiting
	pl.queuedCount--

	pl.closestQueued = findNextQueued(pi)
	return pi.id
}

func findNextQueued(pi *peerInfo) *peerInfo {
	curr := pi
	for curr != nil && curr.status != queued {
		curr = curr.next
	}
	return curr
}
