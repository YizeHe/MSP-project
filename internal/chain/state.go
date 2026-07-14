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

// State account ledger + miner reward pool.
type State struct {
	mu                sync.RWMutex
	Accounts          map[string]*Account `json:"accounts"`
	TotalClaimed      uint64              `json:"total_claimed"`
	MinerPoolRemaining uint64             `json:"miner_pool_remaining"`
	Height            uint64              `json:"height"`
	Tip               string              `json:"tip"`
	pendingMinerFees   uint64
	// known burn tx hashes for ticket validation (recent)
	burnTxSeen map[string]bool
}

// NewState empty with full miner pool.
func NewState() *State {
	return &State{
		Accounts:           make(map[string]*Account),
		MinerPoolRemaining:  MinerRewardPool,
		burnTxSeen:         make(map[string]bool),
	}
}

// Clone.
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ns := NewState()
	ns.TotalClaimed = s.TotalClaimed
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

// Root.
func (s *State) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.Accounts))
	for k := range s.Accounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "claimed:%d|pool:%d|height:%d|", s.TotalClaimed, s.MinerPoolRemaining, s.Height)
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

// HasBurnTx true if burn tx id was applied.
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
		// height-0 only; parameters are constants — accept if well-formed
		if height != 0 {
			return fmt.Errorf("network_params only allowed at height 0")
		}
		var d NetworkParamsData
		if err := json.Unmarshal(tx.Data, &d); err != nil {
			return err
		}
		if d.ChainID != ChainID || d.Name != NetworkName {
			return fmt.Errorf("network params chain_id/name mismatch")
		}
		return nil
	case TxCoinbase:
		return s.applyCoinbase(tx, height)
	case TxGenesisClaim:
		return s.applyGenesisClaim(tx, blockTime)
	case TxTransfer:
		return s.applyTransfer(tx)
	case TxBurn:
		return s.applyBurn(tx)
	case TxRegister:
		return s.applyRegister(tx, height)
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
	// Consensus: reward is exactly min(BlockReward(height), pool remaining).
	// Miners cannot mint above the schedule by stuffing CoinbaseData.Amount.
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
	m := s.ensure(tx.Sender)
	m.Balance += want
	return nil
}

func (s *State) applyGenesisClaim(tx *Transaction, blockTime time.Time) error {
	if blockTime.After(NetworkGenesis.Add(ClaimWindow)) {
		return errClaimClosed
	}
	a := s.ensure(tx.Sender)
	if a.Claimed {
		return errAlreadyClaimed
	}
	// 代办6: hard cap on number of free claims (210 nodes × 128 MST)
	claimCount := s.TotalClaimed / GenesisGrant
	if claimCount >= MaxClaimNodes {
		return errSupplyExhausted
	}
	if s.TotalClaimed+GenesisGrant > ClaimableSupply {
		return errSupplyExhausted
	}
	if tx.Nonce != a.Nonce {
		return errBadNonce
	}
	a.Balance += GenesisGrant
	a.Claimed = true
	a.Nonce++
	s.TotalClaimed += GenesisGrant
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
	if from.Balance < fee {
		return errInsufficient
	}
	from.Balance -= fee
	from.Nonce++
	from.Ed25519Pub = d.Ed25519Pub
	from.X25519Pub = d.X25519Pub
	if from.RegHeight == 0 {
		from.RegHeight = height
	}
	s.pendingMinerFees += fee
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
	if s.pendingMinerFees > 0 && b.Header.MinerID != "" && b.Header.MinerID != "network-genesis" {
		m := s.ensure(b.Header.MinerID)
		m.Balance += s.pendingMinerFees
	}
	s.Height = b.Header.Height
	s.Tip = b.Header.HashHex()
	s.pendingMinerFees = 0
	return nil
}
