package chain

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
)

func unmarshalJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

func sha256Bytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func hexEncode(b []byte) string { return hex.EncodeToString(b) }

// nodeIDFromPubBytes matches protocol.NodeIDFromPub / engine.nodeIDFromPub (SHA1 hex).
func nodeIDFromPubBytes(pub []byte) string {
	h := sha1.Sum(pub)
	return hex.EncodeToString(h[:])
}

// safeAddUint64 returns a+b or false on overflow.
func safeAddUint64(a, b uint64) (uint64, bool) {
	if b > 0 && a > math.MaxUint64-b {
		return 0, false
	}
	return a + b, true
}
