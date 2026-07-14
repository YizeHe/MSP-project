// Engine: PoS full node — economic txs + slot proposers (mainnet-2).
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
	mining  bool // proposing
}

// OpenEngine loads or creates mainnet-2 genesis.
func OpenEngine(dir string, id *identity.Identity) (*Engine, error) {
	st, err := OpenStore(dir)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		ID: id, Store: st, State: NewState(), Mempool: NewMempool(5000),
		Log: func(string) {}, stop: make(chan struct{}),
	}
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
		e.Log(fmt.Sprintf("mainnet-2 genesis installed chain_id=%s hash=%s consensus=pos",
			ChainID, g.Header.HashHex()[:16]))
	} else {
		g0 := st.GetByHeight(0)
		if g0 == nil {
			return nil, fmt.Errorf("missing genesis block at height 0")
		}
		if err := ValidateMainnetGenesis(g0); err != nil {
			return nil, fmt.Errorf("stored chain is not %s: %w — use a fresh MSP_DATA", ChainID, err)
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
		e.Log(fmt.Sprintf("mainnet-2 loaded tip=%d", st.Height()))
	}
	return e, nil
}

// Close proposer loop.
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
	tipHash, h, slot := "", uint64(0), uint64(0)
	if tip != nil {
		tipHash = tip.Header.HashHex()
		h = tip.Header.Height
		slot = tip.Header.Slot
	}
	acc := e.State.GetAccount(e.ID.NodeID)
	gHash := ""
	if g0 := e.Store.GetByHeight(0); g0 != nil {
		gHash = g0.Header.HashHex()
	}
	cur := CurrentSlot()
	prevHash := hex32zero()
	if tip != nil {
		prevHash = tip.Header.HashHex()
	}
	nextProp := SelectProposer(e.State, prevHash, cur)
	return map[string]any{
		"network":              NetworkName,
		"chain_id":             ChainID,
		"consensus":            "pos-eth-inspired",
		"mode":                 "economy-only",
		"height":               h,
		"tip":                  tipHash,
		"tip_slot":             slot,
		"current_slot":         cur,
		"next_proposer":        nextProp,
		"i_am_next":            nextProp == e.ID.NodeID || (nextProp == "" && cur < BootstrapSlots),
		"genesis_hash":         gHash,
		"mempool":              e.Mempool.Len(),
		"proposing":            e.mining,
		"state_root":           e.State.Root(),
		"my_balance":           acc.Balance,
		"my_stake":             acc.Stake,
		"my_active":            acc.Active,
		"my_nonce":             acc.Nonce,
		"my_burned":            acc.Burned,
		"my_registered":        acc.RegHeight > 0,
		"total_active_stake":   e.State.TotalActiveStake(),
		"validators":           len(e.State.ActiveValidators()),
		"reward_pool_remaining": e.State.MinerPoolRemaining,
		"genesis_supply":       GenesisSupply,
		"block_reward_next":    BlockReward(h + 1),
		"slot_duration":        SlotDuration.String(),
		"min_stake":            MinStake,
		"bootstrap_slots":      BootstrapSlots,
		"free_claim":           false,
		"note":                 "mainnet-2 PoS: no free claim; earn MST by proposing; stake for weight",
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
	e.mu.Lock()
	defer e.mu.Unlock()
	acc := e.State.GetAccount(e.ID.NodeID)
	tx := &Transaction{
		Type: typ, Sender: e.ID.NodeID, SenderPub: e.ID.Ed25519Pub,
		Nonce: acc.Nonce, Fee: fee, Data: EncodeData(data),
	}
	tx.Sign(e.ID.EdPrivate())
	return tx, nil
}

// ClaimGenesis disabled.
func (e *Engine) ClaimGenesis() (*Transaction, error) {
	return nil, errClaimDisabled
}

// Transfer.
func (e *Engine) Transfer(to string, amount uint64) (*Transaction, error) {
	return e.BuildAndSignTx(TxTransfer, FeeTransfer, TransferData{To: to, Amount: amount})
}

// Register.
func (e *Engine) Register() (*Transaction, error) {
	return e.BuildAndSignTx(TxRegister, FeeRegister, RegisterData{
		Ed25519Pub: e.ID.Ed25519Pub, X25519Pub: e.ID.X25519Pub,
	})
}

// Stake locks liquid → stake.
func (e *Engine) Stake(amount uint64) (*Transaction, error) {
	return e.BuildAndSignTx(TxStake, FeeStake, StakeData{Amount: amount})
}

// Unstake stake → liquid.
func (e *Engine) Unstake(amount uint64) (*Transaction, error) {
	return e.BuildAndSignTx(TxUnstake, FeeUnstake, UnstakeData{Amount: amount})
}

// Activate joins validator set.
func (e *Engine) Activate() (*Transaction, error) {
	return e.BuildAndSignTx(TxActivate, 0, ActivateData{Ed25519Pub: e.ID.Ed25519Pub})
}

