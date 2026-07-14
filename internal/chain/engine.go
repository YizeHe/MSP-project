// Engine: chain full node — economic txs only (代办4).
package chain

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/YizeHe/MSP-project/internal/identity"
)

// Engine blockchain core.
type Engine struct {
	mu      sync.Mutex
	ID      *identity.Identity
	Store   *Store
	State   *State
	Mempool *Mempool
	OnBlock func(*Block)
	Log     func(string)
	stop    chan struct{}
	mining  bool
}

// OpenEngine loads or creates the mainnet genesis (msp-mainnet-1).
func OpenEngine(dir string, id *identity.Identity) (*Engine, error) {
	st, err := OpenStore(dir)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		ID: id, Store: st, State: NewState(), Mempool: NewMempool(5000),
		Log: func(string) {}, stop: make(chan struct{}),
	}
	// self-check: BuildGenesis is the sole source of truth
	canon := MainnetGenesis()
	if err := ValidateMainnetGenesis(canon); err != nil {
		return nil, fmt.Errorf("internal genesis: %w", err)
	}
	if st.Tip() == nil {
		g := MainnetGenesis()
		if err := e.State.ApplyBlock(g); err != nil {
			return nil, err
		}
		if err := st.Append(g); err != nil {
			return nil, err
		}
		e.Log(fmt.Sprintf("mainnet genesis installed network=%s chain_id=%s hash=%s",
			NetworkName, ChainID, g.Header.HashHex()[:16]))
	} else {
		g0 := st.GetByHeight(0)
		if g0 == nil {
			return nil, fmt.Errorf("missing genesis block at height 0")
		}
		if err := ValidateMainnetGenesis(g0); err != nil {
			return nil, fmt.Errorf("stored chain is not %s: %w — use a fresh MSP_DATA or migrate", ChainID, err)
		}
		for h := uint64(0); h <= st.Height(); h++ {
			b := st.GetByHeight(h)
			if b == nil {
				return nil, fmt.Errorf("missing block %d", h)
			}
			if err := e.State.ApplyBlock(b); err != nil {
				return nil, fmt.Errorf("replay %d: %w", h, err)
			}
		}
		e.Log(fmt.Sprintf("mainnet chain loaded tip=%d network=%s", st.Height(), NetworkName))
	}
	return e, nil
}

// Close miner.
func (e *Engine) Close() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
}

// TipHeight.
func (e *Engine) TipHeight() uint64 { return e.Store.Height() }

// Status.
func (e *Engine) Status() map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	tip := e.Store.Tip()
	tipHash, h := "", uint64(0)
	if tip != nil {
		tipHash = tip.Header.HashHex()
		h = tip.Header.Height
	}
	acc := e.State.GetAccount(e.ID.NodeID)
	gHash := ""
	if g0 := e.Store.GetByHeight(0); g0 != nil {
		gHash = g0.Header.HashHex()
	}
	return map[string]any{
		"network":              NetworkName,
		"chain_id":             ChainID,
		"mode":                 "economy-only",
		"height":               h,
		"tip":                  tipHash,
		"genesis_hash":         gHash,
		"mempool":              e.Mempool.Len(),
		"mining":               e.mining,
		"state_root":           e.State.Root(),
		"my_balance":           acc.Balance,
		"my_nonce":             acc.Nonce,
		"my_claimed":           acc.Claimed,
		"my_burned":            acc.Burned,
		"my_registered":        acc.RegHeight > 0,
		"total_claimed":        e.State.TotalClaimed,
		"claimable_supply":     ClaimableSupply,
		"genesis_grant":        GenesisGrant,
		"max_claim_nodes":      MaxClaimNodes,
		"claims_used":          e.State.TotalClaimed / GenesisGrant,
		"miner_pool_remaining": e.State.MinerPoolRemaining,
		"genesis_supply":       GenesisSupply,
		"block_reward_next":    BlockReward(h + 1),
		"block_interval":       TargetInterval().String(),
		"note":                 "MSP mainnet: chain holds MST ledger + burn proofs only; messages are DTN-only",
	}
}

