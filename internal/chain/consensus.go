package chain

import (
	"encoding/hex"
	"os"
	"time"

	"github.com/YizeHe/MSP-project/internal/pow"
)

// ValidateBlock checks PoW, links, merkle, size (not full state).
func ValidateBlock(b *Block, prev *BlockHeader) error {
	if b.Size() > MaxBlockBytes {
		return errBlockTooLarge
	}
	if MerkleRootFromTxs(b.Txs) != b.Header.MerkleRoot {
		return errBadMerkle
	}
	if prev == nil {
		// genesis
		if b.Header.Height != 0 {
			return errBadHeight
		}
		return nil
	}
	if b.Header.PrevBlock != prev.HashHex() {
		return errBadPrev
	}
	if b.Header.Height != prev.Height+1 {
		return errBadHeight
	}
	// PoW on header material
	if b.Header.Height > 0 {
		mat := headerPoWMaterial(&b.Header)
		if !pow.Verify(mat, b.Header.Nonce, b.Header.Difficulty, b.Header.PoWHash) {
			return errBadPoW
		}
	}
	return nil
}

func headerPoWMaterial(h *BlockHeader) []byte {
	// exclude nonce and pow_hash
	cp := *h
	cp.Nonce = 0
	cp.PoWHash = ""
	raw, _ := jsonMarshal(cp)
	return raw
}

// MineBlock fills nonce until PoW valid.
func MineBlock(b *Block, bits int) time.Duration {
	if bits < 1 {
		bits = 1
	}
	b.Header.Difficulty = bits
	mat := headerPoWMaterial(&b.Header)
	nonce, hash, took := pow.Mine(mat, bits)
	b.Header.Nonce = nonce
	b.Header.PoWHash = hash
	return took
}

// TargetInterval block time.
func TargetInterval() time.Duration {
	if os.Getenv("MSP_CHAIN_FAST") == "1" || os.Getenv("MSP_POW_FAST") == "1" {
		return FastBlockInterval
	}
	return BlockInterval
}

// DefaultMineBits for blocks in lab vs prod.
func DefaultMineBits() int {
	if os.Getenv("MSP_CHAIN_FAST") == "1" || os.Getenv("MSP_POW_FAST") == "1" {
		return 2
	}
	return 4
}

func jsonMarshal(v any) ([]byte, error) {
	// local to avoid import cycle issues — use encoding/json
	return marshalJSON(v)
}

// HashBytes helper.
func HashBytes(b []byte) string {
	// unused placeholder
	return hex.EncodeToString(b)
}
