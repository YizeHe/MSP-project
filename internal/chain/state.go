package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// State account ledger + reward pool + validator flags.
type State struct {
	mu                 sync.RWMutex
	Accounts           map[string]*Account `json:"accounts"`
	MinerPoolRemaining uint64              `json:"miner_pool_remaining"`
	Height             uint64              `json:"height"`
	Tip                string              `json:"tip"`
	pendingMinerFees    uint64
	burnTxSeen         map[string]bool
}

// NewState empty with full reward pool.
func NewState() *State {
	return &State{
		Accounts:           make(map[string]*Account),
		MinerPoolRemaining: MinerRewardPool,
		burnTxSeen:         make(map[string]bool),
	}
}

// Clone.
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := NewState()
	ns.MinerPoolRemaining = s.MinerPoolRemaining
	ns.Height = s.Height
	ns.Tip = s.Tip
	for k, a := range s.Accounts {
		cp := *a
		ns.Accounts[k] = &cp
	}
	for k, v := range s.burnTxSeen {
		ns.burnTxSeen[k] = v
	}
	return ns
}

// Root includes accounts + pool + active stakes.
func (s *State) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.Accounts))
	for k := range s.Accounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "pool:%d|height:%d|pos:v2|", s.MinerPoolRemaining, s.Height)
	for _, k := range keys {
		raw, _ := json.Marshal(s.Accounts[k])
		sum := sha256.Sum256(raw)
		_, _ = h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GetAccount copy.
func (s *State) GetAccount(id string) Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a, ok := s.Accounts[id]; ok {
		return *a
	}
	return Account{NodeID: id}
}

// HasBurnTx.
func (s *State) HasBurnTx(txHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.burnTxSeen[txHash]
}

