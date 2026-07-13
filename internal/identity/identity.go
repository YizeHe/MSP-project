// Package identity implements BIP39 mnemonic + MST key derivation (v0.1).
package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/pbkdf2"

	"github.com/YizeHe/MSP-project/internal/protocol"
)

// Identity is a recoverable MST node identity (keys only; no messages).
type Identity struct {
	Mnemonic     string `json:"mnemonic"`
	NodeID       string `json:"node_id"`
	Fingerprint  string `json:"fingerprint"`
	Ed25519Pub   string `json:"ed25519_pub"`  // base64
	X25519Pub    string `json:"x25519_pub"`   // base64
	CreatedAtRFC string `json:"created_at"`

	// Private material — never logged.
	edPriv ed25519.PrivateKey
	xPriv  [32]byte
}

// Generate creates a new 24-word identity from multi-source entropy mixed with SHA-512.
func Generate() (*Identity, error) {
	entropy, err := collectEntropy(32)
	if err != nil {
		return nil, err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, err
	}
	return FromMnemonic(mnemonic, "")
}

// FromMnemonic restores identity from BIP39 words (+ optional passphrase).
func FromMnemonic(mnemonic, passphrase string) (*Identity, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, passphrase)
	// HD: HMAC-SHA512("MST seed", seed) → master; single hardened path for identity.
	mac := hmac.New(sha512.New, []byte("MST seed"))
	_, _ = mac.Write(seed)
	I := mac.Sum(nil)
	// further harden with path index 0'
	mac2 := hmac.New(sha512.New, I[32:])
	var idx [5]byte
	idx[0] = 0
	binaryPutU32(idx[1:], 0x80000000) // hardened 0
	_, _ = mac2.Write(append(I[:32], idx[:]...))
	child := mac2.Sum(nil)

	edSeed := child[:32]
	edPriv := ed25519.NewKeyFromSeed(edSeed)
	edPub := edPriv.Public().(ed25519.PublicKey)

	var xPriv [32]byte
	copy(xPriv[:], child[32:64])
	// X25519 clamp
	xPriv[0] &= 248
	xPriv[31] &= 127
	xPriv[31] |= 64
	var xPub [32]byte
	curve25519.ScalarBaseMult(&xPub, &xPriv)

	id := &Identity{
		Mnemonic:     mnemonic,
		NodeID:       protocol.NodeIDFromPub(edPub),
		Fingerprint:  protocol.Fingerprint16(edPub),
		Ed25519Pub:   base64.StdEncoding.EncodeToString(edPub),
		X25519Pub:    base64.StdEncoding.EncodeToString(xPub[:]),
		CreatedAtRFC: time.Now().UTC().Format(time.RFC3339),
		edPriv:       edPriv,
		xPriv:        xPriv,
	}
	return id, nil
}

func binaryPutU32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

// collectEntropy multi-source pool → SHA-512 (whitepaper §3.2).
func collectEntropy(n int) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	// OS CSPRNG
	csprng := make([]byte, 64)
	if _, err := rand.Read(csprng); err != nil {
		return nil, err
	}
	buf = append(buf, csprng...)
	// wall + mono-ish times
	now := time.Now()
	tbytes := make([]byte, 24)
	binaryPutU64(tbytes[0:], uint64(now.UnixNano()))
	binaryPutU64(tbytes[8:], uint64(now.UnixMicro()))
	binaryPutU64(tbytes[16:], uint64(time.Since(time.Unix(0, 0))))
	buf = append(buf, tbytes...)
	// process / host
	buf = append(buf, []byte(fmt.Sprintf("%d|%d", os.Getpid(), os.Getppid()))...)
	if h, err := os.Hostname(); err == nil {
		buf = append(buf, []byte(h)...)
	}
	// disk IO latency sample
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("msp-ent-%d", now.UnixNano()))
	t0 := time.Now()
	_ = os.WriteFile(tmp, csprng, 0o600)
	_, _ = os.ReadFile(tmp)
	_ = os.Remove(tmp)
	ioLat := make([]byte, 8)
	binaryPutU64(ioLat, uint64(time.Since(t0).Nanoseconds()))
	buf = append(buf, ioLat...)
	// scheduling jitter
	t1 := time.Now()
	runtime.Gosched()
	j := make([]byte, 8)
	binaryPutU64(j, uint64(time.Since(t1).Nanoseconds()))
	buf = append(buf, j...)
	// network interface + (best-effort) traffic counters as time-varying entropy
	if ifs, err := net.Interfaces(); err == nil {
		for _, iface := range ifs {
			buf = append(buf, []byte(iface.Name)...)
			buf = append(buf, iface.HardwareAddr...)
			// MTU / flags change rarely but add diversity
			var mb [8]byte
			binaryPutU64(mb[:], uint64(iface.MTU))
			buf = append(buf, mb[:]...)
			if addrs, err := iface.Addrs(); err == nil {
				for _, a := range addrs {
					buf = append(buf, []byte(a.String())...)
				}
			}
		}
	}
	// Linux: /proc/net/dev packet counters (micro-variation over time)
	if raw, err := os.ReadFile("/proc/net/dev"); err == nil {
		// sample up to 512 bytes mid-file for counter noise
		if len(raw) > 512 {
			buf = append(buf, raw[len(raw)/2:len(raw)/2+256]...)
		} else {
			buf = append(buf, raw...)
		}
	}
	// Windows: no /proc; use high-res clock samples as packet-timing surrogate
	for i := 0; i < 8; i++ {
		t := time.Now().UnixNano()
		var tb [8]byte
		binaryPutU64(tb[:], uint64(t))
		buf = append(buf, tb[:]...)
		runtime.Gosched()
	}
	// extra CSPRNG
	extra := make([]byte, 64)
	_, _ = rand.Read(extra)
	buf = append(buf, extra...)
	// CPU timing loop jitter (software RDTSC surrogate)
	var sink uint64
	t2 := time.Now()
	for i := 0; i < 10000; i++ {
		sink += uint64(i) * uint64(now.Nanosecond())
	}
	_ = sink
	loop := make([]byte, 8)
	binaryPutU64(loop, uint64(time.Since(t2).Nanoseconds()))
	buf = append(buf, loop...)

	sum := sha512.Sum512(buf)
	out := pbkdf2.Key(sum[:], []byte("MSP-entropy-v1"), 4096, n, sha512.New)
	return out, nil
}

