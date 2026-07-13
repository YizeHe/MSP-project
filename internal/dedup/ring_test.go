package dedup

import "testing"

func TestRingDedup(t *testing.T) {
	r := New()
	k := Key{SHA1: "a", SHA512: "b"}
	if !r.Add(k) {
		t.Fatal("first add should succeed")
	}
	if r.Add(k) {
		t.Fatal("duplicate should fail")
	}
	if !r.Seen(k) {
		t.Fatal("should be seen")
	}
}
