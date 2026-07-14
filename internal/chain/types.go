// Package chain: public ledger for MST economy + Ethereum-inspired PoS consensus.
// Zero message content on-chain. Production mainnet only (no lab FAST modes).
package chain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"
)

const (
	ProtocolVersion int32 = 3

	// Mainnet identity — v2 after PoS + no free-claim redesign
	ChainID     = "msp-mainnet-2"
	NetworkName = "mainnet"

	// --- PoS (Ethereum-inspired: slots, stake-weighted proposer) ---
	// Fixed slot / block target: 10 minutes (production).
	SlotDuration  = 10 * time.Minute
	EpochLength   = uint64(32) // slots per epoch (ETH-like granularity)
	// MinStake after bootstrap: ~one early block reward unit
	MinStake uint64 = 50
	// BootstrapSlots: first epoch — validators may activate with 0 liquid MST
	// (cold-start: no free claim; earn rewards by proposing, then restake).
	BootstrapSlots = EpochLength // 32 slots ≈ 5.3 hours

	// --- Token economics: issuance ONLY via block rewards (no free claim) ---
	// Pre-allocated validator reward pool (scheme B, no inflation beyond genesis allocation).
	MinerRewardPool uint64 = 4_200_000
	GenesisSupply   uint64 = MinerRewardPool // all MST enters via proposing rewards
	// Legacy names kept zero so old call sites fail closed
	GenesisGrant    uint64 = 0
	MaxClaimNodes   uint64 = 0
	ClaimableSupply uint64 = 0
	ClaimWindow            = 0

	// ~4 years of 10-min blocks
	HalvingInterval uint64 = 210_240

	MaxBlockBytes = 1 << 20
	ConfirmDepth  = 6

	// Fees
	FeeTransfer  uint64 = 1
	FeeRegister  uint64 = 1
	FeeStake     uint64 = 1
	FeeUnstake   uint64 = 1
	BurnDirect    uint64 = 2
	BurnBroadcast uint64 = 5
	BurnDTN       uint64 = 10
	BurnAlert     uint64 = 20
	FeeBurnBase   uint64 = 1

	// Tx types
	TxTransfer      = "transfer"
	TxBurn          = "burn"
	TxRegister      = "register"
	TxInitAlert     = "init_alert"
	TxNetworkParams = "network_params"
	TxCoinbase      = "coinbase"
	TxStake         = "stake"
	TxUnstake       = "unstake"
	TxActivate      = "activate" // join validator set (bond existing stake)
	// TxGenesisClaim removed — free claim disabled on mainnet-2

	MsgDirect    = "direct"
	MsgDTN       = "dtn"
	MsgBroadcast = "broadcast"
	MsgAlert     = "alert"
)

// NetworkGenesis fixed mainnet birth (slot 0).
var NetworkGenesis = time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

// BlockHeader — PoS fields (no PoW nonce mining for consensus).
type BlockHeader struct {
	Version      int32  `json:"version"`
	PrevBlock    string `json:"prev_block"`
	MerkleRoot   string `json:"merkle_root"`
	StateRoot    string `json:"state_root"`
	Timestamp    int64  `json:"timestamp"`
	Height       uint64 `json:"height"`
	Slot         uint64 `json:"slot"`
	Proposer     string `json:"proposer"` // NodeID
	ProposerPub  string `json:"proposer_pub"`
	ProposerSig  string `json:"proposer_sig"` // Ed25519 over SignMaterial
	TxCount      uint32 `json:"tx_count"`
	// Difficulty/Nonce/PoWHash retained empty for wire compat; unused in PoS
	Difficulty int    `json:"difficulty,omitempty"`
	Nonce      uint64 `json:"nonce,omitempty"`
	PoWHash    string `json:"pow_hash,omitempty"`
	MinerID    string `json:"miner_id,omitempty"` // alias of proposer for old tools
}

// Hash of header for chaining (excludes signature).
func (h *BlockHeader) Hash() [32]byte {
	raw, _ := json.Marshal(struct {
		Version     int32  `json:"version"`
		PrevBlock   string `json:"prev_block"`
		MerkleRoot  string `json:"merkle_root"`
		StateRoot   string `json:"state_root"`
		Timestamp   int64  `json:"timestamp"`
		Height      uint64 `json:"height"`
		Slot        uint64 `json:"slot"`
		Proposer    string `json:"proposer"`
		ProposerPub string `json:"proposer_pub"`
		TxCount     uint32 `json:"tx_count"`
	}{
		h.Version, h.PrevBlock, h.MerkleRoot, h.StateRoot,
		h.Timestamp, h.Height, h.Slot, h.Proposer, h.ProposerPub, h.TxCount,
	})
	return sha256.Sum256(raw)
}

// HashHex hex.
func (h *BlockHeader) HashHex() string {
	sum := h.Hash()
	return hex.EncodeToString(sum[:])
}

// SignMaterial for proposer Ed25519 signature.
func (h *BlockHeader) SignMaterial() []byte {
	sum := h.Hash()
	return sum[:]
}

// Block.
type Block struct {
	Header BlockHeader   `json:"header"`
	Txs    []Transaction `json:"txs"`
}

// Size JSON size.
func (b *Block) Size() int {
	raw, _ := json.Marshal(b)
	return len(raw)
}

