package chain

import (
	"fmt"
	"time"
)

// ValidateBlock PoS consensus checks (no block PoW).
func ValidateBlock(b *Block, prev *BlockHeader, st *State) error {
	if b.Size() > MaxBlockBytes {
		return errBlockTooLarge
	}
	if MerkleRootFromTxs(b.Txs) != b.Header.MerkleRoot {
		return errBadMerkle
	}
	if prev == nil {
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
	if b.Header.Slot <= prev.Slot && b.Header.Height > 0 {
		// slots must be non-decreasing; allow equal only if height 0
		if b.Header.Slot < prev.Slot {
			return errBadSlot
		}
	}
	// timestamp within slot window (±1 slot skew)
	slotStart := SlotStartTime(b.Header.Slot)
	slotEnd := slotStart.Add(SlotDuration)
	ts := time.UnixMicro(b.Header.Timestamp)
	if ts.Before(slotStart.Add(-SlotDuration)) || ts.After(slotEnd.Add(SlotDuration)) {
		return fmt.Errorf("%w: timestamp outside slot window", errBadSlot)
	}
	// proposer election + signature
	if err := VerifyProposerSig(&b.Header); err != nil {
		return err
	}
	if b.Header.Proposer == "" {
		return errBadProposer
	}
	// election relative to parent state (before this block)
	if st != nil {
		if !MayPropose(st, prev.HashHex(), b.Header.Proposer, b.Header.Slot, b.Header.Height) {
			return fmt.Errorf("%w: not elected for slot %d", errBadProposer, b.Header.Slot)
		}
	}
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
		if err := unmarshalJSON(b.Txs[i].Data, &d); err != nil {
			return errBadCoinbase
		}
		if d.Amount > want {
			return fmt.Errorf("%w: amount %d > schedule %d", errBadCoinbase, d.Amount, want)
		}
		// coinbase sender must be proposer
		if b.Txs[i].Sender != b.Header.Proposer && b.Header.Proposer != "" {
			return fmt.Errorf("%w: coinbase sender != proposer", errBadCoinbase)
		}
	}
	if want > 0 && seen == 0 {
		return fmt.Errorf("%w: missing coinbase", errBadCoinbase)
	}
	return nil
}

// TargetInterval always production 10 minutes (no lab shortcuts).
func TargetInterval() time.Duration {
	return SlotDuration
}

// CurrentSlot wall-clock slot.
func CurrentSlot() uint64 {
	return SlotAtTime(time.Now().UTC())
}
