package trie

import (
	"github.com/plprobelab/go-kademlia/key"
	"math/rand"
	"testing"
)

func TestImmutableRemoveIsImmutable(t *testing.T) {
	for _, keySet := range testAddSamples {
		trie := FromKeys[any](keySet.Keys)
		for _, key := range keySet.Keys {
			updated := Remove(trie, key)
			if Equal(trie, updated) {
				t.Fatalf("immuatble remove should not mutate trie, original: %v, updated: %v", trie, updated)
			}
			trie = updated
		}
	}
}

func TestMutableAndImmutableRemoveSame(t *testing.T) {
	for _, keySet := range append(testAddSamples, randomTestAddSamples[key.Bytes](100)...) {
		mut := FromKeys[any](keySet.Keys)
		immut := FromKeys[any](keySet.Keys)

		for _, k := range keySet.Keys {
			mut.Remove(k)
			immut = Remove(immut, k)
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
}

func TestRemoveIsOrderIndependent(t *testing.T) {
	for _, keySet := range append(testAddSamples, randomTestAddSamples[key.Bytes](100)...) {
		mut := FromKeys[any](keySet.Keys)
		immut := FromKeys[any](keySet.Keys)

		for j := 0; j < 100; j++ {
			perm := rand.Perm(len(keySet.Keys))
			for _, idx := range perm {
				mut.Remove(keySet.Keys[idx])
				immut = Remove(immut, keySet.Keys[idx])

				if d := immut.CheckInvariant(); d != nil {
					t.Fatalf("trie invariant discrepancy: %v", d)
				}
				if !Equal(mut, immut) {
					t.Errorf("trie %v differs from trie %v", mut, immut)
				}
			}
		}
	}
}

func TestRemoveReturnsOriginalWhenNoKeyRemoved(t *testing.T) {
	trie := FromKeys[any](testAddSamples[0].Keys)

	result := Remove(trie, key.Bytes{2})
	if trie != result {
		t.Fatalf("Remove should return original trie when no key was removed")
	}
}