// BurnForMessage.
func (e *Engine) BurnForMessage(msgType, refHash string) (*Transaction, error) {
	amount := BurnAmountForMsgType(msgType)
	return e.BuildAndSignTx(TxBurn, FeeBurnBase, BurnData{
		MsgType: msgType, RefHash: refHash,
		Timestamp: time.Now().UnixMicro(), Amount: amount,
	})
}

// HasTxHash.
func (e *Engine) HasTxHash(hash string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Mempool.Has(hash) {
		return true
	}
	return e.State.HasBurnTx(hash)
}

// IssueTicketForBurn.
func (e *Engine) IssueTicketForBurn(burnTx *Transaction) (*BurnTicket, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.Mempool.Has(burnTx.TxID()) && !e.State.HasBurnTx(burnTx.TxID()) {
		if burnTx.Type != TxBurn {
			return nil, fmt.Errorf("burn tx not in mempool")
		}
	}
	hint := e.Store.Height() + 1
	return IssueBurnTicket(burnTx, e.ID.NodeID, e.ID.EdPrivate(), hint)
}

// AnonymousBurn: fund burner → burn → ticket (may need balance from rewards).
func (e *Engine) AnonymousBurn(msgType, refHash string) (ticket *BurnTicket, burnTx *Transaction, err error) {
	amount := BurnAmountForMsgType(msgType)
	fund := amount + FeeBurnBase
	totalFromMain := fund + FeeTransfer

	e.mu.Lock()
	acc := e.State.GetAccount(e.ID.NodeID)
	if acc.Balance < totalFromMain {
		e.mu.Unlock()
		return nil, nil, fmt.Errorf("%w: have %d need %d (anonymous burn)", errInsufficient, acc.Balance, totalFromMain)
	}
	e.mu.Unlock()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	burnerID := nodeIDFromPub(pub)
	burnerPubB64 := base64.StdEncoding.EncodeToString(pub)

	xfer, err := e.BuildAndSignTx(TxTransfer, FeeTransfer, TransferData{To: burnerID, Amount: fund})
	if err != nil {
		return nil, nil, err
	}
	if err := e.SubmitTx(xfer); err != nil {
		return nil, nil, fmt.Errorf("fund burner: %w", err)
	}
	// Propose a block if we can (to apply transfer) — best-effort
	if _, err := e.ProposeOnce(true); err != nil {
		// leave in mempool; caller may wait for next slot
		_ = err
	}

	// After propose, transfer should be applied if we were proposer
	e.mu.Lock()
	bBal := e.State.GetAccount(burnerID).Balance
	e.mu.Unlock()
	if bBal < fund {
		// still pending — burn from main as fallback is NOT anonymous; return error
		return nil, nil, fmt.Errorf("burner not funded yet (wait for block including transfer)")
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
	return ticket, burnTx, err
}

func nodeIDFromPub(pub ed25519.PublicKey) string {
	h := sha1.Sum(pub)
	return hex.EncodeToString(h[:])
}

// AcceptBlock from peer.
func (e *Engine) AcceptBlock(b *Block) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.acceptLocked(b)
}

func (e *Engine) acceptLocked(b *Block) error {
	tip := e.Store.Tip()
	var prev *BlockHeader
	parentState := e.State.Clone()
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
		// parent state is current tip state only if height == tip+1
	}
	if err := ValidateBlock(b, prev, parentState); err != nil {
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
	e.Log(fmt.Sprintf("accepted block h=%d slot=%d proposer=%s", b.Header.Height, b.Header.Slot, shortID(b.Header.Proposer)))
	if e.OnBlock != nil {
		go e.OnBlock(b)
	}
	return nil
}

