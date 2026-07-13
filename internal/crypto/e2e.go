// Package crypto: AES-GCM seal + explicit HMAC-SHA512 MAC (whitepaper §5.2).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// DeriveSessionKey legacy single ECDH (fallback). Prefer X3DH.
func DeriveSessionKey(myPriv [32]byte, theirPub []byte, info string) ([]byte, error) {
	if len(theirPub) != 32 {
		return nil, errors.New("x25519 public key must be 32 bytes")
	}
	shared, err := curve25519.X25519(myPriv[:], theirPub)
	if err != nil {
		return nil, err
	}
	allZero := true
	for _, b := range shared {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil, errors.New("invalid ECDH shared secret")
	}
	r := hkdf.New(sha512.New, shared, []byte("MSP-v1"), []byte(info))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

// Seal encrypts + returns nonce, ciphertext+GCM tag, and explicit HMAC-SHA512 MAC.
func Seal(key, plaintext []byte) (nonceB64, cipherB64, macB64 string, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", "", err
	}
	out := gcm.Seal(nil, nonce, plaintext, nil)
	mac := HMAC(key, nonce, out)
	return base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(out),
		base64.StdEncoding.EncodeToString(mac),
		nil
}

// Open decrypts after verifying explicit MAC.
func Open(key []byte, nonceB64, cipherB64, macB64 string) ([]byte, error) {
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return nil, err
	}
	if macB64 != "" {
		mac, err := base64.StdEncoding.DecodeString(macB64)
		if err != nil {
			return nil, err
		}
		want := HMAC(key, nonce, ct)
		if !hmac.Equal(mac, want) {
			return nil, errors.New("payload MAC mismatch")
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, nil)
}

// HMAC SHA-512 over nonce||ciphertext.
func HMAC(key, nonce, ct []byte) []byte {
	m := hmac.New(sha512.New, key)
	_, _ = m.Write(nonce)
	_, _ = m.Write(ct)
	return m.Sum(nil)
}

// BroadcastKey for public floods (signed content; key is public domain).
func BroadcastKey() []byte {
	sum := sha512.Sum512([]byte("MSP-broadcast-public-v1"))
	return sum[:32]
}
