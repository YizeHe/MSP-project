package chain

import "errors"

var (
	errInvalidSig      = errors.New("invalid transaction signature")
	errInsufficient    = errors.New("insufficient MST balance")
	errBadNonce        = errors.New("bad account nonce")
	errClaimDisabled   = errors.New("free genesis claim disabled on mainnet-2 (stake/propose only)")
	errUnknownTx       = errors.New("unknown tx type")
	errBlockTooLarge   = errors.New("block exceeds size limit")
	errBadPoW          = errors.New("invalid block PoW") // unused in PoS; kept for errors
	errBadDifficulty   = errors.New("block difficulty below network consensus minimum")
	errBadCoinbase     = errors.New("invalid coinbase reward")
	errBadPrev         = errors.New("prev block mismatch")
	errBadHeight       = errors.New("height mismatch")
	errBadSlot         = errors.New("invalid slot")
	errBadProposer     = errors.New("invalid proposer or signature")
	errBadMerkle       = errors.New("merkle root mismatch")
	errBadState        = errors.New("state root mismatch")
	errDupTx           = errors.New("duplicate transaction")
	errMinStake        = errors.New("stake below minimum for activation")
	errNotValidator    = errors.New("not an active validator")
	errAmountOverflow  = errors.New("amount+fee overflow")
	errAmountTooLarge  = errors.New("amount exceeds supply cap")
)