// SubmitTx to mempool after dry-run.
func (e *Engine) SubmitTx(tx *Transaction) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	clone := e.State.Clone()
	if err := clone.ApplyTx(tx, time.Now(), e.Store.Height()+1); err != nil {
		return err
	}
	return e.Mempool.Add(tx)
}

// BuildAndSignTx.
func (e *Engine) BuildAndSignTx(typ string, fee uint64, data any) (*Transaction, error) {
	acc := e.State.GetAccount(e.ID.NodeID)
	tx := &Transaction{
		Type: typ, Sender: e.ID.NodeID, SenderPub: e.ID.Ed25519Pub,
		Nonce: acc.Nonce, Fee: fee, Data: EncodeData(data),
	}
	tx.Sign(e.ID.EdPrivate())
	return tx, nil
}

// ClaimGenesis tx.
func (e *Engine) ClaimGenesis() (*Transaction, error) {
	acc := e.State.GetAccount(e.ID.NodeID)
	if acc.Claimed {
		return nil, errAlreadyClaimed
	}
	return e.BuildAndSignTx(TxGenesisClaim, 0, map[string]any{})
}

// Transfer.
func (e *Engine) Transfer(to string, amount uint64) (*Transaction, error) {
	return e.BuildAndSignTx(TxTransfer, FeeTransfer, TransferData{To: to, Amount: amount})
}

// Register keys on chain.
func (e *Engine) Register() (*Transaction, error) {
	return e.BuildAndSignTx(TxRegister, FeeRegister, RegisterData{
		Ed25519Pub: e.ID.Ed25519Pub,
		X25519Pub:  e.ID.X25519Pub,
	})
}

// BurnForMessage creates burn from main identity (non-anonymous).
func (e *Engine) BurnForMessage(msgType, refHash string) (*Transaction, error) {
	amount := BurnAmountForMsgType(msgType)
	return e.BuildAndSignTx(TxBurn, FeeBurnBase, BurnData{
		MsgType: msgType, RefHash: refHash,
		Timestamp: time.Now().UnixMicro(), Amount: amount,
	})
}

// CanAfford main account.
func (e *Engine) CanAfford(msgType string) error {
	acc := e.State.GetAccount(e.ID.NodeID)
	need := BurnAmountForMsgType(msgType) + FeeBurnBase
	// anonymous path also needs transfer amount+fees ~ need + FeeTransfer + FeeBurnBase
	anonNeed := need + FeeTransfer + FeeBurnBase
	if acc.Balance < need {
		return fmt.Errorf("%w: have %d need %d for %s", errInsufficient, acc.Balance, need, msgType)
	}
	_ = anonNeed
	return nil
}

// HasTxHash mempool or confirmed burn.
func (e *Engine) HasTxHash(hash string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Mempool.Has(hash) {
		return true
	}
	return e.State.HasBurnTx(hash)
}

// IssueTicketForBurn signs BurnTicket for a burn tx already in mempool (local issuer).
func (e *Engine) IssueTicketForBurn(burnTx *Transaction) (*BurnTicket, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.Mempool.Has(burnTx.TxID()) && !e.State.HasBurnTx(burnTx.TxID()) {
		// still allow if just submitted in same call path — check type
		if burnTx.Type != TxBurn {
			return nil, fmt.Errorf("burn tx not in mempool")
		}
	}
	hint := e.Store.Height() + 1
	return IssueBurnTicket(burnTx, e.ID.NodeID, e.ID.EdPrivate(), hint)
}

