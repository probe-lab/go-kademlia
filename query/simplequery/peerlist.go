package simplequery

import (
	"sort"

	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
	"github.com/probe-lab/go-kademlia/network/endpoint"
)

type NodeStatus uint8

const (
	queued NodeStatus = iota
	waiting
	queried
	unreachable
)

type nodeInfo[K kad.Key[K], A kad.Address[A]] struct {
	distance          K
	status            NodeStatus
	id                kad.NodeID[K]
	addrs             []A
	tryAgainOnFailure bool

	next *nodeInfo[K, A]
}

type PeerList[K kad.Key[K], A kad.Address[A]] struct {
	target   K
	endpoint endpoint.NetworkedEndpoint[K, A]

	closest       *nodeInfo[K, A]
	closestQueued *nodeInfo[K, A]

	queuedCount int
}

func newPeerList[K kad.Key[K], A kad.Address[A]](target K, ep endpoint.Endpoint[K, A]) *PeerList[K, A] {
	nep, ok := ep.(endpoint.NetworkedEndpoint[K, A])
	if !ok {
		nep = nil
	}
	return &PeerList[K, A]{
		target:   target,
		endpoint: nep,
	}
}

// normally peers should already be ordered with distance to target, but we
// sort them just in case
func (pl *PeerList[K, A]) addToPeerlist(ids []kad.NodeID[K]) {
	// linked list of new peers sorted by distance to target
	newHead := sliceToPeerInfos[K, A](pl.target, ids)
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
	var prev *nodeInfo[K, A]
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
								if addr.Equal(oldAddr) {
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

func sliceToPeerInfos[K kad.Key[K], A kad.Address[A]](target K, ids []kad.NodeID[K]) *nodeInfo[K, A] {
	// create a new list of nodeInfo
	newPeers := make([]*nodeInfo[K, A], 0, len(ids))
	for _, id := range ids {
		newPeer := addrInfoToPeerInfo[K, A](target, id)
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

func addrInfoToPeerInfo[K kad.Key[K], A kad.Address[A]](target K, id kad.NodeID[K]) *nodeInfo[K, A] {
	if id == nil || id.String() == "" || target.BitLen() != id.Key().BitLen() {
		return nil
	}
	return &nodeInfo[K, A]{
		distance: target.Xor(id.Key()),
		status:   queued,
		id:       id,
	}
}

func (pl *PeerList[K, A]) enqueueUnreachablePeer(pi *nodeInfo[K, A]) {
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
func (pl *PeerList[K, A]) popClosestQueued() kad.NodeID[K] {
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

func (pl *PeerList[K, A]) queriedPeer(id kad.NodeID[K]) {
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
func (pl *PeerList[K, A]) unreachablePeer(id kad.NodeID[K]) {
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

func findNextQueued[K kad.Key[K], A kad.Address[A]](pi *nodeInfo[K, A]) *nodeInfo[K, A] {
	curr := pi
	for curr != nil && curr.status != queued {
		curr = curr.next
	}
	return curr
}
