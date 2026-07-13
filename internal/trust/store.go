// Package trust caches peer public keys and detects NodeID conflicts (MITM layer 2).
package trust

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Record is a known peer key binding.
type Record struct {
	NodeID     string `json:"node_id"`
	Ed25519Pub string `json:"ed25519_pub"`
	X25519Pub  string `json:"x25519_pub"`
	TrustedFP  string `json:"trusted_fp,omitempty"` // user-verified fingerprint prefix
	Verified   bool   `json:"verified"`
}

// Store is a thread-safe pubkey cache.
type Store struct {
	mu   sync.RWMutex
	byID map[string]*Record
	path string
}

// Open loads or creates store at path.
func Open(path string) (*Store, error) {
	s := &Store{byID: make(map[string]*Record), path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var list []*Record
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	for _, r := range list {
		s.byID[r.NodeID] = r
	}
	return s, nil
}

// Save persists to disk (keys only, not messages).
func (s *Store) Save() error {
	s.mu.RLock()
	list := make([]*Record, 0, len(s.byID))
	for _, r := range s.byID {
		cp := *r
		list = append(list, &cp)
	}
	s.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

// Observe returns error if conflicting keys for same NodeID.
func (s *Store) Observe(nodeID, ed, x string) error {
	if nodeID == "" || ed == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.byID[nodeID]
	if !ok {
		s.byID[nodeID] = &Record{NodeID: nodeID, Ed25519Pub: ed, X25519Pub: x}
		return nil
	}
	if prev.Ed25519Pub != "" && prev.Ed25519Pub != ed {
		return fmt.Errorf("MITM alert: node_id %s has conflicting ed25519 pubkeys", nodeID)
	}
	if prev.X25519Pub != "" && x != "" && prev.X25519Pub != x {
		return fmt.Errorf("MITM alert: node_id %s has conflicting x25519 pubkeys", nodeID)
	}
	if prev.X25519Pub == "" && x != "" {
		prev.X25519Pub = x
	}
	return nil
}

// SetVerified marks fingerprint as user-verified.
func (s *Store) SetVerified(nodeID, fp string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.byID[nodeID]
	if !ok {
		return fmt.Errorf("unknown peer %s — observe a message first", nodeID)
	}
	r.TrustedFP = fp
	r.Verified = true
	return nil
}

// IsVerified reports user verification.
func (s *Store) IsVerified(nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byID[nodeID]
	return ok && r.Verified
}

// Get returns a copy.
func (s *Store) Get(nodeID string) (*Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byID[nodeID]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// List all records.
func (s *Store) List() []*Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Record, 0, len(s.byID))
	for _, r := range s.byID {
		cp := *r
		out = append(out, &cp)
	}
	return out
}

// RequireVerified if record exists and verified flag required for session.
func (s *Store) RequireVerified(nodeID string, require bool) error {
	if !require {
		return nil
	}
	if !s.IsVerified(nodeID) {
		return fmt.Errorf("peer %s fingerprint not verified — run: msp -c verify-fingerprint", short(nodeID))
	}
	return nil
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
