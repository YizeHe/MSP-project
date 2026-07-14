package chain

import "errors"

var (
	errInvalidSig      = errors.New("invalid transaction signature")
	errInsufficient    = errors.New("insufficient MST balance")
	errBadNonce        = errors.New("bad account nonce")
	errAlreadyClaimed  = errors.New("genesis already claimed")
	errClaimClosed     = errors.New("genesis claim window closed")
	errSupplyExhausted = errors.New("genesis supply exhausted")
	errUnknownTx       = errors.New("unknown tx type")
	errBlockTooLarge   = errors.New("block exceeds size limit")
	errBadPoW          = errors.New("invalid block PoW")
	errBadDifficulty   = errors.New("block difficulty below network consensus minimum")
	errBadCoinbase     = errors.New("invalid coinbase reward")
	errBadPrev         = errors.New("prev block mismatch")
	errBadHeight       = errors.New("height mismatch")
	errBadMerkle       = errors.New("merkle root mismatch")
	errBadState        = errors.New("state root mismatch")
	errDupTx           = errors.New("duplicate transaction")
)
