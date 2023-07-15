package stringid

import (
	"testing"
)

func TestNodeIDEqual(t *testing.T) {
	n1 := NewStringID("abcdef").NodeID()
	n2 := NewStringID("abcdef").NodeID()

	// StringIDs are directly comparable
	if n1 != n2 {
		t.Errorf("got n1!=n2, wanted n1==n2")
	}

	if !n1.Equal(n2) {
		t.Errorf("got n1.Equal(n2)=false, wanted true")
	}
}
