package simplert

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type peerInfo[K kad.Key[K]] struct {
	id    address.NodeID[K]
	kadId K
}

type SimpleRT[K kad.Key[K]] struct {
	self       K
	buckets    [][]peerInfo[K]
	bucketSize int
}

// var _ routing.Table = (*SimpleRT)(nil)

func New[K kad.Key[K]](self K, bucketSize int) *SimpleRT[K] {
	rt := SimpleRT[K]{
		self:       self,
		buckets:    make([][]peerInfo[K], 0),
		bucketSize: bucketSize,
	}
	// define bucket 0
	rt.buckets = append(rt.buckets, make([]peerInfo[K], 0))
	return &rt
}

func (rt *SimpleRT[K]) Self() K {
	return rt.self
}

func (rt *SimpleRT[K]) KeySize() int {
	return rt.self.BitLen() * 8
}

func (rt *SimpleRT[K]) BucketSize() int {
	return rt.bucketSize
}

func (rt *SimpleRT[K]) BucketIdForKey(kadId K) (int, error) {
	bid := rt.self.CommonPrefixLength(kadId)
	nBuckets := len(rt.buckets)
	if bid >= nBuckets {
		bid = nBuckets - 1
	}
	return bid, nil
}

func (rt *SimpleRT[K]) SizeOfBucket(bucketId int) int {
	return len(rt.buckets[bucketId])
}

func (rt *SimpleRT[K]) AddPeer(ctx context.Context, id address.NodeID[K]) (bool, error) {
	return rt.addPeer(ctx, id.Key(), id)
}

func (rt *SimpleRT[K]) addPeer(ctx context.Context, kadId K, id address.NodeID[K]) (bool, error) {
	_, span := util.StartSpan(ctx, "routing.simple.addPeer", trace.WithAttributes(
		attribute.String("KadID", key.HexString(kadId)),
		attribute.Stringer("PeerID", id),
	))
	defer span.End()

	// no need to check the error here, it's already been checked in keyError
	bid, _ := rt.BucketIdForKey(kadId)

	lastBucketId := len(rt.buckets) - 1

	if rt.alreadyInBucket(kadId, bid) {
		span.AddEvent("peer not added, already in bucket " + strconv.Itoa(bid))
		// discard new peer
		return false, nil
	}

	if bid < lastBucketId {
		// new peer doesn't belong in last bucket
		if len(rt.buckets[bid]) >= rt.bucketSize {
			span.AddEvent("peer not added, bucket " + strconv.Itoa(bid) + " full")
			// bucket is full, discard new peer
			return false, nil
		}

		// add new peer to bucket
		rt.buckets[bid] = append(rt.buckets[bid], peerInfo[K]{id, kadId})
		span.AddEvent("peer added to bucket " + strconv.Itoa(bid))
		return true, nil
	}
	if len(rt.buckets[lastBucketId]) < rt.bucketSize {
		// last bucket is not full, add new peer
		rt.buckets[lastBucketId] = append(rt.buckets[lastBucketId], peerInfo[K]{id, kadId})
		span.AddEvent("peer added to bucket " + strconv.Itoa(lastBucketId))
		return true, nil
	}
	// last bucket is full, try to split it
	for len(rt.buckets[lastBucketId]) == rt.bucketSize {
		// farBucket contains peers with a CPL matching lastBucketId
		farBucket := make([]peerInfo[K], 0)
		// closeBucket contains peers with a CPL higher than lastBucketId
		closeBucket := make([]peerInfo[K], 0)

		span.AddEvent("splitting last bucket (" + strconv.Itoa(lastBucketId) + ")")

		for _, p := range rt.buckets[lastBucketId] {
			if p.kadId.CommonPrefixLength(rt.self) == lastBucketId {
				farBucket = append(farBucket, p)
			} else {
				closeBucket = append(closeBucket, p)
				span.AddEvent(p.id.String() + " moved to new bucket (" +
					strconv.Itoa(lastBucketId+1) + ")")
			}
		}
		if len(farBucket) == rt.bucketSize &&
			rt.self.CommonPrefixLength(kadId) == lastBucketId {
			// if all peers in the last bucket have the CPL matching this bucket,
			// don't split it and discard the new peer
			return false, nil
		}
		// replace last bucket with farBucket
		rt.buckets[lastBucketId] = farBucket
		// add closeBucket as a new bucket
		rt.buckets = append(rt.buckets, closeBucket)

		lastBucketId++
	}

	newBid, _ := rt.BucketIdForKey(kadId)
	// add new peer to appropraite bucket
	rt.buckets[newBid] = append(rt.buckets[newBid], peerInfo[K]{id, kadId})
	span.AddEvent("peer added to bucket " + strconv.Itoa(newBid))
	return true, nil
}

func (rt *SimpleRT[K]) alreadyInBucket(kadId K, bucketId int) bool {
	for _, p := range rt.buckets[bucketId] {
		// error already checked in keyError by the caller
		if key.Equal(kadId, p.kadId) {
			return true
		}
	}
	return false
}

func (rt *SimpleRT[K]) RemoveKey(ctx context.Context, kadId K) (bool, error) {
	_, span := util.StartSpan(ctx, "routing.simple.removeKey", trace.WithAttributes(
		attribute.String("KadID", key.HexString(kadId)),
	))
	defer span.End()

	bid, _ := rt.BucketIdForKey(kadId)
	for i, p := range rt.buckets[bid] {
		if key.Equal(kadId, p.kadId) {
			// remove peer from bucket
			rt.buckets[bid][i] = rt.buckets[bid][len(rt.buckets[bid])-1]
			rt.buckets[bid] = rt.buckets[bid][:len(rt.buckets[bid])-1]

			span.AddEvent(fmt.Sprint(p.id.String(), "removed from bucket", bid))
			return true, nil
		}
	}
	// peer not found in the routing table
	span.AddEvent(fmt.Sprint("peer not found in bucket", bid))
	return false, nil
}

func (rt *SimpleRT[K]) Find(ctx context.Context, kadId K) (address.NodeID[K], error) {
	_, span := util.StartSpan(ctx, "routing.simple.find", trace.WithAttributes(
		attribute.String("KadID", key.HexString(kadId)),
	))
	defer span.End()

	bid, _ := rt.BucketIdForKey(kadId)
	for _, p := range rt.buckets[bid] {
		if key.Equal(kadId, p.kadId) {
			return p.id, nil
		}
	}
	return nil, nil
}

// TODO: not exactly working as expected
// returns min(n, bucketSize) peers from the bucket matching the given key
func (rt *SimpleRT[K]) NearestPeers(ctx context.Context, kadId K, n int) ([]address.NodeID[K], error) {
	_, span := util.StartSpan(ctx, "routing.simple.nearestPeers", trace.WithAttributes(
		attribute.String("KadID", key.HexString(kadId)),
		attribute.Int("n", int(n)),
	))
	defer span.End()

	bid, _ := rt.BucketIdForKey(kadId)

	var peers []peerInfo[K]
	// TODO: optimize this
	if len(rt.buckets[bid]) == n {
		peers = make([]peerInfo[K], len(rt.buckets[bid]))
		copy(peers, rt.buckets[bid])
	} else {
		peers = make([]peerInfo[K], 0)
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
	pids := make([]address.NodeID[K], min(n, len(peers)))
	for i := 0; i < min(n, len(peers)); i++ {
		pids[i] = peers[i].id
	}

	return pids, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
