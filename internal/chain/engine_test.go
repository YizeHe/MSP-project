package chain

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YizeHe/MSP-project/internal/identity"
)

func openTestEngine(t *testing.T) (*Engine, *identity.Identity) {
	t.Helper()
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

func TestMainnet2GenesisNoFreeClaim(t *testing.T) {
	g := MainnetGenesis()
	if err := ValidateMainnetGenesis(g); err != nil {
		t.Fatal(err)
	}
	if ChainID != "msp-mainnet-2" {
		t.Fatal(ChainID)
	}
	info := GenesisInfo()
	if info["free_claim"] != false {
		t.Fatal("free claim must be false")
	}
	if GenesisGrant != 0 || ClaimableSupply != 0 {
		t.Fatal("claim pool must be zero")
	}
	e, _ := openTestEngine(t)
	if e.Status()["free_claim"] != false {
		t.Fatal(e.Status())
	}
	if _, err := e.ClaimGenesis(); err == nil {
		t.Fatal("claim must be disabled")
	}
}

func TestPoSProposeAndReward(t *testing.T) {
	e, id := openTestEngine(t)
	// cold start: no validators → MayPropose allows
	b, err := e.ProposeOnce(true)
	if err != nil {
		t.Fatal(err)
	}
	if b.Header.Height != 1 {
		t.Fatalf("height %d", b.Header.Height)
	}
	if b.Header.Proposer != id.NodeID {
		t.Fatal("proposer")
	}
	if err := VerifyProposerSig(&b.Header); err != nil {
		t.Fatal(err)
	}
	acc := e.Account(id.NodeID)
	// coinbase 50 + auto-activate
	if acc.Balance != BlockReward(1) {
		t.Fatalf("balance %d want %d active=%v", acc.Balance, BlockReward(1), acc.Active)
	}
	if !acc.Active {
		t.Fatal("should auto-activate")
	}
	// second block
	b2, err := e.ProposeOnce(true)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Header.Height != 2 {
		t.Fatalf("h2 %d", b2.Header.Height)
	}
	acc = e.Account(id.NodeID)
	want := BlockReward(1) + BlockReward(2)
	if acc.Balance != want {
		t.Fatalf("balance %d want %d", acc.Balance, want)
	}
}

func TestStakeWeightAndRejectWrongProposer(t *testing.T) {
	e, id := openTestEngine(t)
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	// stake some of reward (leave fee room: balance 50 after h1)
	st, err := e.Stake(40)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitTx(st); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	acc := e.Account(id.NodeID)
	if acc.Stake < 40 {
		t.Fatalf("stake %d", acc.Stake)
	}
	// craft block with wrong proposer sig from another identity
	id2, _ := identity.Generate()
	tip := e.Store.Tip()
	slot := tip.Header.Slot + 1
	bad := &Block{
		Header: BlockHeader{
			Version: ProtocolVersion, PrevBlock: tip.Header.HashHex(),
			Timestamp: time.Now().UnixMicro(), Height: tip.Header.Height + 1,
			Slot: slot, Proposer: id2.NodeID, ProposerPub: id2.Ed25519Pub,
			MinerID: id2.NodeID,
		},
		Txs: []Transaction{{
			Type: TxCoinbase, Sender: id2.NodeID, SenderPub: id2.Ed25519Pub,
			Data: EncodeData(CoinbaseData{Amount: BlockReward(tip.Header.Height + 1), Height: tip.Header.Height + 1, Slot: slot}),
		}},
	}
	bad.Header.MerkleRoot = MerkleRootFromTxs(bad.Txs)
	cl := e.State.Clone()
	_ = cl.ApplyBlock(bad)
	bad.Header.StateRoot = cl.Root()
	SignBlockHeader(&bad.Header, id2.EdPrivate())
	if err := e.AcceptBlock(bad); err == nil {
		t.Fatal("expected reject wrong proposer election")
	}
}

func TestBurnStillWorks(t *testing.T) {
	e, _ := openTestEngine(t)
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	// need more balance for burn 10+1
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	ref := RefHashFromBytes([]byte("cipher"))
	burn, err := e.BurnForMessage(MsgDTN, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitTx(burn); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	if !e.HasTxHash(burn.TxID()) {
		t.Fatal("burn not applied")
	}
	ticket, err := e.IssueTicketForBurn(burn)
	if err != nil {
		t.Fatal(err)
	}
	if err := ticket.Verify(); err != nil {
		t.Fatal(err)
	}
	_ = base64.StdEncoding.EncodeToString([]byte("x"))
	_ = os.TempDir()
}

func TestSlotHelpers(t *testing.T) {
	if SlotDuration != 10*time.Minute {
		t.Fatal(SlotDuration)
	}
	s0 := SlotAtTime(NetworkGenesis)
	if s0 != 0 {
		t.Fatal(s0)
	}
	s1 := SlotAtTime(NetworkGenesis.Add(10 * time.Minute))
	if s1 != 1 {
		t.Fatal(s1)
	}
}

func TestRejectSenderPubImpersonation(t *testing.T) {
	e, id := openTestEngine(t)
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	// attacker key signs a transfer claiming victim's Sender
	atk, _ := identity.Generate()
	tx := &Transaction{
		Type: TxTransfer, Sender: id.NodeID, SenderPub: atk.Ed25519Pub,
		Nonce: e.Account(id.NodeID).Nonce, Fee: FeeTransfer,
		Data: EncodeData(TransferData{To: atk.NodeID, Amount: 1}),
	}
	tx.Sign(atk.EdPrivate())
	if err := tx.Verify(); err == nil {
		t.Fatal("expected reject: sender not bound to sender_pub")
	}
	if err := e.SubmitTx(tx); err == nil {
		t.Fatal("expected submit reject")
	}
}

func TestRejectAmountOverflow(t *testing.T) {
	e, id := openTestEngine(t)
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	tx := &Transaction{
		Type: TxTransfer, Sender: id.NodeID, SenderPub: id.Ed25519Pub,
		Nonce: e.Account(id.NodeID).Nonce, Fee: 1,
		Data: EncodeData(TransferData{To: id.NodeID, Amount: ^uint64(0)}),
	}
	tx.Sign(id.EdPrivate())
	if err := e.SubmitTx(tx); err == nil {
		t.Fatal("expected overflow/too-large reject")
	}
}

func TestIssueTicketRequiresSubmittedBurn(t *testing.T) {
	e, id := openTestEngine(t)
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ProposeOnce(true); err != nil {
		t.Fatal(err)
	}
	// fabricates burn not in mempool
	fake := &Transaction{
		Type: TxBurn, Sender: id.NodeID, SenderPub: id.Ed25519Pub,
		Nonce: 99, Fee: FeeBurnBase,
		Data: EncodeData(BurnData{MsgType: MsgDTN, RefHash: "ab", Amount: BurnDTN, Timestamp: 1}),
	}
	fake.Sign(id.EdPrivate())
	if _, err := e.IssueTicketForBurn(fake); err == nil {
		t.Fatal("expected reject unsigned/unsubmitted burn ticket")
	}
}
