package simplequery

import (
	"sort"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
)

type peerStatus uint8

const (
	queued peerStatus = iota
	waiting
	queried
	unreachable
)

type peerInfo[K kad.Key[K]] struct {
	distance          K
	status            peerStatus
	id                address.NodeID[K]
	addrs             []address.Addr
	tryAgainOnFailure bool

	next *peerInfo[K]
}

type peerList[K kad.Key[K]] struct {
	target   K
	endpoint endpoint.NetworkedEndpoint[K]

	closest       *peerInfo[K]
	closestQueued *peerInfo[K]

	queuedCount int
}

func newPeerList[K kad.Key[K]](target K, ep endpoint.Endpoint[K]) *peerList[K] {
	nep, ok := ep.(endpoint.NetworkedEndpoint[K])
	if !ok {
		nep = nil
	}
	return &peerList[K]{
		target:   target,
		endpoint: nep,
	}
}

// normally peers should already be ordered with distance to target, but we
// sort them just in case
func (pl *peerList[K]) addToPeerlist(ids []address.NodeID[K]) {
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
	var prev *peerInfo[K]
	currOld := true // current element is from old list
	closestQueuedReached := false

	r := oldHead.distance.Compare(newHead.distance)
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
			if r == 0 {
				// newHead is a duplicate of oldHead, discard newHead
				newHead = newHead.next

				if pl.endpoint != nil && (oldHead.status == unreachable ||
					oldHead.status == waiting) {
					// If oldHead is unreachable, and new addresses are discovered
					// for this NodeID, set status to queued as we will try again.
					// If oldHead is waiting, set tryAgainOnFailure to true
					// so that we will try again with the newly discovered
					// addresses upon failure.
					na, err := pl.endpoint.NetworkAddress(oldHead.id)
					if err == nil {
						for _, addr := range na.Addresses() {
							found := false
							for _, oldAddr := range oldHead.addrs {
								if addr == oldAddr {
									found = true
									break
								}
							}
							if !found {
								if oldHead.status == unreachable {
									pl.enqueueUnreachablePeer(oldHead)
								} else if oldHead.status == waiting {
									oldHead.tryAgainOnFailure = true
								}
							}
						}
					}
				}
			}
			prev = oldHead
			oldHead = oldHead.next
		}
		// we are done when we reach the end of either list
		if oldHead == nil || newHead == nil {
			break
		}
		r = oldHead.distance.Compare(newHead.distance)
	}

	// append the remaining list to the end
	if oldHead == nil {
		if !closestQueuedReached {
			pl.closestQueued = newHead
		}
		prev.next = newHead

		// if there are still new peers to be appended, increase queued count
		for curr := newHead; curr != nil; curr = curr.next {
			pl.queuedCount++
		}
	} else {
		prev.next = oldHead
	}
}

func sliceToPeerInfos[K kad.Key[K]](target K, ids []address.NodeID[K]) *peerInfo[K] {
	// create a new list of peerInfo
	newPeers := make([]*peerInfo[K], 0, len(ids))
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
		return newPeers[i].distance.Compare(newPeers[j].distance) < 0
	})

	// convert slice to linked list and remove duplicates
	curr := newPeers[0]
	for i := 1; i < len(newPeers); i++ {
		if !key.Equal(curr.distance, newPeers[i].distance) {
			curr.next = newPeers[i]
			curr = curr.next
		}
	}
	// return head of linked list
	return newPeers[0]
}

func addrInfoToPeerInfo[K kad.Key[K]](target K, id address.NodeID[K]) *peerInfo[K] {
	if id == nil || id.String() == "" || target.BitLen() != id.Key().BitLen() {
		return nil
	}
	return &peerInfo[K]{
		distance: target.Xor(id.Key()),
		status:   queued,
		id:       id,
	}
}

func (pl *peerList[K]) enqueueUnreachablePeer(pi *peerInfo[K]) {
	if pi != nil {
		// curr is the id we are looking for
		if pi.status != queued {
			pi.tryAgainOnFailure = false
			pi.status = queued

			pl.queuedCount++
			// if curr is closer to target than closestQueued, update closestQueued
			if pi.distance.Compare(pl.closestQueued.distance) < 0 {
				pl.closestQueued = pi
			}
		}
	}
}

// setPeerWaiting sets the status of a "queued" peer to "waiting". it records
// the addresses associated with id in the peerstore at the time of the request
// to curr.addrs.
func (pl *peerList[K]) popClosestQueued() address.NodeID[K] {
	if pl.closestQueued == nil {
		return nil
	}
	pi := pl.closestQueued
	if pl.endpoint != nil {
		na, _ := pl.endpoint.NetworkAddress(pi.id)
		for na == nil {
			// if peer doesn't have addresses, set status to unreachable
			pi.status = unreachable
			pl.queuedCount--

			pi = findNextQueued(pi)
			if pi == nil {
				return nil
			}
			na, _ = pl.endpoint.NetworkAddress(pi.id)
		}
		pi.addrs = na.Addresses()
	}
	pi.status = waiting
	pl.queuedCount--

	pl.closestQueued = findNextQueued(pi)
	return pi.id
}

func (pl *peerList[K]) queriedPeer(id address.NodeID[K]) {
	curr := pl.closest
	for curr != nil && curr.id.String() != id.String() {
		curr = curr.next
	}
	if curr != nil {
		// curr is the id we are looking for
		if curr.status == waiting {
			curr.status = queried
		}
	}
}

// unreachablePeer sets the status of a "waiting" peer to "unreachable".
func (pl *peerList[K]) unreachablePeer(id address.NodeID[K]) {
	curr := pl.closest
	for curr != nil && curr.id.String() != id.String() {
		curr = curr.next
	}
	if curr != nil {
		// curr is the id we are looking for
		if curr.status == waiting {
			if curr.tryAgainOnFailure {
				pl.enqueueUnreachablePeer(curr)
			} else {
				curr.status = unreachable
			}
		}
	}
}

func findNextQueued[K kad.Key[K]](pi *peerInfo[K]) *peerInfo[K] {
	curr := pi
	for curr != nil && curr.status != queued {
		curr = curr.next
	}
	return curr
}
