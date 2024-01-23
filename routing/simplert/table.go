package simplert

import (
	"sort"
	"sync"

	"github.com/probe-lab/go-kademlia/internal/kadtest"

	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
)

type peerInfo[K kad.Key[K], N kad.NodeID[K]] struct {
	id    N
	kadId K
}

type SimpleRT[K kad.Key[K], N kad.NodeID[K]] struct {
	self       K
	bucketSize int

	mu      sync.RWMutex // guards access to buckets
	buckets [][]peerInfo[K, N]
}

var _ kad.RoutingTable[key.Key256, kadtest.ID[key.Key256]] = (*SimpleRT[key.Key256, kadtest.ID[key.Key256]])(nil)

func New[K kad.Key[K], N kad.NodeID[K]](self N, bucketSize int) *SimpleRT[K, N] {
	rt := SimpleRT[K, N]{
		self:       self.Key(),
		buckets:    make([][]peerInfo[K, N], 0),
		bucketSize: bucketSize,
	}
	// define bucket 0
	rt.buckets = append(rt.buckets, make([]peerInfo[K, N], 0))
	return &rt
}

func (rt *SimpleRT[K, N]) Self() K {
	return rt.self
}

func (rt *SimpleRT[K, N]) KeySize() int {
	return rt.self.BitLen() * 8
}

func (rt *SimpleRT[K, N]) BucketSize() int {
	return rt.bucketSize
}

func (rt *SimpleRT[K, N]) BucketIdForKey(kadId K) (int, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.bucketIdForKey(kadId)
}

// bucketIdForKey must only be called while rt.mu is held
func (rt *SimpleRT[K, N]) bucketIdForKey(kadId K) (int, error) {
	bid := rt.self.CommonPrefixLength(kadId)
	nBuckets := len(rt.buckets)
	if bid >= nBuckets {
		bid = nBuckets - 1
	}
	return bid, nil
}

func (rt *SimpleRT[K, N]) SizeOfBucket(bucketId int) int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.buckets[bucketId])
}

func (rt *SimpleRT[K, N]) AddNode(id N) bool {
	return rt.addPeer(id.Key(), id)
}

func (rt *SimpleRT[K, N]) addPeer(kadId K, id N) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// no need to check the error here, it's already been checked in keyError
	bid, _ := rt.bucketIdForKey(kadId)

	lastBucketId := len(rt.buckets) - 1

	if rt.alreadyInBucket(kadId, bid) {
		// discard new peer
		return false
	}

	if bid < lastBucketId {
		// new peer doesn't belong in last bucket
		if len(rt.buckets[bid]) >= rt.bucketSize {
			// bucket is full, discard new peer
			return false
		}

		// add new peer to bucket
		rt.buckets[bid] = append(rt.buckets[bid], peerInfo[K, N]{id, kadId})
		return true
	}
	if len(rt.buckets[lastBucketId]) < rt.bucketSize {
		// last bucket is not full, add new peer
		rt.buckets[lastBucketId] = append(rt.buckets[lastBucketId], peerInfo[K, N]{id, kadId})
		return true
	}
	// last bucket is full, try to split it
	for len(rt.buckets[lastBucketId]) == rt.bucketSize {
		// farBucket contains peers with a CPL matching lastBucketId
		farBucket := make([]peerInfo[K, N], 0)
		// closeBucket contains peers with a CPL higher than lastBucketId
		closeBucket := make([]peerInfo[K, N], 0)

		for _, p := range rt.buckets[lastBucketId] {
			if p.kadId.CommonPrefixLength(rt.self) == lastBucketId {
				farBucket = append(farBucket, p)
			} else {
				closeBucket = append(closeBucket, p)
			}
		}
		if len(farBucket) == rt.bucketSize &&
			rt.self.CommonPrefixLength(kadId) == lastBucketId {
			// if all peers in the last bucket have the CPL matching this bucket,
			// don't split it and discard the new peer
			return false
		}
		// replace last bucket with farBucket
		rt.buckets[lastBucketId] = farBucket
		// add closeBucket as a new bucket
		rt.buckets = append(rt.buckets, closeBucket)

		lastBucketId++
	}

	newBid, _ := rt.bucketIdForKey(kadId)
	// add new peer to appropraite bucket
	rt.buckets[newBid] = append(rt.buckets[newBid], peerInfo[K, N]{id, kadId})
	// span.AddEvent("peer added to bucket " + strconv.Itoa(newBid))
	return true
}

func (rt *SimpleRT[K, N]) alreadyInBucket(kadId K, bucketId int) bool {
	for _, p := range rt.buckets[bucketId] {
		// error already checked in keyError by the caller
		if key.Equal(kadId, p.kadId) {
			return true
		}
	}
	return false
}

func (rt *SimpleRT[K, N]) RemoveKey(kadId K) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	bid, _ := rt.bucketIdForKey(kadId)
	for i, p := range rt.buckets[bid] {
		if key.Equal(kadId, p.kadId) {
			// remove peer from bucket
			rt.buckets[bid][i] = rt.buckets[bid][len(rt.buckets[bid])-1]
			rt.buckets[bid] = rt.buckets[bid][:len(rt.buckets[bid])-1]
			return true
		}
	}
	return false
}

func (rt *SimpleRT[K, N]) GetNode(kadId K) (N, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	bid, _ := rt.bucketIdForKey(kadId)
	for _, p := range rt.buckets[bid] {
		if key.Equal(kadId, p.kadId) {
			return p.id, true
		}
	}
	var zero N
	return zero, false
}

// TODO: not exactly working as expected
// returns min(n, bucketSize) peers from the bucket matching the given key
func (rt *SimpleRT[K, N]) NearestNodes(kadId K, n int) []N {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	bid, _ := rt.bucketIdForKey(kadId)

	var peers []peerInfo[K, N]
	// TODO: optimize this
	if len(rt.buckets[bid]) == n {
		peers = make([]peerInfo[K, N], len(rt.buckets[bid]))
		copy(peers, rt.buckets[bid])
	} else {
		peers = make([]peerInfo[K, N], 0)
		for i := 0; i < len(rt.buckets); i++ {
			for _, p := range rt.buckets[i] {
				if !key.Equal(rt.self, p.kadId) {
					peers = append(peers, p)
				}
			}
		}
	}

	sort.SliceStable(peers, func(i, j int) bool {
		distI := peers[i].kadId.Xor(kadId)
		distJ := peers[j].kadId.Xor(kadId)

		cmp := distI.Compare(distJ)
		if cmp != 0 {
			return cmp < 0
		}
		return false
	})
	pids := make([]N, min(n, len(peers)))
	for i := 0; i < min(n, len(peers)); i++ {
		pids[i] = peers[i].id
	}

	return pids
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Cpl returns the longest common prefix length the supplied key shares with the table's key.
func (rt *SimpleRT[K, N]) Cpl(kk K) int {
	return rt.self.CommonPrefixLength(kk)
}

// CplSize returns the number of nodes in the table whose longest common prefix with the table's key is of length cpl.
func (rt *SimpleRT[K, N]) CplSize(cpl int) int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	bid := cpl // cpl is simply the bucket id
	nBuckets := len(rt.buckets)
	if bid >= nBuckets {
		bid = nBuckets - 1
	}

	return len(rt.buckets[bid])
}
