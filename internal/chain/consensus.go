package chain

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/YizeHe/MSP-project/internal/pow"
)

// RequiredDifficulty is the consensus PoW difficulty for a block height.
// Independent of local MSP_*_FAST env — those only affect miner tick rate / DTN message PoW.
func RequiredDifficulty(height uint64) int {
	if height == 0 {
		return 1 // genesis fixed
	}
	return ConsensusMinDifficulty
}

// ValidateBlock checks PoW, links, merkle, size, consensus difficulty (not full state).
func ValidateBlock(b *Block, prev *BlockHeader) error {
	if b.Size() > MaxBlockBytes {
		return errBlockTooLarge
	}
	if MerkleRootFromTxs(b.Txs) != b.Header.MerkleRoot {
		return errBadMerkle
	}
	if prev == nil {
		// genesis — must match frozen mainnet
		if b.Header.Height != 0 {
			return errBadHeight
		}
		return ValidateMainnetGenesis(b)
	}
	if b.Header.PrevBlock != prev.HashHex() {
		return errBadPrev
	}
	if b.Header.Height != prev.Height+1 {
		return errBadHeight
	}
	// Consensus difficulty: miner cannot advertise an easier target
	need := RequiredDifficulty(b.Header.Height)
	if b.Header.Difficulty < need {
		return fmt.Errorf("%w: got %d need >= %d", errBadDifficulty, b.Header.Difficulty, need)
	}
	// PoW must satisfy the difficulty claimed in the header (and thus >= network min)
	mat := headerPoWMaterial(&b.Header)
	if !pow.Verify(mat, b.Header.Nonce, b.Header.Difficulty, b.Header.PoWHash) {
		return errBadPoW
	}
	// Coinbase schedule (if present) must match consensus reward
	if err := validateCoinbaseInBlock(b); err != nil {
		return err
	}
	return nil
}

func validateCoinbaseInBlock(b *Block) error {
	if b.Header.Height == 0 {
		return nil
	}
	want := BlockReward(b.Header.Height)
	var seen int
	for i := range b.Txs {
		if b.Txs[i].Type != TxCoinbase {
			continue
		}
		seen++
		if seen > 1 {
			return fmt.Errorf("%w: multiple coinbase", errBadCoinbase)
		}
		if i != 0 {
			return fmt.Errorf("%w: coinbase must be first tx", errBadCoinbase)
		}
		var d CoinbaseData
		if err := jsonUnmarshal(b.Txs[i].Data, &d); err != nil {
			return errBadCoinbase
		}
		// Amount may be lower only when pool is smaller — full check in state with pool;
		// here enforce upper bound by schedule (cannot mint above BlockReward).
		if d.Amount > want {
			return fmt.Errorf("%w: amount %d > schedule %d", errBadCoinbase, d.Amount, want)
		}
		if d.Height != 0 && d.Height != b.Header.Height {
			return fmt.Errorf("%w: height field mismatch", errBadCoinbase)
		}
	}
	// height>0 blocks should include coinbase when reward > 0 (pool may be empty later)
	if want > 0 && seen == 0 {
		return fmt.Errorf("%w: missing coinbase", errBadCoinbase)
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

// MineBlock fills nonce until PoW valid at the given difficulty bits.
func MineBlock(b *Block, bits int) time.Duration {
	need := RequiredDifficulty(b.Header.Height)
	if bits < need {
		bits = need
	}
	b.Header.Difficulty = bits
	mat := headerPoWMaterial(&b.Header)
	nonce, hash, took := pow.Mine(mat, bits)
	b.Header.Nonce = nonce
	b.Header.PoWHash = hash
	return took
}

// TargetInterval is local miner loop cadence only (not a consensus rule).
// MSP_CHAIN_FAST / MSP_POW_FAST speed up how often THIS node tries to mine,
// not the difficulty other nodes will accept.
func TargetInterval() time.Duration {
	if os.Getenv("MSP_CHAIN_FAST") == "1" || os.Getenv("MSP_POW_FAST") == "1" {
		return FastBlockInterval
	}
	return BlockInterval
}

// DefaultMineBits returns consensus mining difficulty for new blocks.
// Local FAST env does not soften chain PoW (would be rejected by honest peers).
func DefaultMineBits() int {
	return ConsensusMinDifficulty
}

func jsonMarshal(v any) ([]byte, error) {
	return marshalJSON(v)
}

func jsonUnmarshal(data []byte, v any) error {
	return unmarshalJSON(data, v)
}

// HashBytes helper.
func HashBytes(b []byte) string {
	return hex.EncodeToString(b)
}
