package trie

import (
	"testing"

	"github.com/plprobelab/go-kademlia/key"
)

func FromKeys[T any](kks []key.KadKey) (*Trie[T], error) {
	t := New[T]()
	var err error
	for _, kk := range kks {
		t, err = Add(t, kk)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func TestInsertRemove(t *testing.T) {
	r := New[any]()
	testSeq(r, t)
	testSeq(r, t)
}

func testSeq[T any](tr *Trie[T], t *testing.T) {
	var err error
	for _, s := range testInsertSeq {
		tr, err = Add(tr, s.key)
		if err != nil {
			t.Fatalf("unexpected error during add: %v", err)
		}
		depth, found, err := Find(tr, s.key)
		if err != nil {
			t.Fatalf("unexpected error during find: %v", err)
		}
		if !found {
			t.Fatalf("key not found: %v", s.key)
		}
		if depth != s.insertedDepth {
			t.Errorf("inserting expected depth %d, got %d", s.insertedDepth, depth)
		}
	}
	for _, s := range testRemoveSeq {
		tr, err = Remove(tr, s.key)
		if err != nil {
			t.Fatalf("unexpected error during remove: %v", err)
		}
		depth, found, err := Find(tr, s.key)
		if err != nil {
			t.Fatalf("unexpected error during find: %v", err)
		}
		if found {
			t.Fatalf("key unexpectedly found: %v", s.key)
		}
		if depth != s.reachedDepth {
			t.Errorf("removing key %q expected depth %d, got %d", s.key, s.reachedDepth, depth)
		}
	}
}

func TestCopy(t *testing.T) {
	for _, sample := range testAddSamples {
		trie, err := FromKeys[any](sample.Keys)
		if err != nil {
			t.Fatalf("unexpected error during from keys: %v", err)
		}
		copy := trie.Copy()
		if d := copy.CheckInvariant(); d != nil {
			t.Fatalf("trie invariant discrepancy: %v", d)
		}
		if trie == copy {
			t.Errorf("Expected trie copy not to be the same reference as original")
		}
		if !Equal(trie, copy) {
			t.Errorf("Expected tries to be equal, original: %v\n, copy: %v\n", trie, copy)
		}
	}
}

var testInsertSeq = []struct {
	key           key.KadKey
	insertedDepth int
}{
	{key: key.KadKey([]byte{0x00}), insertedDepth: 0},
	{key: key.KadKey([]byte{0x80}), insertedDepth: 1},
	{key: key.KadKey([]byte{0x10}), insertedDepth: 4},
	{key: key.KadKey([]byte{0xc0}), insertedDepth: 2},
	{key: key.KadKey([]byte{0x20}), insertedDepth: 3},
}

var testRemoveSeq = []struct {
	key          key.KadKey
	reachedDepth int
}{
	{key: key.KadKey([]byte{0x00}), reachedDepth: 4},
	{key: key.KadKey([]byte{0x10}), reachedDepth: 3},
	{key: key.KadKey([]byte{0x20}), reachedDepth: 1},
	{key: key.KadKey([]byte{0x80}), reachedDepth: 2},
	{key: key.KadKey([]byte{0xc0}), reachedDepth: 0},
}

// // Verify mutable and immutable add do the same thing.
// func TestMutableAndImmutableAddSame(t *testing.T) {
// 	for _, s := range append(testAddSamples, randomTestAddSamples(100)...) {
// 		mut := New[any]()
// 		immut := New[any]()
// 		for _, k := range s.Keys {
// 			mut.Add(k)
// 			immut = Add(immut, k)
// 		}
// 		if d := mut.CheckInvariant(); d != nil {
// 			t.Fatalf("mutable trie invariant discrepancy: %v", d)
// 		}
// 		if d := immut.CheckInvariant(); d != nil {
// 			t.Fatalf("immutable trie invariant discrepancy: %v", d)
// 		}
// 		if !Equal(mut, immut) {
// 			t.Errorf("mutable trie %v differs from immutable trie %v", mut, immut)
// 		}
// 	}
// }

// func TestAddIsOrderIndependent(t *testing.T) {
// 	for _, s := range append(testAddSamples, randomTestAddSamples(100)...) {
// 		base := New[any]()
// 		for _, k := range s.Keys {
// 			base.Add(k)
// 		}
// 		if d := base.CheckInvariant(); d != nil {
// 			t.Fatalf("base trie invariant discrepancy: %v", d)
// 		}
// 		for j := 0; j < 100; j++ {
// 			perm := rand.Perm(len(s.Keys))
// 			reordered := New[any]()
// 			for i := range s.Keys {
// 				reordered.Add(s.Keys[perm[i]])
// 			}
// 			if d := reordered.CheckInvariant(); d != nil {
// 				t.Fatalf("reordered trie invariant discrepancy: %v", d)
// 			}
// 			if !Equal(base, reordered) {
// 				t.Errorf("trie %v differs from trie %v", base, reordered)
// 			}
// 		}
// 	}
// }

// type testAddSample struct {
// 	Keys []key.KadKey
// }

// var testAddSamples = []*testAddSample{
// 	{Keys: []key.KadKey{key.ByteKey(1), key.ByteKey(3), key.ByteKey(5), key.ByteKey(7), key.ByteKey(11), key.ByteKey(13)}},
// 	{Keys: []key.KadKey{
// 		key.ByteKey(11), key.ByteKey(22), key.ByteKey(23), key.ByteKey(25),
// 		key.ByteKey(27), key.ByteKey(28), key.ByteKey(31), key.ByteKey(32), key.ByteKey(33),
// 	}},
// }

// func randomTestAddSamples(count int) []*testAddSample {
// 	s := make([]*testAddSample, count)
// 	for i := range s {
// 		s[i] = randomTestAddSample(31, 2)
// 	}
// 	return s
// }

// func randomTestAddSample(setSize, keySizeByte int) *testAddSample {
// 	keySet := make([]key.KadKey, setSize)
// 	for i := range keySet {
// 		k := make([]byte, keySizeByte)
// 		rand.Read(k)
// 		keySet[i] = key.BytesKey(k)
// 	}
// 	return &testAddSample{
// 		Keys: keySet,
// 	}
// }
