// BurnTicket: pre-confirmation burn credential (代办5 修复一).
package chain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// BurnTicketTTL ticket validity.
const BurnTicketTTL = 24 * time.Hour

// BurnTicket is signed by a miner (or local block producer) for a burn tx in mempool/chain.
type BurnTicket struct {
	BurnTxHash string `json:"burn_tx_hash"`
	MsgType    string `json:"msg_type"`
	RefHash    string `json:"ref_hash"`
	Amount     uint64 `json:"amount"` // burn amount + fee locked
	Nonce      uint64 `json:"nonce"`
	Timestamp  int64  `json:"timestamp"`
	MinerID    string `json:"miner_id"`
	MinerPub   string `json:"miner_pub"` // ed25519 b64 of issuer
	BlockHint  uint64 `json:"block_hint,omitempty"`
	MinerSig    string `json:"miner_sig"`
}

// SignBytes canonical bytes for miner signature.
func (t *BurnTicket) SignBytes() []byte {
	raw, _ := json.Marshal(struct {
		BurnTxHash string `json:"burn_tx_hash"`
		MsgType    string `json:"msg_type"`
		RefHash    string `json:"ref_hash"`
		Amount     uint64 `json:"amount"`
		Nonce      uint64 `json:"nonce"`
		Timestamp  int64  `json:"timestamp"`
		MinerID    string `json:"miner_id"`
		MinerPub    string `json:"miner_pub"`
		BlockHint  uint64 `json:"block_hint"`
	}{
		t.BurnTxHash, t.MsgType, t.RefHash, t.Amount, t.Nonce,
		t.Timestamp, t.MinerID, t.MinerPub, t.BlockHint,
	})
	return raw
}

// Sign ticket with miner private key.
func (t *BurnTicket) Sign(priv ed25519.PrivateKey) {
	t.MinerPub = base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	sig := ed25519.Sign(priv, t.SignBytes())
	t.MinerSig = base64.StdEncoding.EncodeToString(sig)
}

// Verify miner signature.
func (t *BurnTicket) Verify() error {
	pub, err := base64.StdEncoding.DecodeString(t.MinerPub)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("invalid miner pub")
	}
	sig, err := base64.StdEncoding.DecodeString(t.MinerSig)
	if err != nil {
		return errors.New("invalid miner sig")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), t.SignBytes(), sig) {
		return errors.New("bad miner signature")
	}
	return nil
}

// Expired.
func (t *BurnTicket) Expired(now time.Time) bool {
	if t.Timestamp == 0 {
		return true
	}
	return now.Sub(time.UnixMicro(t.Timestamp)) > BurnTicketTTL || time.UnixMicro(t.Timestamp).After(now.Add(time.Minute))
}

// Encode ticket to base64 JSON for packet field.
func (t *BurnTicket) Encode() string {
	raw, _ := json.Marshal(t)
	return base64.StdEncoding.EncodeToString(raw)
}

// DecodeTicket from base64 JSON.
func DecodeTicket(s string) (*BurnTicket, error) {
	if s == "" {
		return nil, errors.New("empty ticket")
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var t BurnTicket
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// IssueBurnTicket signs a ticket for a burn transaction (local miner/issuer).
func IssueBurnTicket(burnTx *Transaction, minerID string, priv ed25519.PrivateKey, blockHint uint64) (*BurnTicket, error) {
	if burnTx.Type != TxBurn {
		return nil, errors.New("not a burn tx")
	}
	var d BurnData
	if err := json.Unmarshal(burnTx.Data, &d); err != nil {
		return nil, err
	}
	amount := d.Amount
	if amount == 0 {
		amount = BurnAmountForMsgType(d.MsgType)
	}
	amount += burnTx.Fee
	if burnTx.Fee == 0 {
		amount += FeeBurnBase
	}
	var nb [8]byte
	_, _ = rand.Read(nb[:])
	nonce := binary.BigEndian.Uint64(nb[:])
	t := &BurnTicket{
		BurnTxHash: burnTx.TxID(),
		MsgType:    d.MsgType,
		RefHash:    d.RefHash,
		Amount:     amount,
		Nonce:      nonce,
		Timestamp:  time.Now().UnixMicro(),
		MinerID:    minerID,
		BlockHint:  blockHint,
	}
	t.Sign(priv)
	return t, nil
}

// ValidateBurnTicket full receive-side checks (caller supplies hasTx).
func ValidateBurnTicket(ticket *BurnTicket, payloadCipher string, hasTx func(txHash string) bool, now time.Time) error {
	if ticket == nil {
		return errors.New("missing burn ticket")
	}
	if err := ticket.Verify(); err != nil {
		return err
	}
	if ticket.Expired(now) {
		return errors.New("burn ticket expired")
	}
	// RefHash matches ciphertext (or provided raw)
	var msgHash string
	if payloadCipher != "" {
		raw, err := base64.StdEncoding.DecodeString(payloadCipher)
		if err != nil {
			// treat as raw string bytes
			sum := sha256.Sum256([]byte(payloadCipher))
			msgHash = hex.EncodeToString(sum[:])
		} else {
			sum := sha256.Sum256(raw)
			msgHash = hex.EncodeToString(sum[:])
		}
		// also accept hash of the base64 string itself
		sum2 := sha256.Sum256([]byte(payloadCipher))
		h2 := hex.EncodeToString(sum2[:])
		if ticket.RefHash != msgHash && ticket.RefHash != h2 {
			// third form: RefHash of full packet computed by sender
			// allow exact match only to ticket — sender must use same method
			if !refHashFlexibleMatch(ticket.RefHash, payloadCipher) {
				return fmt.Errorf("ref hash mismatch")
			}
		}
	}
	required := BurnAmountForMsgType(ticket.MsgType)
	if ticket.Amount < required {
		return fmt.Errorf("insufficient burn: ticket %d need %d", ticket.Amount, required)
	}
	if hasTx != nil && !hasTx(ticket.BurnTxHash) {
		return errors.New("burn tx not found in mempool or chain")
	}
	return nil
}

func refHashFlexibleMatch(want, payloadCipher string) bool {
	// try sha256 of decoded cipher, of raw b64, of utf8
	candidates := [][]byte{[]byte(payloadCipher)}
	if raw, err := base64.StdEncoding.DecodeString(payloadCipher); err == nil {
		candidates = append(candidates, raw)
	}
	for _, c := range candidates {
		sum := sha256.Sum256(c)
		if hex.EncodeToString(sum[:]) == want {
			return true
		}
	}
	return want == payloadCipher // last resort identity
}

// RefHashPayloadCipher standard ref hash for tickets.
func RefHashPayloadCipher(payloadCipherB64 string) string {
	raw, err := base64.StdEncoding.DecodeString(payloadCipherB64)
	if err != nil {
		sum := sha256.Sum256([]byte(payloadCipherB64))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
