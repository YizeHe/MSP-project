package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func unmarshalJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

func sha256Bytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func hexEncode(b []byte) string { return hex.EncodeToString(b) }
