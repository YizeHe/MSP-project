// Package alert implements official Alert authority (whitepaper §7).
package alert

import (
	"crypto/ed25519"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/YizeHe/MSP-project/internal/mst"
)

// NetworkAlertSeed is the deterministic seed for the unique Alert keypair.
// After 3 years (mst.AlertOpenDeadline), this seed is considered public.
const NetworkAlertSeed = "MSP-NETWORK-ALERT-V1-UNIQUE-AUTHORITY-DO-NOT-USE-FOR-ROUTING"

// PublicKey returns the hard-coded network Alert Ed25519 public key.
func PublicKey() ed25519.PublicKey {
	_, pub := keyPair()
	return pub
}

// PublicKeyB64 base64.
func PublicKeyB64() string {
	return base64.StdEncoding.EncodeToString(PublicKey())
}

// NodeID of alert authority (SHA1 of Ed25519 pubkey — same as protocol).
func NodeID() string {
	h := sha1.Sum(PublicKey())
	return hex.EncodeToString(h[:])
}

// PrivateKey returns Alert private key only when MSP_ALERT_SEED is set (operator).
// Hard-coded NetworkAlertSeed is PUBLIC and must NOT mint a usable private key in-process.
func PrivateKey() (ed25519.PrivateKey, error) {
	if v := os.Getenv("MSP_ALERT_SEED"); v != "" {
		sum := sha512.Sum512([]byte(v))
		return ed25519.NewKeyFromSeed(sum[:32]), nil
	}
	return nil, errors.New("alert private key unavailable — set MSP_ALERT_SEED (do not use public NetworkAlertSeed)")
}

func keyPair() (ed25519.PrivateKey, ed25519.PublicKey) {
	// Canonical network public identity is derived from NetworkAlertSeed for IsAlertSender
	// and genesis init_alert. Private material requires explicit MSP_ALERT_SEED.
	sum := sha512.Sum512([]byte(NetworkAlertSeed))
	priv := ed25519.NewKeyFromSeed(sum[:32])
	return priv, priv.Public().(ed25519.PublicKey)
}

// IsAlertSender true if sender pub matches network alert.
func IsAlertSender(pubB64 string) bool {
	raw, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.PublicKey(raw).Equal(PublicKey())
}

// Status for CLI/GUI.
func Status() map[string]any {
	_, hasPriv := os.LookupEnv("MSP_ALERT_SEED")
	return map[string]any{
		"alert_pubkey":   PublicKeyB64(),
		"alert_node_id":  NodeID(),
		"genesis":        mst.NetworkGenesis.Format(time.RFC3339),
		"open_deadline":  mst.AlertOpenDeadline().Format(time.RFC3339),
		"should_publish": mst.AlertShouldPublish(),
		// Do not echo private seed material; NetworkAlertSeed is a public label only after deadline.
		"seed_label":      NetworkAlertSeed,
		"seed_published":  mst.AlertShouldPublish(),
		"operator_privkey": hasPriv,
	}
}