// Transaction economic / staking.
type Transaction struct {
	Type      string          `json:"type"`
	Sender    string          `json:"sender"`
	SenderPub string          `json:"sender_pub"`
	Nonce     uint64          `json:"nonce"`
	Fee       uint64          `json:"fee"`
	Data      json.RawMessage `json:"data"`
	Signature string          `json:"signature"`
}

// SignMaterial.
func (tx *Transaction) SignMaterial() []byte {
	raw, _ := json.Marshal(struct {
		Type      string          `json:"type"`
		Sender    string          `json:"sender"`
		SenderPub string          `json:"sender_pub"`
		Nonce     uint64          `json:"nonce"`
		Fee       uint64          `json:"fee"`
		Data      json.RawMessage `json:"data"`
	}{tx.Type, tx.Sender, tx.SenderPub, tx.Nonce, tx.Fee, tx.Data})
	return raw
}

// Sign.
func (tx *Transaction) Sign(priv ed25519.PrivateKey) {
	sig := ed25519.Sign(priv, tx.SignMaterial())
	tx.Signature = base64.StdEncoding.EncodeToString(sig)
}

// Verify.
func (tx *Transaction) Verify() error {
	if tx.Type == TxInitAlert || tx.Type == TxCoinbase || tx.Type == TxNetworkParams {
		return nil
	}
	pub, err := base64.StdEncoding.DecodeString(tx.SenderPub)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errInvalidSig
	}
	sig, err := base64.StdEncoding.DecodeString(tx.Signature)
	if err != nil {
		return errInvalidSig
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), tx.SignMaterial(), sig) {
		return errInvalidSig
	}
	return nil
}

// TxID.
func (tx *Transaction) TxID() string {
	h := sha256.Sum256(append(tx.SignMaterial(), []byte(tx.Signature)...))
	return hex.EncodeToString(h[:])
}

// TransferData.
type TransferData struct {
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
}

// BurnData — hash only.
type BurnData struct {
	MsgType   string `json:"msg_type"`
	RefHash   string `json:"ref_hash"`
	Timestamp int64  `json:"timestamp"`
	Amount    uint64 `json:"amount"`
}

// RegisterData.
type RegisterData struct {
	Ed25519Pub string `json:"ed25519_pub"`
	X25519Pub  string `json:"x25519_pub"`
}

// InitAlertData.
type InitAlertData struct {
	PubKey string `json:"pubkey"`
}

// NetworkParamsData frozen at genesis.
type NetworkParamsData struct {
	ChainID         string `json:"chain_id"`
	Name            string `json:"name"`
	Consensus       string `json:"consensus"` // "pos-eth-inspired"
	SlotSeconds     int64  `json:"slot_seconds"`
	EpochLength     uint64 `json:"epoch_length"`
	MinStake        uint64 `json:"min_stake"`
	BootstrapSlots  uint64 `json:"bootstrap_slots"`
	MinerRewardPool uint64 `json:"miner_reward_pool"`
	GenesisSupply   uint64 `json:"genesis_supply"`
	HalvingInterval uint64 `json:"halving_interval"`
	FreeClaim       bool   `json:"free_claim"` // always false on mainnet-2
}

// CoinbaseData proposer reward from pool.
type CoinbaseData struct {
	Amount uint64 `json:"amount"`
	Height uint64 `json:"height"`
	Slot   uint64 `json:"slot"`
}

// StakeData lock MST into validator stake.
type StakeData struct {
	Amount uint64 `json:"amount"`
}

// UnstakeData unlock stake to liquid balance.
type UnstakeData struct {
	Amount uint64 `json:"amount"`
}

// ActivateData join/refresh validator set.
type ActivateData struct {
	Ed25519Pub string `json:"ed25519_pub"`
}

// Account ledger.
type Account struct {
	NodeID     string `json:"node_id"`
	Balance    uint64 `json:"balance"` // liquid
	Stake      uint64 `json:"stake"`   // bonded for PoS
	Nonce      uint64 `json:"nonce"`
	Burned     uint64 `json:"burned"`
	RegHeight  uint64 `json:"reg_height"`
	Ed25519Pub string `json:"ed25519_pub,omitempty"`
	X25519Pub  string `json:"x25519_pub,omitempty"`
	Active     bool   `json:"active,omitempty"` // in validator set
}

// EncodeData JSON.
func EncodeData(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// BlockReward from pool (halving schedule).
func BlockReward(height uint64) uint64 {
	if height == 0 {
		return 0
	}
	halvings := (height - 1) / HalvingInterval
	if halvings >= 4 {
		return 0
	}
	base := uint64(50)
	for i := uint64(0); i < halvings; i++ {
		base /= 2
	}
	return base
}

// BurnAmountForMsgType.
func BurnAmountForMsgType(msgType string) uint64 {
	switch msgType {
	case MsgDirect:
		return BurnDirect
	case MsgDTN:
		return BurnDTN
	case MsgBroadcast:
		return BurnBroadcast
	case MsgAlert:
		return BurnAlert
	default:
		return BurnDTN
	}
}

// RefHashFromBytes sha256 hex.
func RefHashFromBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// SlotAtTime maps wall clock to slot index since NetworkGenesis.
func SlotAtTime(t time.Time) uint64 {
	if t.Before(NetworkGenesis) {
		return 0
	}
	return uint64(t.Sub(NetworkGenesis) / SlotDuration)
}

// SlotStartTime returns UTC start of slot.
func SlotStartTime(slot uint64) time.Time {
	return NetworkGenesis.Add(time.Duration(slot) * SlotDuration)
}
