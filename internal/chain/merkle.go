package chain

import (
	"crypto/sha256"
	"encoding/hex"
)

// MerkleRoot of transaction ids (hex strings).
func MerkleRoot(txIDs []string) string {
	if len(txIDs) == 0 {
		z := sha256.Sum256(nil)
		return hex.EncodeToString(z[:])
	}
	level := make([][32]byte, len(txIDs))
	for i, id := range txIDs {
		level[i] = sha256.Sum256([]byte(id))
	}
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([][32]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			var cat [64]byte
			copy(cat[:32], level[i][:])
			copy(cat[32:], level[i+1][:])
			next = append(next, sha256.Sum256(cat[:]))
		}
		level = next
	}
	return hex.EncodeToString(level[0][:])
}

// MerkleRootFromTxs convenience.
func MerkleRootFromTxs(txs []Transaction) string {
	ids := make([]string, len(txs))
	for i := range txs {
		ids[i] = txs[i].TxID()
	}
	return MerkleRoot(ids)
}
