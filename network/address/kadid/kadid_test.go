package kadid

import (
	"testing"

	"github.com/plprobelab/go-kademlia/key"
)

func TestNodeIDEqual(t *testing.T) {
	k1 := NewKadID(key.KadKey([]byte{1, 2, 3, 4, 5}))
	k2 := NewKadID(key.KadKey([]byte{1, 2, 3, 4, 5}))

	// KadIDs are not directly comparable
	if k1 == k2 {
		t.Errorf("got k1==k2, wanted k1!=k2")
	}

	if !k1.Equal(k2) {
		t.Errorf("got k1.Equal(k2)=false, wanted true")
	}

	var nilKadID *KadID
	if k1.Equal(nilKadID) {
		t.Errorf("got k1.Equal(nilKadID)=true, wanted false")
	}
	if nilKadID.Equal(k1) {
		t.Errorf("got nilKadID.Equal(k1)=true, wanted false")
	}

	n1 := k1.NodeID()
	n2 := k2.NodeID()

	if n1 == n2 {
		t.Errorf("got n1==n2, wanted n1!=n2")
	}

	if !n1.Equal(n2) {
		t.Errorf("got n1.Equal(n2)=false, wanted true")
	}

	if n1.Equal(nilKadID) {
		t.Errorf("got n1.Equal(nilKadID)=true, wanted false")
	}
	if nilKadID.Equal(n1) {
		t.Errorf("got nilKadID.Equal(n1)=true, wanted false")
	}
}
