package trie

import (
	"math/rand"
	"testing"

	"github.com/plprobelab/go-kademlia/key"
)

func trieFromKeys[T any](kks []key.KadKey) (*Trie[T], error) {
	t := New[T]()
	var err error
	for _, kk := range kks {
		var v T
		t, err = Add(t, kk, v)
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
		var v T
		tr, err = Add(tr, s.key, v)
		if err != nil {
			t.Fatalf("unexpected error during add: %v", err)
		}
		found, depth := Locate(tr, s.key)
		if !found {
			t.Fatalf("key not found: %v", s.key)
		}
		if depth != s.insertedDepth {
			t.Errorf("inserting expected depth %d, got %d", s.insertedDepth, depth)
		}
		if d := CheckInvariant(tr); d != nil {
			t.Fatalf("trie invariant discrepancy: %v", d)
		}
	}
	for _, s := range testRemoveSeq {
		tr, err = Remove(tr, s.key)
		if err != nil {
			t.Fatalf("unexpected error during remove: %v", err)
		}
		found, _ := Locate(tr, s.key)
		if found {
			t.Fatalf("key unexpectedly found: %v", s.key)
		}
		if d := CheckInvariant(tr); d != nil {
			t.Fatalf("trie invariant discrepancy: %v", d)
		}
	}
}

func TestCopy(t *testing.T) {
	for _, sample := range testAddSamples {
		trie, err := trieFromKeys[any](sample.Keys)
		if err != nil {
			t.Fatalf("unexpected error during from keys: %v", err)
		}
		copy := trie.Copy()
		if d := CheckInvariant(copy); d != nil {
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

func TestAddIsOrderIndependent(t *testing.T) {
	for _, s := range append(testAddSamples, randomTestAddSamples(100)...) {
		base := New[any]()
		for _, k := range s.Keys {
			base, _ = Add(base, k, nil)
		}
		if d := CheckInvariant(base); d != nil {
			t.Fatalf("base trie invariant discrepancy: %v", d)
		}
		for j := 0; j < 100; j++ {
			perm := rand.Perm(len(s.Keys))
			reordered := New[any]()
			for i := range s.Keys {
				reordered, _ = Add(reordered, s.Keys[perm[i]], nil)
			}
			if d := CheckInvariant(reordered); d != nil {
				t.Fatalf("reordered trie invariant discrepancy: %v", d)
			}
			if !Equal(base, reordered) {
				t.Errorf("trie %v differs from trie %v", base, reordered)
			}
		}
	}
}

type testAddSample struct {
	Keys []key.KadKey
}

var testAddSamples = []*testAddSample{
	{Keys: []key.KadKey{[]byte{1}, []byte{3}, []byte{5}, []byte{7}, []byte{11}, []byte{13}}},
	{Keys: []key.KadKey{
		[]byte{11},
		[]byte{22},
		[]byte{23},
		[]byte{25},
		[]byte{27},
		[]byte{28},
		[]byte{31},
		[]byte{32},
		[]byte{33},
	}},
}

func randomTestAddSamples(count int) []*testAddSample {
	s := make([]*testAddSample, count)
	for i := range s {
		s[i] = randomTestAddSample(31, 2)
	}
	return s
}

func randomTestAddSample(setSize, keySizeByte int) *testAddSample {
	keySet := make([]key.KadKey, setSize)
	for i := range keySet {
		k := make([]byte, keySizeByte)
		rand.Read(k)
		keySet[i] = key.KadKey(k)
	}
	return &testAddSample{
		Keys: keySet,
	}
}

type InvariantDiscrepancy struct {
	Reason            string
	PathToDiscrepancy string
	KeyAtDiscrepancy  string
}

// CheckInvariant panics of the trie does not meet its invariant.
func CheckInvariant[T any](tr *Trie[T]) *InvariantDiscrepancy {
	return checkInvariant(tr, 0, nil)
}

func checkInvariant[T any](tr *Trie[T], depth int, pathSoFar *triePath) *InvariantDiscrepancy {
	switch {
	case tr.IsEmptyLeaf():
		return nil
	case tr.IsNonEmptyLeaf():
		if !pathSoFar.matchesKey(tr.Key) {
			return &InvariantDiscrepancy{
				Reason:            "key found at invalid location in trie",
				PathToDiscrepancy: pathSoFar.BitString(),
				KeyAtDiscrepancy:  tr.Key.BitString(),
			}
		}
		return nil
	default:
		if !tr.HasKey() {
			b0, b1 := tr.Branch[0], tr.Branch[1]
			if d0 := checkInvariant(b0, depth+1, pathSoFar.Push(0)); d0 != nil {
				return d0
			}
			if d1 := checkInvariant(b1, depth+1, pathSoFar.Push(1)); d1 != nil {
				return d1
			}
			switch {
			case b0.IsEmptyLeaf() && b1.IsEmptyLeaf():
				return &InvariantDiscrepancy{
					Reason:            "intermediate node with two empty leaves",
					PathToDiscrepancy: pathSoFar.BitString(),
					KeyAtDiscrepancy:  "none",
				}
			case b0.IsEmptyLeaf() && b1.IsNonEmptyLeaf():
				return &InvariantDiscrepancy{
					Reason:            "intermediate node with one empty leaf/0",
					PathToDiscrepancy: pathSoFar.BitString(),
					KeyAtDiscrepancy:  "none",
				}
			case b0.IsNonEmptyLeaf() && b1.IsEmptyLeaf():
				return &InvariantDiscrepancy{
					Reason:            "intermediate node with one empty leaf/1",
					PathToDiscrepancy: pathSoFar.BitString(),
					KeyAtDiscrepancy:  "none",
				}
			default:
				return nil
			}
		} else {
			return &InvariantDiscrepancy{
				Reason:            "intermediate node with a key",
				PathToDiscrepancy: pathSoFar.BitString(),
				KeyAtDiscrepancy:  tr.Key.BitString(),
			}
		}
	}
}

type triePath struct {
	parent *triePath
	bit    byte
}

func (p *triePath) Push(bit byte) *triePath {
	return &triePath{parent: p, bit: bit}
}

func (p *triePath) RootPath() []byte {
	if p == nil {
		return nil
	} else {
		return append(p.parent.RootPath(), p.bit)
	}
}

func (p *triePath) matchesKey(k key.KadKey) bool {
	ok, _ := p.walk(k, 0)
	return ok
}

func (p *triePath) walk(k key.KadKey, depthToLeaf int) (ok bool, depthToRoot int) {
	if p == nil {
		return true, 0
	} else {
		parOk, parDepthToRoot := p.parent.walk(k, depthToLeaf+1)
		return k.BitAt(parDepthToRoot) == int(p.bit) && parOk, parDepthToRoot + 1
	}
}

func (p *triePath) BitString() string {
	return p.bitString(0)
}

func (p *triePath) bitString(depthToLeaf int) string {
	if p == nil {
		return ""
	} else {
		switch {
		case p.bit == 0:
			return p.parent.bitString(depthToLeaf+1) + "0"
		case p.bit == 1:
			return p.parent.bitString(depthToLeaf+1) + "1"
		default:
			panic("bit digit > 1")
		}
	}
}
