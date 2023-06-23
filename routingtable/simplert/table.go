package simplert

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable"
	"github.com/libp2p/go-libp2p-kad-dht/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type peerInfo struct {
	id    address.NodeID
	kadId key.KadKey
}

type SimpleRT struct {
	self       key.KadKey
	buckets    [][]peerInfo
	bucketSize int
}

var _ routingtable.RoutingTable = (*SimpleRT)(nil)

func NewSimpleRT(self key.KadKey, bucketSize int) *SimpleRT {
	rt := SimpleRT{
		self:       self,
		buckets:    make([][]peerInfo, 0),
		bucketSize: bucketSize,
	}
	// define bucket 0
	rt.buckets = append(rt.buckets, make([]peerInfo, 0))
	return &rt
}

func (rt *SimpleRT) keyError(kadId key.KadKey) error {
	if rt.self.Size() != kadId.Size() {
		return key.ErrInvalidKey(rt.self.Size())
	}
	return nil
}

func (rt *SimpleRT) Self() key.KadKey {
	return rt.self
}

func (rt *SimpleRT) KeySize() int {
	return rt.self.Size()
}

func (rt *SimpleRT) BucketSize() int {
	return rt.bucketSize
}

func (rt *SimpleRT) BucketIdForKey(kadId key.KadKey) (int, error) {
	bid, err := rt.self.CommonPrefixLength(kadId)
	if err != nil {
		return 0, err
	}
	nBuckets := len(rt.buckets)
	if bid >= nBuckets {
		bid = nBuckets - 1
	}
	return bid, nil
}

func (rt *SimpleRT) SizeOfBucket(bucketId int) int {
	return len(rt.buckets[bucketId])
}

func (rt *SimpleRT) AddPeer(ctx context.Context, id address.NodeID) (bool, error) {
	return rt.addPeer(ctx, id.Key(), id)
}

func (rt *SimpleRT) addPeer(ctx context.Context, kadId key.KadKey, id address.NodeID) (bool, error) {
	_, span := util.StartSpan(ctx, "simplert.addPeer", trace.WithAttributes(
		attribute.String("KadID", kadId.String()),
		attribute.Stringer("PeerID", id),
	))
	defer span.End()

	if err := rt.keyError(kadId); err != nil {
		span.RecordError(err)
		return false, err
	}

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
		rt.buckets[bid] = append(rt.buckets[bid], peerInfo{id, kadId})
		span.AddEvent("peer added to bucket " + strconv.Itoa(bid))
		return true, nil
	}
	if len(rt.buckets[lastBucketId]) < rt.bucketSize {
		// last bucket is not full, add new peer
		rt.buckets[lastBucketId] = append(rt.buckets[lastBucketId], peerInfo{id, kadId})
		span.AddEvent("peer added to bucket " + strconv.Itoa(lastBucketId))
		return true, nil
	}
	// last bucket is full, try to split it
	for len(rt.buckets[lastBucketId]) == rt.bucketSize {
		// farBucket contains peers with a CPL matching lastBucketId
		farBucket := make([]peerInfo, 0)
		// closeBucket contains peers with a CPL higher than lastBucketId
		closeBucket := make([]peerInfo, 0)

		span.AddEvent("splitting last bucket (" + strconv.Itoa(lastBucketId) + ")")

		for _, p := range rt.buckets[lastBucketId] {
			if cpl, _ := p.kadId.CommonPrefixLength(rt.self); cpl == lastBucketId {
				farBucket = append(farBucket, p)
			} else {
				closeBucket = append(closeBucket, p)
				span.AddEvent(p.id.String() + " moved to new bucket (" +
					strconv.Itoa(lastBucketId+1) + ")")
			}
		}
		cpl, _ := rt.self.CommonPrefixLength(kadId)
		if len(farBucket) == rt.bucketSize && cpl == lastBucketId {
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
	rt.buckets[newBid] = append(rt.buckets[newBid], peerInfo{id, kadId})
	span.AddEvent("peer added to bucket " + strconv.Itoa(newBid))
	return true, nil
}

func (rt *SimpleRT) alreadyInBucket(kadId key.KadKey, bucketId int) bool {
	for _, p := range rt.buckets[bucketId] {
		// error already checked in keyError by the caller
		c, _ := kadId.Compare(p.kadId)
		if c == 0 {
			return true
		}
	}
	return false
}

func (rt *SimpleRT) RemoveKey(ctx context.Context, kadId key.KadKey) (bool, error) {
	_, span := util.StartSpan(ctx, "simplert.removePeer", trace.WithAttributes(
		attribute.String("KadID", kadId.String()),
	))
	defer span.End()

	if err := rt.keyError(kadId); err != nil {
		span.RecordError(err)
		return false, err
	}

	bid, _ := rt.BucketIdForKey(kadId)
	for i, p := range rt.buckets[bid] {
		c, _ := kadId.Compare(p.kadId)
		if c == 0 {
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

func (rt *SimpleRT) Find(ctx context.Context, kadId key.KadKey) (address.NodeID, error) {
	_, span := util.StartSpan(ctx, "simplert.removePeer", trace.WithAttributes(
		attribute.String("KadID", kadId.String()),
	))
	defer span.End()

	if err := rt.keyError(kadId); err != nil {
		span.RecordError(err)
		return nil, err
	}

	bid, _ := rt.BucketIdForKey(kadId)
	for _, p := range rt.buckets[bid] {
		c, _ := kadId.Compare(p.kadId)
		if c == 0 {
			return p.id, nil
		}
	}
	return nil, nil
}

// TODO: not exactly working as expected
// returns min(n, bucketSize) peers from the bucket matching the given key
func (rt *SimpleRT) NearestPeers(ctx context.Context, kadId key.KadKey, n int) ([]address.NodeID, error) {
	_, span := util.StartSpan(ctx, "simplert.nearestPeers", trace.WithAttributes(
		attribute.String("KadID", kadId.String()),
		attribute.Int("n", int(n)),
	))
	defer span.End()

	if err := rt.keyError(kadId); err != nil {
		span.RecordError(err)
		return nil, err
	}

	bid, _ := rt.BucketIdForKey(kadId)

	var peers []peerInfo
	// TODO: optimize this
	if len(rt.buckets[bid]) == n {
		peers = make([]peerInfo, len(rt.buckets[bid]))
		copy(peers, rt.buckets[bid])
	} else {
		peers = make([]peerInfo, 0)
		for i := 0; i < len(rt.buckets); i++ {
			for _, p := range rt.buckets[i] {
				c, _ := rt.self.Compare(p.kadId)
				if c != 0 {
					peers = append(peers, p)
				}
			}
		}
	}

	sort.SliceStable(peers, func(i, j int) bool {
		for k := 0; k < rt.KeySize(); k++ {
			distI := peers[i].kadId[k] ^ kadId[k]
			distJ := peers[j].kadId[k] ^ kadId[k]
			if distI != distJ {
				return distI < distJ
			}
		}
		return false
	})
	pids := make([]address.NodeID, min(n, len(peers)))
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