func binaryPutU64(b []byte, v uint64) {
	for i := 0; i < 8; i++ {
		b[7-i] = byte(v)
		v >>= 8
	}
}

// EdPrivate returns signing key.
func (id *Identity) EdPrivate() ed25519.PrivateKey { return id.edPriv }

// EdPublic returns verifying key.
func (id *Identity) EdPublic() ed25519.PublicKey {
	return id.edPriv.Public().(ed25519.PublicKey)
}

// XPrivate returns X25519 private scalar.
func (id *Identity) XPrivate() [32]byte { return id.xPriv }

// XPublicBytes returns X25519 public key bytes.
func (id *Identity) XPublicBytes() []byte {
	raw, _ := base64.StdEncoding.DecodeString(id.X25519Pub)
	return raw
}

// Save writes identity to path. If passphrase non-empty, mnemonic is AES-GCM encrypted.
// Messages are NEVER written.
func (id *Identity) Save(path string, filePassphrase string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	type fileFmt struct {
		Version     int    `json:"v"`
		Mnemonic    string `json:"mnemonic,omitempty"`
		EncMnemonic string `json:"enc_mnemonic,omitempty"`
		EncNonce    string `json:"enc_nonce,omitempty"`
		EncSalt     string `json:"enc_salt,omitempty"`
		NodeID      string `json:"node_id"`
		Fingerprint string `json:"fingerprint"`
		Ed25519Pub  string `json:"ed25519_pub"`
		X25519Pub   string `json:"x25519_pub"`
		CreatedAt   string `json:"created_at"`
		Note        string `json:"note"`
	}
	f := fileFmt{
		Version:     2,
		NodeID:      id.NodeID,
		Fingerprint: id.Fingerprint,
		Ed25519Pub:  id.Ed25519Pub,
		X25519Pub:   id.X25519Pub,
		CreatedAt:   id.CreatedAtRFC,
		Note:        "Identity only. Messages are memory-only and never restored.",
	}
	if filePassphrase == "" {
		f.Mnemonic = id.Mnemonic
	} else {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
		key := pbkdf2.Key([]byte(filePassphrase), salt, 100000, 32, sha512.New)
		block, err := aes.NewCipher(key)
		if err != nil {
			return err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return err
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return err
		}
		ct := gcm.Seal(nil, nonce, []byte(id.Mnemonic), nil)
		f.EncMnemonic = base64.StdEncoding.EncodeToString(ct)
		f.EncNonce = base64.StdEncoding.EncodeToString(nonce)
		f.EncSalt = base64.StdEncoding.EncodeToString(salt)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Load reads identity from disk and re-derives keys from mnemonic.
// filePassphrase decrypts enc_mnemonic when present.
func Load(path string, filePassphrase string) (*Identity, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file struct {
		Mnemonic    string `json:"mnemonic"`
		EncMnemonic string `json:"enc_mnemonic"`
		EncNonce    string `json:"enc_nonce"`
		EncSalt     string `json:"enc_salt"`
		Passphrase  string `json:"passphrase"` // bip39 passphrase (legacy field)
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	mnemonic := file.Mnemonic
	if mnemonic == "" && file.EncMnemonic != "" {
		if filePassphrase == "" {
			return nil, errors.New("identity is encrypted — set MSP_PASSPHRASE")
		}
		salt, err := base64.StdEncoding.DecodeString(file.EncSalt)
		if err != nil {
			return nil, err
		}
		nonce, err := base64.StdEncoding.DecodeString(file.EncNonce)
		if err != nil {
			return nil, err
		}
		ct, err := base64.StdEncoding.DecodeString(file.EncMnemonic)
		if err != nil {
			return nil, err
		}
		key := pbkdf2.Key([]byte(filePassphrase), salt, 100000, 32, sha512.New)
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		pt, err := gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			return nil, errors.New("wrong passphrase or corrupt identity")
		}
		mnemonic = string(pt)
	}
	if mnemonic == "" {
		return nil, errors.New("identity file missing mnemonic")
	}
	return FromMnemonic(mnemonic, file.Passphrase)
}

// PublicHex helpers for display.
func (id *Identity) EdPublicHex() string {
	return hex.EncodeToString(id.EdPublic())
}
