// Package crypto — X3DH session establishment (whitepaper §4.2 layer 1).
// OPK is single-use: ConsumeOPK after successful 4-DH, then RotateOPK for next peers.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"sync"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// PreKeyBundle is published for others to initiate X3DH.
type PreKeyBundle struct {
	mu sync.Mutex

	IdentityX    [32]byte
	SignedPreX   [32]byte
	SignedPreSig []byte
	OneTimeX     [32]byte
	HasOPK       bool
	OPKID        string // hex sha256 of OneTimeX — for Consume tracking

	// owner secrets
	edPriv        ed25519.PrivateKey
	identityPriv  [32]byte
	signedPrePriv [32]byte
	oneTimePriv   [32]byte
	hasOPKPriv    bool
}

// GenerateBundle creates SPK + OPK signed by ed identity key.
func GenerateBundle(edPriv ed25519.PrivateKey, identityXPriv [32]byte) (*PreKeyBundle, error) {
	b := &PreKeyBundle{edPriv: edPriv, identityPriv: identityXPriv}
	var idPub [32]byte
	curve25519.ScalarBaseMult(&idPub, &identityXPriv)
	b.IdentityX = idPub

	if err := b.rotateSPK(); err != nil {
		return nil, err
	}
	if err := b.RotateOPK(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *PreKeyBundle) rotateSPK() error {
	var spkPriv [32]byte
	if _, err := rand.Read(spkPriv[:]); err != nil {
		return err
	}
	clamp(&spkPriv)
	var spkPub [32]byte
	curve25519.ScalarBaseMult(&spkPub, &spkPriv)
	b.signedPrePriv = spkPriv
	b.SignedPreX = spkPub
	b.SignedPreSig = ed25519.Sign(b.edPriv, spkPub[:])
	return nil
}

// RotateOPK mints a fresh one-time prekey (after consume or on demand).
func (b *PreKeyBundle) RotateOPK() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rotateOPKLocked()
}

func (b *PreKeyBundle) rotateOPKLocked() error {
	var opkPriv [32]byte
	if _, err := rand.Read(opkPriv[:]); err != nil {
		return err
	}
	clamp(&opkPriv)
	var opkPub [32]byte
	curve25519.ScalarBaseMult(&opkPub, &opkPriv)
	b.oneTimePriv = opkPriv
	b.OneTimeX = opkPub
	b.HasOPK = true
	b.hasOPKPriv = true
	b.OPKID = OPKIDFromPub(opkPub[:])
	return nil
}

// OPKIDFromPub stable id for an OPK public key.
func OPKIDFromPub(pub []byte) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:16])
}

// ConsumeOPK marks local OPK as used (single-use). Call after successful 4-DH respond.
// Returns true if an OPK was consumed. Caller should RotateOPK + re-flood.
func (b *PreKeyBundle) ConsumeOPK() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasOPKPriv || !b.HasOPK {
		return false
	}
	// wipe OPK secret
	for i := range b.oneTimePriv {
		b.oneTimePriv[i] = 0
	}
	for i := range b.OneTimeX {
		b.OneTimeX[i] = 0
	}
	b.HasOPK = false
	b.hasOPKPriv = false
	b.OPKID = ""
	return true
}

// StripRemoteOPK clears OPK on a *remote* cached bundle after initiator used it once.
func (b *PreKeyBundle) StripRemoteOPK() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.HasOPK = false
	b.hasOPKPriv = false
	for i := range b.OneTimeX {
		b.OneTimeX[i] = 0
	}
	b.OPKID = ""
}

// PublicJSON for wire.
func (b *PreKeyBundle) PublicJSON() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := map[string]any{
		"identity_x":     base64.StdEncoding.EncodeToString(b.IdentityX[:]),
		"signed_pre_x":   base64.StdEncoding.EncodeToString(b.SignedPreX[:]),
		"signed_pre_sig": base64.StdEncoding.EncodeToString(b.SignedPreSig),
	}
	if b.HasOPK {
		m["one_time_x"] = base64.StdEncoding.EncodeToString(b.OneTimeX[:])
		m["opk_id"] = b.OPKID
	}
	return m
}

