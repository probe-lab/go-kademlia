package trie

import (
	"github.com/plprobelab/go-kademlia/key"
	"math/rand"
	"testing"
)

// Verify mutable and immutable add do the same thing.
func TestMutableAndImmutableAddSame(t *testing.T) {
	for _, s := range append(testAddSamples, randomTestAddSamples[key.Bytes](100)...) {
		mut := New[any, key.Bytes]()
		immut := New[any, key.Bytes]()
		for _, k := range s.Keys {
			mut.Add(k)
			immut = Add(immut, k)
		}
		if d := mut.CheckInvariant(); d != nil {
			t.Fatalf("mutable trie invariant discrepancy: %v", d)
		}
		if d := immut.CheckInvariant(); d != nil {
			t.Fatalf("immutable trie invariant discrepancy: %v", d)
		}
		if !Equal(mut, immut) {
			t.Errorf("mutable trie %v differs from immutable trie %v", mut, immut)
		}
	}
}

func TestAddIsOrderIndependent(t *testing.T) {
	for _, s := range append(testAddSamples, randomTestAddSamples[key.Bytes](100)...) {
		base := New[any, key.Bytes]()
		for _, k := range s.Keys {
			base.Add(k)
		}
		if d := base.CheckInvariant(); d != nil {
			t.Fatalf("base trie invariant discrepancy: %v", d)
		}
		for j := 0; j < 100; j++ {
			perm := rand.Perm(len(s.Keys))
			reordered := New[any, key.Bytes]()
			for i := range s.Keys {
				reordered.Add(s.Keys[perm[i]].Key())
			}
			if d := reordered.CheckInvariant(); d != nil {
				t.Fatalf("reordered trie invariant discrepancy: %v", d)
			}
			if !Equal(base, reordered) {
				t.Errorf("trie %v differs from trie %v", base, reordered)
			}
		}
	}
}

type testAddSample[T key.Kademlia[T]] struct {
	Keys []T
}

var testAddSamples = []*testAddSample[key.Bytes]{
	{Keys: []key.Bytes{{1}, {3}, {5}, {7}, {11}, {13}}},
	{Keys: []key.Bytes{
		{11}, {22}, {23}, {25},
		{27}, {28}, {31}, {32}, {33},
	}},
}

func randomTestAddSamples[T key.Kademlia[T]](count int) []*testAddSample[key.Bytes] {
	s := make([]*testAddSample[key.Bytes], count)
	for i := range s {
		s[i] = randomTestAddSample(31, 2)
	}
	return s
}

func randomTestAddSample(setSize, keySizeByte int) *testAddSample[key.Bytes] {
	keySet := make([]key.Bytes, setSize)
	for i := range keySet {
		k := make([]byte, keySizeByte)
		rand.Read(k)
		keySet[i] = key.Bytes(k)
	}
	return &testAddSample[key.Bytes]{
		Keys: keySet,
	}
}
