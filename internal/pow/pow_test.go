package pow

import "testing"

func TestMineAndVerify(t *testing.T) {
	mat := []byte("msp-test-material")
	nonce, h, _ := Mine(mat, 2)
	if !Verify(mat, nonce, 2, h) {
		t.Fatalf("verify failed nonce=%d hash=%s", nonce, h)
	}
	if Verify(mat, nonce+1, 2, h) {
		t.Fatal("wrong nonce should fail")
	}
}