// ApplyTx.
func (s *State) ApplyTx(tx *Transaction, blockTime time.Time, height uint64) error {
	if err := tx.Verify(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyTxLocked(tx, blockTime, height)
}

func (s *State) applyTxLocked(tx *Transaction, blockTime time.Time, height uint64) error {
	switch tx.Type {
	case TxInitAlert:
		return nil
	case TxNetworkParams:
		if height != 0 {
			return fmt.Errorf("network_params only at height 0")
		}
		var d NetworkParamsData
		if err := json.Unmarshal(tx.Data, &d); err != nil {
			return err
		}
		if d.ChainID != ChainID || d.Name != NetworkName {
			return fmt.Errorf("network params mismatch")
		}
		if d.FreeClaim {
			return fmt.Errorf("free_claim must be false on mainnet-2")
		}
		return nil
	case TxCoinbase:
		return s.applyCoinbase(tx, height)
	case "genesis_claim":
		return errClaimDisabled
	case TxTransfer:
		return s.applyTransfer(tx)
	case TxBurn:
		return s.applyBurn(tx)
	case TxRegister:
		return s.applyRegister(tx, height)
	case TxStake:
		return s.applyStake(tx)
	case TxUnstake:
		return s.applyUnstake(tx)
	case TxActivate:
		return s.applyActivate(tx, height)
	default:
		return errUnknownTx
	}
}

func (s *State) ensure(id string) *Account {
	a, ok := s.Accounts[id]
	if !ok {
		a = &Account{NodeID: id}
		s.Accounts[id] = a
	}
	return a
}

func (s *State) applyCoinbase(tx *Transaction, height uint64) error {
	var d CoinbaseData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	want := BlockReward(height)
	if want > s.MinerPoolRemaining {
		want = s.MinerPoolRemaining
	}
	if d.Amount != want {
		return fmt.Errorf("%w: amount %d want %d at height %d", errBadCoinbase, d.Amount, want, height)
	}
	if d.Height != 0 && d.Height != height {
		return fmt.Errorf("%w: height field", errBadCoinbase)
	}
	if want == 0 {
		return nil
	}
	s.MinerPoolRemaining -= want
	// Proposer receives to liquid balance (can stake later)
	m := s.ensure(tx.Sender)
	m.Balance += want
	return nil
}

func (s *State) applyTransfer(tx *Transaction) error {
	var d TransferData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	from := s.ensure(tx.Sender)
	if tx.Nonce != from.Nonce {
		return errBadNonce
	}
	fee := tx.Fee
	if fee == 0 {
		fee = FeeTransfer
	}
	total := d.Amount + fee
	if from.Balance < total {
		return errInsufficient
	}
	from.Balance -= total
	from.Nonce++
	to := s.ensure(d.To)
	to.Balance += d.Amount
	s.pendingMinerFees += fee
	return nil
}

func (s *State) applyBurn(tx *Transaction) error {
	var d BurnData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	if d.RefHash == "" {
		return fmt.Errorf("burn requires ref_hash")
	}
	amount := d.Amount
	if amount == 0 {
		amount = BurnAmountForMsgType(d.MsgType)
	}
	fee := tx.Fee
	if fee == 0 {
		fee = FeeBurnBase
	}
	from := s.ensure(tx.Sender)
	if tx.Nonce != from.Nonce {
		return errBadNonce
	}
	total := amount + fee
	if from.Balance < total {
		return errInsufficient
	}
	from.Balance -= total
	from.Burned += amount
	from.Nonce++
	s.pendingMinerFees += fee
	s.burnTxSeen[tx.TxID()] = true
	return nil
}

func (s *State) applyRegister(tx *Transaction, height uint64) error {
	var d RegisterData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	from := s.ensure(tx.Sender)
	if tx.Nonce != from.Nonce {
		return errBadNonce
	}
	fee := tx.Fee
	if fee == 0 {
		fee = FeeRegister
	}
	// bootstrap: allow register with 0 fee if no balance yet and height small
	if from.Balance < fee {
		if height > BootstrapSlots || fee != FeeRegister {
			return errInsufficient
		}
		// free register during bootstrap only
		fee = 0
	} else {
		from.Balance -= fee
		s.pendingMinerFees += fee
	}
	from.Nonce++
	from.Ed25519Pub = d.Ed25519Pub
	from.X25519Pub = d.X25519Pub
	if from.RegHeight == 0 {
		from.RegHeight = height
	}
	return nil
}

func (s *State) applyStake(tx *Transaction) error {
	var d StakeData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	if d.Amount == 0 {
		return fmt.Errorf("stake amount required")
	}
	from := s.ensure(tx.Sender)
	if tx.Nonce != from.Nonce {
		return errBadNonce
	}
	fee := tx.Fee
	if fee == 0 {
		fee = FeeStake
	}
	need := d.Amount + fee
	if from.Balance < need {
		return errInsufficient
	}
	from.Balance -= need
	from.Stake += d.Amount
	from.Nonce++
	s.pendingMinerFees += fee
	return nil
}

func (s *State) applyUnstake(tx *Transaction) error {
	var d UnstakeData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	if d.Amount == 0 {
		return fmt.Errorf("unstake amount required")
	}
	from := s.ensure(tx.Sender)
	if tx.Nonce != from.Nonce {
		return errBadNonce
	}
	if from.Stake < d.Amount {
		return errInsufficient
	}
	fee := tx.Fee
	if fee == 0 {
		fee = FeeUnstake
	}
	// fee from liquid or from unstaked amount
	if from.Balance >= fee {
		from.Balance -= fee
	} else if d.Amount > fee {
		d.Amount -= fee
	} else {
		return errInsufficient
	}
	from.Stake -= d.Amount
	from.Balance += d.Amount
	from.Nonce++
	s.pendingMinerFees += fee
	if from.Stake == 0 {
		from.Active = false
	}
	return nil
}

func (s *State) applyActivate(tx *Transaction, height uint64) error {
	var d ActivateData
	if err := json.Unmarshal(tx.Data, &d); err != nil {
		return err
	}
	from := s.ensure(tx.Sender)
	if tx.Nonce != from.Nonce {
		return errBadNonce
	}
	// After bootstrap, require MinStake
	if height > BootstrapSlots && from.Stake < MinStake {
		return errMinStake
	}
	if d.Ed25519Pub != "" {
		from.Ed25519Pub = d.Ed25519Pub
	}
	from.Active = true
	from.Nonce++
	return nil
}

// ApplyBlock.
func (s *State) ApplyBlock(b *Block) error {
	t := time.UnixMicro(b.Header.Timestamp)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingMinerFees = 0
	for i := range b.Txs {
		if err := b.Txs[i].Verify(); err != nil {
			return fmt.Errorf("tx %d verify: %w", i, err)
		}
		if err := s.applyTxLocked(&b.Txs[i], t, b.Header.Height); err != nil {
			return fmt.Errorf("tx %d (%s): %w", i, b.Txs[i].Type, err)
		}
	}
	// fees to proposer
	proposer := b.Header.Proposer
	if proposer == "" {
		proposer = b.Header.MinerID
	}
	if s.pendingMinerFees > 0 && proposer != "" && proposer != "network-genesis" {
		m := s.ensure(proposer)
		m.Balance += s.pendingMinerFees
	}
	s.Height = b.Header.Height
	s.Tip = b.Header.HashHex()
	s.pendingMinerFees = 0
	return nil
}
