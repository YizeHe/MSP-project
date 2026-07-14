package chain

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YizeHe/MSP-project/internal/identity"
	"github.com/YizeHe/MSP-project/internal/pow"
)

func testEnv(t *testing.T) {
	t.Helper()
	_ = os.Setenv("MSP_POW_FAST", "1")
	_ = os.Setenv("MSP_CHAIN_FAST", "1")
}

func openTestEngine(t *testing.T) (*Engine, *identity.Identity) {
	t.Helper()
	testEnv(t)
	dir := t.TempDir()
	id, err := identity.Generate()
	if err != nil {
		t.Fatal(err)
	}
	e, err := OpenEngine(filepath.Join(dir, "chain"), id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	return e, id
}

func claimAndMine(t *testing.T, e *Engine) {
	t.Helper()
	tx, err := e.ClaimGenesis()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitTx(tx); err != nil {
		t.Fatal(err)
	}
	if _, err := e.MineOnce(false); err != nil {
		t.Fatal(err)
	}
}

func TestEconomyOnlyChain(t *testing.T) {
	e, id := openTestEngine(t)

	claimAndMine(t, e)
	// After claim + MineOnce: GenesisGrant + BlockReward(1) coinbase from miner pool
	wantBal := GenesisGrant + BlockReward(1)
	if e.Account(id.NodeID).Balance != wantBal {
		t.Fatalf("after claim balance %d want %d", e.Account(id.NodeID).Balance, wantBal)
	}

	// burn for dtn message (hash only)
	ref := RefHashFromBytes([]byte("fake-ciphertext-packet"))
	burn, err := e.BurnForMessage(MsgDTN, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitTx(burn); err != nil {
		t.Fatal(err)
	}
	if _, err := e.MineOnce(false); err != nil {
		t.Fatal(err)
	}
	acc := e.Account(id.NodeID)
	// BurnDTN leaves circulation; FeeBurnBase goes to miner.
	// Solo test: miner == sender → fee reclaimed; plus coinbase at height 2.
	want := GenesisGrant + BlockReward(1) + BlockReward(2) - BurnDTN
	if acc.Balance != want {
		t.Fatalf("after burn balance %d want %d burned=%d", acc.Balance, want, acc.Burned)
	}
	if acc.Burned != BurnDTN {
		t.Fatalf("burned %d", acc.Burned)
	}

	// ensure no message types exist as content carriers
	for _, typ := range []string{"direct_msg", "broadcast_msg", "dtn_relay"} {
		tx := &Transaction{Type: typ, Sender: id.NodeID, SenderPub: id.Ed25519Pub, Nonce: acc.Nonce}
		tx.Sign(id.EdPrivate())
		if err := e.SubmitTx(tx); err == nil {
			t.Fatalf("should reject obsolete tx type %s", typ)
		}
	}
}

func TestBurnTicketIssueValidate(t *testing.T) {
	e, id := openTestEngine(t)
	claimAndMine(t, e)

	ref := RefHashPayloadCipher(base64.StdEncoding.EncodeToString([]byte("cipher-bytes-for-ticket")))
	burn, err := e.BurnForMessage(MsgDTN, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitTx(burn); err != nil {
		t.Fatal(err)
	}
	ticket, err := e.IssueTicketForBurn(burn)
	if err != nil {
		t.Fatal(err)
	}
	if err := ticket.Verify(); err != nil {
		t.Fatal(err)
	}
	// success: matching cipher
	cipherB64 := base64.StdEncoding.EncodeToString([]byte("cipher-bytes-for-ticket"))
	if err := ValidateBurnTicket(ticket, cipherB64, e.HasTxHash, time.Now()); err != nil {
		t.Fatalf("validate ok: %v", err)
	}

	// wrong hash
	wrong := base64.StdEncoding.EncodeToString([]byte("other-cipher"))
	if err := ValidateBurnTicket(ticket, wrong, e.HasTxHash, time.Now()); err == nil {
		t.Fatal("expected ref hash mismatch")
	}

	// expired
	old := *ticket
	old.Timestamp = time.Now().Add(-25 * time.Hour).UnixMicro()
	// re-sign with same miner key so Verify still passes but Expired fails
	old.Sign(id.EdPrivate())
	if err := ValidateBurnTicket(&old, cipherB64, e.HasTxHash, time.Now()); err == nil {
		t.Fatal("expected expired ticket")
	}

	// IssueBurnTicket rejects non-burn
	xfer, err := e.Transfer("deadbeef", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := IssueBurnTicket(xfer, id.NodeID, id.EdPrivate(), 1); err == nil {
		t.Fatal("expected not a burn tx")
	}
}

func TestAnonymousBurn(t *testing.T) {
	e, id := openTestEngine(t)
	claimAndMine(t, e)

	before := e.Account(id.NodeID).Balance
	ref := RefHashFromBytes([]byte("anon-cipher"))
	ticket, burnTx, err := e.AnonymousBurn(MsgDTN, ref)
	if err != nil {
		t.Fatal(err)
	}
	if ticket == nil || burnTx == nil {
		t.Fatal("nil ticket/tx")
	}
	if err := ticket.Verify(); err != nil {
		t.Fatal(err)
	}
	if ticket.RefHash != ref {
		t.Fatalf("ref %s want %s", ticket.RefHash, ref)
	}
	if ticket.MsgType != MsgDTN {
		t.Fatalf("msg type %s", ticket.MsgType)
	}
	if !e.HasTxHash(burnTx.TxID()) {
		t.Fatal("burn tx not in mempool/chain")
	}
	if burnTx.Sender == id.NodeID {
		t.Fatal("burner should not equal main identity")
	}
	// main funded burner (transfer mined inside AnonymousBurn) → balance decreased by fund+fee
	// AnonymousBurn mines transfer only; burn stays in mempool.
	// fund = BurnDTN + FeeBurnBase; totalFromMain = fund + FeeTransfer
	// after mine: main lost fund+FeeTransfer, reclaimed FeeTransfer as miner, + coinbase height 2
	// So net from claim block: -fund + BlockReward(2)  (FeeTransfer reclaimed)
	acc := e.Account(id.NodeID)
	// before was GenesisGrant+BlockReward(1); after fund mine:
	// - (BurnDTN+FeeBurnBase+FeeTransfer) + FeeTransfer (miner fee) + BlockReward(2)
	want := before - (BurnDTN + FeeBurnBase) + BlockReward(2)
	if acc.Balance != want {
		t.Fatalf("main balance %d want %d (before=%d)", acc.Balance, want, before)
	}
	// burner still holds fund until burn is mined
	if e.Account(burnTx.Sender).Balance != BurnDTN+FeeBurnBase {
		t.Fatalf("burner bal %d", e.Account(burnTx.Sender).Balance)
	}
}

func TestCoinbasePool(t *testing.T) {
	e, id := openTestEngine(t)

	if e.State.MinerPoolRemaining != MinerRewardPool {
		t.Fatalf("pool start %d", e.State.MinerPoolRemaining)
	}
	if ClaimableSupply != MaxClaimNodes*GenesisGrant {
		t.Fatalf("claimable %d want %d×%d", ClaimableSupply, MaxClaimNodes, GenesisGrant)
	}
	if GenesisSupply != MinerRewardPool+ClaimableSupply {
		t.Fatalf("genesis supply %d", GenesisSupply)
	}
	// 代办6: 128×210 claim pool + 4.2M miner pool
	if ClaimableSupply != 26_880 || MinerRewardPool != 4_200_000 || GenesisGrant != 128 || MaxClaimNodes != 210 {
		t.Fatalf("daiban6 supplies grant=%d maxNodes=%d claimable=%d pool=%d total=%d",
			GenesisGrant, MaxClaimNodes, ClaimableSupply, MinerRewardPool, GenesisSupply)
	}

	// BlockReward schedule (halvings = (height-1)/HalvingInterval, integer div)
	if BlockReward(0) != 0 {
		t.Fatal("h0")
	}
	if BlockReward(1) != 50 {
		t.Fatalf("h1 %d", BlockReward(1))
	}
	if BlockReward(HalvingInterval) != 50 { // (210240-1)/210240 = 0
		t.Fatalf("last pre-halving want 50 got %d", BlockReward(HalvingInterval))
	}
	if BlockReward(HalvingInterval+1) != 25 {
		t.Fatalf("first halving want 25 got %d", BlockReward(HalvingInterval+1))
	}
	if BlockReward(2*HalvingInterval+1) != 12 { // 50/2/2 = 12
		t.Fatalf("second halving want 12 got %d", BlockReward(2*HalvingInterval+1))
	}
	if BlockReward(4*HalvingInterval+1) != 0 {
		t.Fatalf("after 4 halvings want 0 got %d", BlockReward(4*HalvingInterval+1))
	}

	claimAndMine(t, e)
	poolAfter1 := e.State.MinerPoolRemaining
	if poolAfter1 != MinerRewardPool-BlockReward(1) {
		t.Fatalf("pool after h1: %d want %d", poolAfter1, MinerRewardPool-BlockReward(1))
	}

	// another block via burn
	ref := RefHashFromBytes([]byte("pool-test"))
	burn, err := e.BurnForMessage(MsgDirect, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitTx(burn); err != nil {
		t.Fatal(err)
	}
	if _, err := e.MineOnce(false); err != nil {
		t.Fatal(err)
	}
	poolAfter2 := e.State.MinerPoolRemaining
	if poolAfter2 != MinerRewardPool-BlockReward(1)-BlockReward(2) {
		t.Fatalf("pool after h2: %d", poolAfter2)
	}
	// ClaimableSupply is constant (claims only reduce remaining claim capacity, not the constant)
	if ClaimableSupply != 26_880 {
		t.Fatal("ClaimableSupply must stay constant")
	}
	// miner received coinbase
	if e.Account(id.NodeID).Balance < GenesisGrant {
		t.Fatalf("miner bal low %d", e.Account(id.NodeID).Balance)
	}

	// IssueBurnTicket standalone with synthetic burn (no engine)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_ = pub
	tx := &Transaction{
		Type: TxBurn, Sender: "x", SenderPub: base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)),
		Nonce: 0, Fee: FeeBurnBase,
		Data: EncodeData(BurnData{MsgType: MsgDTN, RefHash: ref, Amount: BurnDTN, Timestamp: time.Now().UnixMicro()}),
	}
	tx.Sign(priv)
	tk, err := IssueBurnTicket(tx, "miner", priv, 3)
	if err != nil {
		t.Fatal(err)
	}
	if tk.Amount < BurnDTN {
		t.Fatalf("ticket amount %d", tk.Amount)
	}
}

func TestConsensusRejectsSoftDifficultyAndFatCoinbase(t *testing.T) {
	e, id := openTestEngine(t)
	claimAndMine(t, e)
	// craft a soft-difficulty block tip+1
	tip := e.Store.Tip()
	b := &Block{
		Header: BlockHeader{
			Version: ProtocolVersion, PrevBlock: tip.Header.HashHex(),
			Timestamp: tip.Header.Timestamp + 1, Difficulty: 1, // below consensus min
			Height: tip.Header.Height + 1, MinerID: id.NodeID, TxCount: 1,
		},
	}
	reward := BlockReward(b.Header.Height)
	b.Txs = []Transaction{{
		Type: TxCoinbase, Sender: id.NodeID, SenderPub: id.Ed25519Pub,
		Data: EncodeData(CoinbaseData{Amount: reward, Height: b.Header.Height}),
	}}
	b.Header.MerkleRoot = MerkleRootFromTxs(b.Txs)
	// mine at soft difficulty (attacker)
	MineBlock(b, 1)
	// force header difficulty back to 1 after mine (MineBlock bumps to min)
	b.Header.Difficulty = 1
	mat := headerPoWMaterial(&b.Header)
	// re-mine truly at 1 for valid soft pow
	n, h, _ := pow.Mine(mat, 1)
	b.Header.Nonce, b.Header.PoWHash = n, h
	// state root
	cl := e.State.Clone()
	_ = cl.ApplyBlock(b)
	b.Header.StateRoot = cl.Root()
	if err := e.AcceptBlock(b); err == nil {
		t.Fatal("expected reject soft difficulty")
	}

	// fat coinbase: mine valid difficulty but amount too high
	b2 := &Block{
		Header: BlockHeader{
			Version: ProtocolVersion, PrevBlock: tip.Header.HashHex(),
			Timestamp: tip.Header.Timestamp + 2, Difficulty: ConsensusMinDifficulty,
			Height: tip.Header.Height + 1, MinerID: id.NodeID,
		},
	}
	b2.Txs = []Transaction{{
		Type: TxCoinbase, Sender: id.NodeID, SenderPub: id.Ed25519Pub,
		Data: EncodeData(CoinbaseData{Amount: reward + 999999, Height: b2.Header.Height}),
	}}
	b2.Header.TxCount = 1
	b2.Header.MerkleRoot = MerkleRootFromTxs(b2.Txs)
	MineBlock(b2, ConsensusMinDifficulty)
	if err := e.AcceptBlock(b2); err == nil {
		t.Fatal("expected reject fat coinbase")
	}
}

func TestMainnetGenesis(t *testing.T) {
	g := MainnetGenesis()
	if err := ValidateMainnetGenesis(g); err != nil {
		t.Fatal(err)
	}
	if g.Header.Height != 0 {
		t.Fatal("height")
	}
	if g.Header.HashHex() != MainnetGenesisHash {
		t.Fatalf("hash %s want frozen %s", g.Header.HashHex(), MainnetGenesisHash)
	}
	if len(g.Txs) != 2 {
		t.Fatalf("txs %d", len(g.Txs))
	}
	if g.Txs[0].Type != TxNetworkParams || g.Txs[1].Type != TxInitAlert {
		t.Fatalf("tx types %s %s", g.Txs[0].Type, g.Txs[1].Type)
	}
	info := GenesisInfo()
	if info["chain_id"] != ChainID || info["network"] != NetworkName {
		t.Fatalf("info %+v", info)
	}
	// OpenEngine installs genesis
	e, _ := openTestEngine(t)
	g0 := e.Store.GetByHeight(0)
	if err := ValidateMainnetGenesis(g0); err != nil {
		t.Fatal(err)
	}
	st := e.Status()
	if st["network"] != NetworkName || st["chain_id"] != ChainID {
		t.Fatalf("status %+v", st)
	}
	if st["genesis_hash"] != g.Header.HashHex() {
		t.Fatalf("genesis_hash %v", st["genesis_hash"])
	}
}

func TestMaxClaimNodesCap(t *testing.T) {
	// Unit-level: state rejects claims beyond MaxClaimNodes without full mine loop.
	st := NewState()
	tnow := time.Now()
	// fill TotalClaimed as if MaxClaimNodes already claimed
	st.TotalClaimed = MaxClaimNodes * GenesisGrant
	if st.TotalClaimed != ClaimableSupply {
		t.Fatalf("filled %d want %d", st.TotalClaimed, ClaimableSupply)
	}
	id, err := identity.Generate()
	if err != nil {
		t.Fatal(err)
	}
	tx := &Transaction{
		Type: TxGenesisClaim, Sender: id.NodeID, SenderPub: id.Ed25519Pub,
		Nonce: 0, Fee: 0, Data: EncodeData(struct{}{}),
	}
	tx.Sign(id.EdPrivate())
	if err := st.ApplyTx(tx, tnow, 1); err == nil {
		t.Fatal("expected supply exhausted at MaxClaimNodes")
	}
	// one slot free → ok
	st2 := NewState()
	st2.TotalClaimed = (MaxClaimNodes - 1) * GenesisGrant
	if err := st2.ApplyTx(tx, tnow, 1); err != nil {
		t.Fatalf("should allow last claim: %v", err)
	}
	if st2.TotalClaimed != ClaimableSupply {
		t.Fatalf("after last claim total %d", st2.TotalClaimed)
	}
	// same identity double claim rejected
	tx2 := &Transaction{
		Type: TxGenesisClaim, Sender: id.NodeID, SenderPub: id.Ed25519Pub,
		Nonce: 1, Fee: 0, Data: EncodeData(struct{}{}),
	}
	tx2.Sign(id.EdPrivate())
	if err := st2.ApplyTx(tx2, tnow, 1); err == nil {
		t.Fatal("expected already claimed")
	}
}