// AnonymousBurn: transfer MST to one-time burner, burn from burner, issue ticket (代办5 修复二).
func (e *Engine) AnonymousBurn(msgType, refHash string) (ticket *BurnTicket, burnTx *Transaction, err error) {
	amount := BurnAmountForMsgType(msgType)
	// need: amount + FeeBurnBase for burn, plus FeeTransfer for funding burner, plus transfer amount covering burn total
	fund := amount + FeeBurnBase
	totalFromMain := fund + FeeTransfer

	e.mu.Lock()
	acc := e.State.GetAccount(e.ID.NodeID)
	if acc.Balance < totalFromMain {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: have %d need %d (anonymous burn)", errInsufficient, acc.Balance, totalFromMain)
	}
	e.mu.Unlock()

	// burner keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	burnerID := nodeIDFromPub(pub)
	burnerPubB64 := base64.StdEncoding.EncodeToString(pub)

	// 1) transfer main → burner
	xfer, err := e.BuildAndSignTx(TxTransfer, FeeTransfer, TransferData{To: burnerID, Amount: fund})
	if err != nil {
		return nil, nil, err
	}
	if err := e.SubmitTx(xfer); err != nil {
		return nil, nil, fmt.Errorf("fund burner: %w", err)
	}

	// 2) burn from burner (nonce 0 on fresh account after transfer is applied only on mine —
	// dry-run issue: transfer not yet applied so burner nonce is 0 but balance 0 on clone)
	// Fix: mine a block packing both, OR apply optimistically.
	// Speculative: mine immediately after both in mempool with ordered nonces.
	// Burner account after transfer apply has nonce 0 and balance fund.
	// On dry-run SubmitTx for burn, clone applies from current state without transfer.
	// Solution: MineOnce after transfer only, then burn — two blocks. Heavy but correct.
	if _, err := e.MineOnce(false); err != nil {
		return nil, nil, fmt.Errorf("mine fund: %w", err)
	}

	burnTx = &Transaction{
		Type: TxBurn, Sender: burnerID, SenderPub: burnerPubB64,
		Nonce: 0, Fee: FeeBurnBase,
		Data: EncodeData(BurnData{
			MsgType: msgType, RefHash: refHash,
			Timestamp: time.Now().UnixMicro(), Amount: amount,
		}),
	}
	burnTx.Sign(priv)
	if err := e.SubmitTx(burnTx); err != nil {
		return nil, nil, fmt.Errorf("burn: %w", err)
	}
	ticket, err = e.IssueTicketForBurn(burnTx)
	if err != nil {
		return nil, nil, err
	}
	// keep burn in mempool for HasTx; optional mine later
	return ticket, burnTx, nil
}

func nodeIDFromPub(pub ed25519.PublicKey) string {
	h := sha1.Sum(pub)
	return hex.EncodeToString(h[:])
}

// AcceptBlock.
func (e *Engine) AcceptBlock(b *Block) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.acceptLocked(b)
}

func (e *Engine) acceptLocked(b *Block) error {
	tip := e.Store.Tip()
	var prev *BlockHeader
	if tip != nil {
		ph := tip.Header
		prev = &ph
		if b.Header.Height <= tip.Header.Height {
			if b.Header.HashHex() == tip.Header.HashHex() {
				return nil
			}
			if b.Header.Height < tip.Header.Height {
				return nil
			}
		}
	}
	if err := ValidateBlock(b, prev); err != nil {
		return err
	}
	clone := e.State.Clone()
	if err := clone.ApplyBlock(b); err != nil {
		return err
	}
	if clone.Root() != b.Header.StateRoot {
		return errBadState
	}
	if err := e.State.ApplyBlock(b); err != nil {
		return err
	}
	if err := e.Store.Append(b); err != nil {
		return err
	}
	ids := make([]string, len(b.Txs))
	for i := range b.Txs {
		ids[i] = b.Txs[i].TxID()
	}
	e.Mempool.Remove(ids...)
	e.Log(fmt.Sprintf("accepted block h=%d txs=%d", b.Header.Height, len(b.Txs)))
	if e.OnBlock != nil {
		go e.OnBlock(b)
	}
	return nil
}