// ParseBundle from public map (remote peer — no private keys).
func ParseBundle(m map[string]string, edPub ed25519.PublicKey) (*PreKeyBundle, error) {
	b := &PreKeyBundle{}
	if err := b64to32(m["identity_x"], &b.IdentityX); err != nil {
		return nil, err
	}
	if err := b64to32(m["signed_pre_x"], &b.SignedPreX); err != nil {
		return nil, err
	}
	sig, err := base64.StdEncoding.DecodeString(m["signed_pre_sig"])
	if err != nil {
		return nil, err
	}
	b.SignedPreSig = sig
	if !ed25519.Verify(edPub, b.SignedPreX[:], sig) {
		return nil, errors.New("signed prekey signature invalid")
	}
	if ot, ok := m["one_time_x"]; ok && ot != "" {
		if err := b64to32(ot, &b.OneTimeX); err != nil {
			return nil, err
		}
		b.HasOPK = true
		if id, ok := m["opk_id"]; ok && id != "" {
			b.OPKID = id
		} else {
			b.OPKID = OPKIDFromPub(b.OneTimeX[:])
		}
	}
	return b, nil
}

// InitiatorSecret X3DH as sender. Uses OPK if present (4-DH), else 3-DH.
func InitiatorSecret(ikaPriv [32]byte, ekPriv [32]byte, remote *PreKeyBundle) (key []byte, ekPub [32]byte, usedOPK bool, opkID string, err error) {
	curve25519.ScalarBaseMult(&ekPub, &ekPriv)

	remote.mu.Lock()
	hasOPK := remote.HasOPK
	signedPre := remote.SignedPreX
	identityX := remote.IdentityX
	oneTime := remote.OneTimeX
	opkID = remote.OPKID
	remote.mu.Unlock()

	dh1, err := curve25519.X25519(ikaPriv[:], signedPre[:])
	if err != nil {
		return nil, ekPub, false, "", err
	}
	dh2, err := curve25519.X25519(ekPriv[:], identityX[:])
	if err != nil {
		return nil, ekPub, false, "", err
	}
	dh3, err := curve25519.X25519(ekPriv[:], signedPre[:])
	if err != nil {
		return nil, ekPub, false, "", err
	}
	material := append([]byte{}, dh1...)
	material = append(material, dh2...)
	material = append(material, dh3...)
	if hasOPK {
		dh4, err := curve25519.X25519(ikaPriv[:], oneTime[:])
		if err != nil {
			return nil, ekPub, false, "", err
		}
		material = append(material, dh4...)
		usedOPK = true
	}
	key, err = kdf(material, "MSP-X3DH-v1")
	return key, ekPub, usedOPK, opkID, err
}

// ResponderSecret X3DH as receiver.
func ResponderSecret(ikaPriv, spkPriv, opkPriv [32]byte, hasOPK bool, remoteIK [32]byte, remoteEK [32]byte) ([]byte, error) {
	dh1, err := curve25519.X25519(spkPriv[:], remoteIK[:])
	if err != nil {
		return nil, err
	}
	dh2, err := curve25519.X25519(ikaPriv[:], remoteEK[:])
	if err != nil {
		return nil, err
	}
	dh3, err := curve25519.X25519(spkPriv[:], remoteEK[:])
	if err != nil {
		return nil, err
	}
	material := append([]byte{}, dh1...)
	material = append(material, dh2...)
	material = append(material, dh3...)
	if hasOPK {
		dh4, err := curve25519.X25519(opkPriv[:], remoteIK[:])
		if err != nil {
			return nil, err
		}
		material = append(material, dh4...)
	}
	return kdf(material, "MSP-X3DH-v1")
}

// NewEphemeral creates clamped X25519 private key.
func NewEphemeral() ([32]byte, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return k, err
	}
	clamp(&k)
	return k, nil
}

func clamp(k *[32]byte) {
	k[0] &= 248
	k[31] &= 127
	k[31] |= 64
}

func kdf(ikm []byte, info string) ([]byte, error) {
	r := hkdf.New(sha512.New, ikm, []byte("MSP-X3DH"), []byte(info))
	out := make([]byte, 32)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

func b64to32(s string, out *[32]byte) error {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	if len(raw) != 32 {
		return errors.New("need 32 bytes")
	}
	copy(out[:], raw)
	return nil
}

// BundlePriv returns owner secrets for responder (snapshot).
func (b *PreKeyBundle) BundlePriv() (spk, opk [32]byte, has bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.signedPrePriv, b.oneTimePriv, b.hasOPKPriv
}

// EncodeU64 helper.
func EncodeU64(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	return b[:]
}
