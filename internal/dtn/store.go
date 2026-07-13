// Package dtn: store-carry-forward for offline peers.
// Ciphertext only; optionally persisted to disk (no plaintext).
package dtn

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Item pending encrypted packet.
type Item struct {
	Raw     []byte    `json:"-"`
	RawB64  string    `json:"raw_b64"`
	Expire  time.Time `json:"expire"`
	Target  string    `json:"target"`
	Except  string    `json:"except"`
	Created time.Time `json:"created"`
}

// Store bounded pending queue with optional disk persistence.
type Store struct {
	mu    sync.Mutex
	items []Item
	max   int
	path  string // empty = memory only
}

// New creates memory-only store.
func New(max int) *Store {
	if max <= 0 {
		max = 1024
	}
	return &Store{max: max}
}

// Open loads or creates a persistent store at path (JSON of base64 ciphertexts).
func Open(path string, max int) (*Store, error) {
	s := New(max)
	s.path = path
	if path == "" {
		return s, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var list []Item
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	now := time.Now()
	for _, it := range list {
		if !it.Expire.IsZero() && !now.Before(it.Expire) {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(it.RawB64)
		if err != nil {
			continue
		}
		it.Raw = b
		s.items = append(s.items, it)
	}
	return s, nil
}

// Push queues ciphertext.
func (s *Store) Push(raw []byte, expire time.Time, target, except string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if len(s.items) >= s.max {
		s.items = s.items[1:]
	}
	cp := make([]byte, len(raw))
	copy(cp, raw)
	s.items = append(s.items, Item{
		Raw: cp, RawB64: base64.StdEncoding.EncodeToString(cp),
		Expire: expire, Target: target, Except: except, Created: time.Now(),
	})
	_ = s.saveLocked()
}

// PopReady returns all non-expired items and clears queue (caller re-floods).
// Successfully re-queued items should Push again if send fails.
func (s *Store) PopReady() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	out := make([]Item, len(s.items))
	copy(out, s.items)
	// deep copy Raw
	for i := range out {
		cp := make([]byte, len(out[i].Raw))
		copy(cp, out[i].Raw)
		out[i].Raw = cp
	}
	s.items = nil
	_ = s.saveLocked()
	return out
}

// PeekLen without clearing.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	return len(s.items)
}

func (s *Store) pruneLocked() {
	now := time.Now()
	dst := s.items[:0]
	for _, it := range s.items {
		if it.Expire.IsZero() || now.Before(it.Expire) {
			dst = append(dst, it)
		}
	}
	s.items = dst
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	// ensure RawB64 set
	for i := range s.items {
		if s.items[i].RawB64 == "" && len(s.items[i].Raw) > 0 {
			s.items[i].RawB64 = base64.StdEncoding.EncodeToString(s.items[i].Raw)
		}
	}
	raw, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}