// MineOnce.
func (e *Engine) MineOnce(force bool) (*Block, error) {
	e.mu.Lock()
	txs := e.Mempool.Peek(200)
	if len(txs) == 0 && !force {
		e.mu.Unlock()
		return nil, fmt.Errorf("mempool empty")
	}
	tip := e.Store.Tip()
	prevHash := hex32zero()
	height := uint64(0)
	if tip != nil {
		prevHash = tip.Header.HashHex()
		height = tip.Header.Height + 1
	}
	// coinbase from miner reward pool (代办5 方案 B)
	reward := BlockReward(height)
	if reward > 0 {
		// capped by remaining pool on clone
		cl := e.State.Clone()
		if reward > cl.MinerPoolRemaining {
			reward = cl.MinerPoolRemaining
		}
	}
	body := make([]Transaction, 0, len(txs)+1)
	if reward > 0 && height > 0 {
		cb := Transaction{
			Type: TxCoinbase, Sender: e.ID.NodeID, SenderPub: e.ID.Ed25519Pub,
			Nonce: 0, Fee: 0,
			Data: EncodeData(CoinbaseData{Amount: reward, Height: height}),
		}
		body = append(body, cb)
	}
	clone := e.State.Clone()
	tnow := time.Now()
	// apply coinbase first on clone
	for i := range body {
		if err := clone.ApplyTx(&body[i], tnow, height); err != nil {
			e.mu.Unlock()
			return nil, err
		}
	}
	for _, tx := range txs {
		if err := clone.ApplyTx(tx, tnow, height); err != nil {
			continue
		}
		body = append(body, *tx)
	}
	if len(body) == 0 && !force {
		e.mu.Unlock()
		return nil, fmt.Errorf("no valid txs")
	}
	// if only coinbase and !force and no mempool — skip empty economic activity unless force
	if !force && len(txs) == 0 {
		e.mu.Unlock()
		return nil, fmt.Errorf("mempool empty")
	}
	b := &Block{
		Header: BlockHeader{
			Version: ProtocolVersion, PrevBlock: prevHash,
			Timestamp: tnow.UnixMicro(), Difficulty: DefaultMineBits(),
			Height: height, MinerID: e.ID.NodeID, TxCount: uint32(len(body)),
		},
		Txs: body,
	}
	b.Header.MerkleRoot = MerkleRootFromTxs(b.Txs)
	// recompute state root with full ApplyBlock on fresh clone
	clone2 := e.State.Clone()
	// temporary header fields for apply
	if err := clone2.ApplyBlock(b); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	b.Header.StateRoot = clone2.Root()
	// reset clone2 side effects already only on clone2

	e.mining = true
	e.mu.Unlock()
	took := MineBlock(b, b.Header.Difficulty)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mining = false
	e.Log(fmt.Sprintf("mined block h=%d pow=%s", b.Header.Height, took.Round(time.Millisecond)))
	if err := e.acceptLocked(b); err != nil {
		return nil, err
	}
	return b, nil
}

// StartMiner loop.
func (e *Engine) StartMiner(autoEmpty bool) {
	go func() {
		t := time.NewTicker(TargetInterval())
		defer t.Stop()
		for {
			select {
			case <-e.stop:
				return
			case <-t.C:
				if e.Mempool.Len() == 0 && !autoEmpty {
					continue
				}
				_, err := e.MineOnce(autoEmpty && e.Mempool.Len() == 0)
				if err != nil && e.Mempool.Len() > 0 {
					e.Log("mine: " + err.Error())
				}
			}
		}
	}()
}

// Headers.
func (e *Engine) Headers() []BlockHeader { return e.Store.Headers() }

// GetBlock.
func (e *Engine) GetBlock(h uint64) *Block { return e.Store.GetByHeight(h) }

// Account.
func (e *Engine) Account(id string) Account { return e.State.GetAccount(id) }
