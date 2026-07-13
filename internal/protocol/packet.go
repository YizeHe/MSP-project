// Package protocol defines the MST Network wire packet (v1).
package protocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	MaxHops       = 30
	TimeSkewMax   = 500 * time.Millisecond
	MaxPayloadBytes = 32 * 1024
	PacketVersion = 2

	// Msg kinds
	KindChat      = "chat"
	KindKeyFlood  = "key_flood"
	KindTimeSync  = "time_sync"
	KindDiffHint  = "diff_hint"
)

// Packet is clear header + encrypted payload (whitepaper §5).
type Packet struct {
	Version int `json:"v"`

	// Kind distinguishes chat vs control floods.
	Kind string `json:"kind,omitempty"`

	PacketSHA1   string `json:"packet_sha1"`
	PacketSHA512 string `json:"packet_sha512"`
	TimestampUs  int64  `json:"timestamp_us"`
	SenderID     string `json:"sender_id"`
	TargetID     string `json:"target_id"`
	Broadcast    bool   `json:"broadcast"`
	Alert        bool   `json:"alert"`

	// PoW (memory-hard algorithm name + difficulty + nonce)
	PoWAlgo       string `json:"pow_algo"` // "msp-mh-v1" (Argon2id memory-hard; RandomX-class anti-ASIC)
	PoWDifficulty int    `json:"pow_difficulty"`
	PoWNonce      uint64 `json:"pow_nonce"`
	PoWHash       string `json:"pow_hash,omitempty"` // hex proof

	TTLMaxHops int `json:"ttl_max_hops"`
	Hops       int `json:"hops"`

	SenderPubKey    string `json:"sender_pubkey"`
	SenderX25519Pub string `json:"sender_x25519"`

	// X3DH: sender ephemeral public (base64 32)
	EphemeralX string `json:"eph_x,omitempty"`
	// Optional: which OPK was used (index / hash)
	OPKUsed string `json:"opk_used,omitempty"`

	// Prekey bundle public fields when Kind=key_flood
	KeyBundle json.RawMessage `json:"key_bundle,omitempty"`

	// Network time sample for clock sync
	ClockSampleUs int64 `json:"clock_sample_us,omitempty"`

	// Difficulty observation for adaptive network difficulty
	DiffHintBits int `json:"diff_hint_bits,omitempty"`

	// Explicit payload MAC (HMAC-SHA512) over nonce||ciphertext — whitepaper §5.2
	PayloadMAC string `json:"payload_mac,omitempty"`

	Signature     string `json:"signature"`
	PayloadNonce  string `json:"payload_nonce,omitempty"`
	PayloadCipher string `json:"payload_cipher,omitempty"`
	ExpireAtUs    int64  `json:"expire_at_us,omitempty"`

	// MST burn proof metadata (optional display)
	MSTBurn uint64 `json:"mst_burn,omitempty"`

	// 代办5: BurnTicket (base64 JSON) — chain economy pre-confirmation
	BurnTicket string `json:"burn_ticket,omitempty"`
}

// Payload is decrypted business payload.
type Payload struct {
	Text       string `json:"text"`
	ExpireAtUs int64  `json:"expire_at_us"`
	// Extra MAC material confirmation
	InnerMAC string `json:"inner_mac,omitempty"`
}

// SignMaterial — header metadata + sealed payload bindings (not hops).
func (p *Packet) SignMaterial() []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, int32(p.Version))
	b.WriteString(p.Kind)
	b.WriteByte(0)
	_ = binary.Write(&b, binary.BigEndian, p.TimestampUs)
	b.WriteString(p.SenderID)
	b.WriteByte(0)
	b.WriteString(p.TargetID)
	b.WriteByte(0)
	if p.Broadcast {
		b.WriteByte(1)
	} else {
		b.WriteByte(0)
	}
	if p.Alert {
		b.WriteByte(1)
	} else {
		b.WriteByte(0)
	}
	b.WriteString(p.PoWAlgo)
	b.WriteByte(0)
	_ = binary.Write(&b, binary.BigEndian, int32(p.PoWDifficulty))
	_ = binary.Write(&b, binary.BigEndian, p.PoWNonce)
	b.WriteString(p.PoWHash)
	b.WriteByte(0)
	_ = binary.Write(&b, binary.BigEndian, int32(p.TTLMaxHops))
	b.WriteString(p.SenderPubKey)
	b.WriteByte(0)
	b.WriteString(p.SenderX25519Pub)
	b.WriteByte(0)
	b.WriteString(p.EphemeralX)
	b.WriteByte(0)
	b.WriteString(p.OPKUsed)
	b.WriteByte(0)
	b.Write(p.KeyBundle)
	b.WriteByte(0)
	b.WriteString(p.PayloadNonce)
	b.WriteByte(0)
	b.WriteString(p.PayloadCipher)
	b.WriteByte(0)
	b.WriteString(p.PayloadMAC)
	_ = binary.Write(&b, binary.BigEndian, p.ExpireAtUs)
	_ = binary.Write(&b, binary.BigEndian, p.MSTBurn)
	// BurnTicket not in signature material — attached after sealing/signing for flexibility
	// Actually include for integrity of ticket binding:
	b.WriteString(p.BurnTicket)
	return b.Bytes()
}