// ProposeOnce builds and signs a block if we are eligible for current/next slot.
// force ignores election (local bootstrap only when allowed by MayPropose cold-start).
func (e *Engine) ProposeOnce(force bool) (*Block, error) {
	e.mu.Lock()
	txs := e.Mempool.Peek(200)
	tip := e.Store.Tip()
	prevHash := hex32zero()
	height := uint64(1)
	prevSlot := uint64(0)
	if tip != nil {
		prevHash = tip.Header.HashHex()
		height = tip.Header.Height + 1
		prevSlot = tip.Header.Slot
	}
	slot := CurrentSlot()
	if slot <= prevSlot {
		slot = prevSlot + 1 // advance if wall clock behind / same slot already used
	}
	// Timestamp must fall inside the slot window for consensus validation
	tnow := time.Now().UTC()
	slotStart := SlotStartTime(slot)
	slotEnd := slotStart.Add(SlotDuration)
	if tnow.Before(slotStart) {
		tnow = slotStart.Add(time.Second)
	}
	if !tnow.Before(slotEnd) {
		// proposing a future/catch-up slot: use mid-slot time
		tnow = slotStart.Add(SlotDuration / 2)
	}
	if !force && !MayPropose(e.State, prevHash, e.ID.NodeID, slot, height) {
		e.mu.Unlock()
		return nil, fmt.Errorf("%w: not proposer for slot %d (elected=%s)",
			errBadProposer, slot, SelectProposer(e.State, prevHash, slot))
	}
	if force && !MayPropose(e.State, prevHash, e.ID.NodeID, slot, height) {
		// force still requires cold-start allowance
		if SelectProposer(e.State, prevHash, slot) != "" && SelectProposer(e.State, prevHash, slot) != e.ID.NodeID {
			e.mu.Unlock()
			return nil, fmt.Errorf("%w: force denied, elected=%s", errBadProposer, SelectProposer(e.State, prevHash, slot))
		}
	}

	reward := BlockReward(height)
	cl := e.State.Clone()
	if reward > cl.MinerPoolRemaining {
		reward = cl.MinerPoolRemaining
	}
	body := make([]Transaction, 0, len(txs)+2)
	if height > 0 && BlockReward(height) > 0 {
		cb := Transaction{
			Type: TxCoinbase, Sender: e.ID.NodeID, SenderPub: e.ID.Ed25519Pub,
			Nonce: 0, Fee: 0,
			Data: EncodeData(CoinbaseData{Amount: reward, Height: height, Slot: slot}),
		}
		body = append(body, cb)
	}
	// Auto-activate proposer if not active (bootstrap / first block)
	me := e.State.GetAccount(e.ID.NodeID)
	if !me.Active {
		act := &Transaction{
			Type: TxActivate, Sender: e.ID.NodeID, SenderPub: e.ID.Ed25519Pub,
			Nonce: me.Nonce, Fee: 0,
			Data: EncodeData(ActivateData{Ed25519Pub: e.ID.Ed25519Pub}),
		}
		act.Sign(e.ID.EdPrivate())
		// dry-run on clone with coinbase first
		cl2 := e.State.Clone()
		tnow := time.Now().UTC()
		for i := range body {
			_ = cl2.ApplyTx(&body[i], tnow, height)
		}
		if err := cl2.ApplyTx(act, tnow, height); err == nil {
			body = append(body, *act)
		}
	}

	clone := e.State.Clone()
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
	if len(body) == 0 {
		e.mu.Unlock()
		return nil, fmt.Errorf("nothing to propose")
	}

	b := &Block{
		Header: BlockHeader{
			Version: ProtocolVersion, PrevBlock: prevHash,
			Timestamp: tnow.UnixMicro(), Height: height, Slot: slot,
			Proposer: e.ID.NodeID, ProposerPub: e.ID.Ed25519Pub,
			MinerID: e.ID.NodeID, TxCount: uint32(len(body)),
		},
		Txs: body,
	}
	b.Header.MerkleRoot = MerkleRootFromTxs(b.Txs)
	clone2 := e.State.Clone()
	if err := clone2.ApplyBlock(b); err != nil {
		e.mu.Unlock()
		return nil, err
	}
	b.Header.StateRoot = clone2.Root()
	SignBlockHeader(&b.Header, e.ID.EdPrivate())
	b.Header.TxCount = uint32(len(body))

	e.mining = true
	e.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	e.mining = false
	e.Log(fmt.Sprintf("proposed block h=%d slot=%d txs=%d", b.Header.Height, b.Header.Slot, len(b.Txs)))
	if err := e.acceptLocked(b); err != nil {
		return nil, err
	}
	return b, nil
}

// MineOnce is an alias for ProposeOnce (CLI compatibility).
func (e *Engine) MineOnce(force bool) (*Block, error) {
	return e.ProposeOnce(force)
}

// StartMiner runs the slot proposer loop (production: check every few seconds, propose when elected).
func (e *Engine) StartMiner(autoEmpty bool) {
	go func() {
		// Poll frequently; slot is 10m — only propose when eligible
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-e.stop:
				return
			case <-t.C:
				e.mu.Lock()
				tip := e.Store.Tip()
				prevHash := hex32zero()
				if tip != nil {
					prevHash = tip.Header.HashHex()
				}
				slot := CurrentSlot()
				if tip != nil && slot <= tip.Header.Slot {
					e.mu.Unlock()
					continue
				}
				ok := MayPropose(e.State, prevHash, e.ID.NodeID, slot, e.Store.Height()+1)
				mpLen := e.Mempool.Len()
				e.mu.Unlock()
				if !ok {
					continue
				}
				if mpLen == 0 && !autoEmpty {
					continue
				}
				_, err := e.ProposeOnce(false)
				if err != nil {
					// not always an error (race lost slot)
					if e.Mempool.Len() > 0 {
						e.Log("propose: " + err.Error())
					}
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

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
