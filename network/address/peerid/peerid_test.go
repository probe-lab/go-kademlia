package peerid

import (
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"
)

func TestNodeIDEqual(t *testing.T) {
	buf := make([]byte, 16)
	rand.Read(buf)
	h, _ := mh.Sum(buf, mh.SHA2_256, -1)

	n1 := NewPeerID(peer.ID(h)).NodeID()
	n2 := NewPeerID(peer.ID(h)).NodeID()

	if n1 == n2 {
		t.Errorf("got n1==n2, wanted n1!=n2")
	}

	if !n1.Equal(n2) {
		t.Errorf("got n1.Equal(n2)=false, wanted true")
	}

	nilNodeID := (*PeerID)(nil).NodeID()
	if n1.Equal(nilNodeID) {
		t.Errorf("got n1.Equal(nilNodeID)=true, wanted false")
	}
	if nilNodeID.Equal(n1) {
		t.Errorf("got nilNodeID.Equal(n1)=true, wanted false")
	}
}