// PoWMaterial zeros nonce for mining base.
func (p *Packet) PoWMaterial() []byte {
	cp := *p
	cp.PoWNonce = 0
	cp.PoWHash = ""
	cp.Signature = ""
	cp.PacketSHA1 = ""
	cp.PacketSHA512 = ""
	return cp.SignMaterial()
}

// ComputeHashes fills dual hashes.
func (p *Packet) ComputeHashes() {
	body := p.hashBody()
	s1 := sha1.Sum(body)
	s512 := sha512.Sum512(body)
	p.PacketSHA1 = hex.EncodeToString(s1[:])
	p.PacketSHA512 = hex.EncodeToString(s512[:])
}

func (p *Packet) hashBody() []byte {
	var b bytes.Buffer
	b.Write(p.SignMaterial())
	b.WriteString(p.Signature)
	return b.Bytes()
}

// Sign with Ed25519.
func (p *Packet) Sign(priv ed25519.PrivateKey) {
	sig := ed25519.Sign(priv, p.SignMaterial())
	p.Signature = base64.StdEncoding.EncodeToString(sig)
	p.ComputeHashes()
}

// Verify signature, hashes, hops, optional clock against refNow.
func (p *Packet) Verify(refNow time.Time, checkClock bool) error {
	if p.Version != PacketVersion && p.Version != 1 {
		return fmt.Errorf("unsupported packet version %d", p.Version)
	}
	if p.Hops > p.TTLMaxHops {
		return errors.New("hop limit exceeded")
	}
	pubRaw, err := base64.StdEncoding.DecodeString(p.SenderPubKey)
	if err != nil || len(pubRaw) != ed25519.PublicKeySize {
		return errors.New("invalid sender pubkey")
	}
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	if err != nil {
		return errors.New("invalid signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(pubRaw), p.SignMaterial(), sig) {
		return errors.New("bad signature")
	}
	want := *p
	want.ComputeHashes()
	if want.PacketSHA1 != p.PacketSHA1 || want.PacketSHA512 != p.PacketSHA512 {
		return errors.New("hash mismatch")
	}
	if NodeIDFromPub(pubRaw) != p.SenderID {
		return errors.New("sender_id does not match pubkey")
	}
	if checkClock {
		ts := time.UnixMicro(p.TimestampUs)
		skew := refNow.Sub(ts)
		if skew < 0 {
			skew = -skew
		}
		if skew > TimeSkewMax {
			return fmt.Errorf("timestamp skew %v exceeds ±%v", skew, TimeSkewMax)
		}
	}
	if p.ExpireAtUs > 0 && refNow.UnixMicro() > p.ExpireAtUs {
		return errors.New("packet expired")
	}
	return nil
}

// MarshalJSONPacket encodes packet.
func MarshalJSONPacket(p *Packet) ([]byte, error) { return json.Marshal(p) }

// UnmarshalJSONPacket decodes packet.
func UnmarshalJSONPacket(data []byte) (*Packet, error) {
	var p Packet
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" {
		p.Kind = KindChat
	}
	return &p, nil
}

// LessForSort: timestamp_us asc, then SHA1 raw bytes asc.
func LessForSort(a, b *Packet) bool {
	if a.TimestampUs != b.TimestampUs {
		return a.TimestampUs < b.TimestampUs
	}
	ab, err1 := hex.DecodeString(a.PacketSHA1)
	bb, err2 := hex.DecodeString(b.PacketSHA1)
	if err1 != nil || err2 != nil {
		return a.PacketSHA1 < b.PacketSHA1
	}
	return bytes.Compare(ab, bb) < 0
}

// NodeIDFromPub SHA1 hex of Ed25519 public key.
func NodeIDFromPub(pub []byte) string {
	h := sha1.Sum(pub)
	return hex.EncodeToString(h[:])
}

// Fingerprint16 short fingerprint.
func Fingerprint16(pub []byte) string {
	id := NodeIDFromPub(pub)
	if len(id) >= 16 {
		return id[:16]
	}
	return id
}
