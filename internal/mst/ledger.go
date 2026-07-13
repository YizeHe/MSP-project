// Package mst implements Message Sending Token economy (whitepaper §8).
// Fixed genesis supply; send burns MST; no mint after genesis claims close.
package mst

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GenesisSupply total MST ever (no inflation). Aligned with chain 代办6/7.
const GenesisSupply uint64 = 4_226_880

// GenesisGrant per identity claim (sum of grants must not exceed supply).
const GenesisGrant uint64 = 128

// CostPerMessage base burn cost (scaled by difficulty).
const CostPerMessage uint64 = 1

// Ledger is local balance + burn tracking (identity-bound).
type Ledger struct {
	mu       sync.Mutex
	path     string
	NodeID   string    `json:"node_id"`
	Balance  uint64    `json:"balance"`
	Burned   uint64    `json:"burned"`
	Claimed  bool      `json:"claimed"`
	Genesis  time.Time `json:"genesis_network"`
	Updated  time.Time `json:"updated"`
}

// NetworkGenesis fixed network birth (for 3-year alert open + genesis).
// Set once at first protocol launch — do not change after release.
var NetworkGenesis = time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

// Open loads or creates ledger for node.
func Open(dir, nodeID string) (*Ledger, error) {
	path := filepath.Join(dir, "mst-ledger.json")
	l := &Ledger{path: path, NodeID: nodeID, Genesis: NetworkGenesis}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, l); err != nil {
		return nil, err
	}
	l.path = path
	return l, nil
}

// Save persists.
func (l *Ledger) Save() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Updated = time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.path, raw, 0o600)
}

// ClaimGenesis one-time grant (simulates genesis PoW allocation).
// Requires powDone=true after caller mined genesis proof.
func (l *Ledger) ClaimGenesis(powDone bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Claimed {
		return errors.New("genesis already claimed")
	}
	if !powDone {
		return errors.New("genesis claim requires valid PoW")
	}
	l.Balance += GenesisGrant
	l.Claimed = true
	return nil
}

// CanSpend checks balance.
func (l *Ledger) CanSpend(amount uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Balance >= amount
}

// Burn spends MST for a message. Returns error if insufficient.
func (l *Ledger) Burn(amount uint64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if amount == 0 {
		amount = CostPerMessage
	}
	if l.Balance < amount {
		return fmt.Errorf("insufficient MST: have %d need %d (claim genesis or lower cost)", l.Balance, amount)
	}
	l.Balance -= amount
	l.Burned += amount
	return nil
}

// Snapshot copy.
func (l *Ledger) Snapshot() (balance, burned uint64, claimed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Balance, l.Burned, l.Claimed
}

// BurnCost for difficulty.
func BurnCost(difficulty int) uint64 {
	if difficulty < 1 {
		return CostPerMessage
	}
	return CostPerMessage * uint64(difficulty)
}

// AlertOpenDeadline returns when Alert private key should be published (3 years).
func AlertOpenDeadline() time.Time {
	return NetworkGenesis.Add(3 * 365 * 24 * time.Hour)
}

// AlertShouldPublish true after 3 years.
func AlertShouldPublish() bool {
	return time.Now().UTC().After(AlertOpenDeadline())
}
