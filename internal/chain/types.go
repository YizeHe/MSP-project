// Package chain: public ledger for MST economy only (代办4.md).
// Zero message content / communication metadata on-chain.
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
	ProtocolVersion int32 = 2
	MaxBlockBytes         = 1 << 20
	BlockInterval         = 10 * time.Minute
	FastBlockInterval     = 15 * time.Second
	ConfirmDepth          = 6
	// 代办6：降低免费认领额度 — 128 MST × 210 节点；主量来自挖矿池
	GenesisGrant    uint64 = 128
	MaxClaimNodes   uint64 = 210
	MinerRewardPool uint64 = 4_200_000           // 方案 B 矿工池不变
	ClaimableSupply uint64 = MaxClaimNodes * GenesisGrant // 26_880
	GenesisSupply   uint64 = MinerRewardPool + ClaimableSupply // 4_226_880
	ClaimWindow            = 2 * 365 * 24 * time.Hour
	// ~4 years of 10-min blocks
	HalvingInterval uint64 = 210_240

	// Fees (代办4/5)
	FeeTransfer  uint64 = 1
	FeeRegister  uint64 = 1
	BurnDirect    uint64 = 2
	BurnBroadcast uint64 = 5
	BurnDTN       uint64 = 10
	BurnAlert     uint64 = 20
	FeeBurnBase   uint64 = 1

	// Tx types — economic only
	TxGenesisClaim = "genesis_claim"
	TxTransfer     = "transfer"
	TxBurn         = "burn"
	TxRegister     = "register"
	TxInitAlert    = "init_alert"
	TxCoinbase     = "coinbase"

	// Burn MsgType values
	MsgDirect    = "direct"
	MsgDTN       = "dtn"
	MsgBroadcast = "broadcast"
	MsgAlert     = "alert"
)

// NetworkGenesis fixed.
var NetworkGenesis = time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

// BlockHeader.
type BlockHeader struct {
	Version    int32  `json:"version"`
	PrevBlock  string `json:"prev_block"`
	MerkleRoot string `json:"merkle_root"`
	StateRoot  string `json:"state_root"`
	Timestamp  int64  `json:"timestamp"`
	Difficulty int    `json:"difficulty"`
	Nonce      uint64 `json:"nonce"`
	Height     uint64 `json:"height"`
	MinerID    string `json:"miner_id"`
	TxCount    uint32 `json:"tx_count"`
	PoWHash    string `json:"pow_hash,omitempty"`
}

// Hash of header for chaining / PoW.
func (h *BlockHeader) Hash() [32]byte {
	raw, _ := json.Marshal(struct {
		Version    int32  `json:"version"`
		PrevBlock  string `json:"prev_block"`
		MerkleRoot string `json:"merkle_root"`
		StateRoot  string `json:"state_root"`
		Timestamp  int64  `json:"timestamp"`
		Difficulty int    `json:"difficulty"`
		Nonce      uint64 `json:"nonce"`
		Height     uint64 `json:"height"`
		MinerID     string `json:"miner_id"`
		TxCount    uint32 `json:"tx_count"`
	}{
		h.Version, h.PrevBlock, h.MerkleRoot, h.StateRoot,
		h.Timestamp, h.Difficulty, h.Nonce, h.Height, h.MinerID, h.TxCount,
	})
	return sha256.Sum256(raw)
}

// HashHex hex.
func (h *BlockHeader) HashHex() string {
	sum := h.Hash()
	return hex.EncodeToString(sum[:])
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

// Transaction economic only.
type Transaction struct {
	Type      string          `json:"type"`
	Sender    string          `json:"sender"`
	SenderPub string          `json:"sender_pub"`
	Nonce     uint64          `json:"nonce"`
	Fee       uint64          `json:"fee"` // miner fee (burned from sender, credited to miner in block apply)
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
	if tx.Type == TxInitAlert || tx.Type == TxCoinbase {
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

// --- payloads (no message bodies) ---

// TransferData.
type TransferData struct {
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
}

// BurnData — only hash of message, never content.
type BurnData struct {
	MsgType   string `json:"msg_type"` // direct|dtn|broadcast|alert
	RefHash   string `json:"ref_hash"` // hex sha256 of ciphertext/packet
	Timestamp int64  `json:"timestamp"`
	Amount    uint64 `json:"amount"` // MST burned for the message
}

// RegisterData public keys on-chain once.
type RegisterData struct {
	Ed25519Pub string `json:"ed25519_pub"`
	X25519Pub  string `json:"x25519_pub"`
}

// InitAlertData genesis.
type InitAlertData struct {
	PubKey string `json:"pubkey"`
}

// CoinbaseData miner reward from pre-allocated pool (代办5 方案 B).
type CoinbaseData struct {
	Amount uint64 `json:"amount"`
	Height uint64 `json:"height"`
}

// BlockReward from miner pool (halving schedule, no inflation beyond genesis).
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

// Account ledger.
type Account struct {
	NodeID    string `json:"node_id"`
	Balance   uint64 `json:"balance"`
	Nonce     uint64 `json:"nonce"`
	Burned    uint64 `json:"burned"`
	Claimed   bool   `json:"claimed"`
	RegHeight uint64 `json:"reg_height"`
	// optional registered keys
	Ed25519Pub string `json:"ed25519_pub,omitempty"`
	X25519Pub  string `json:"x25519_pub,omitempty"`
}

// EncodeData JSON.
func EncodeData(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
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

// RefHashFromBytes sha256 hex of packet bytes.
func RefHashFromBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
