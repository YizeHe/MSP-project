package chain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/YizeHe/MSP-project/internal/alert"
)

// Frozen mainnet-2 genesis fingerprints (PoS, no free claim).
// Updated by: go run ./cmd/msp-genesis-dump
const (
	MainnetGenesisHash      = "e170f62c27e603720ae740edfed485781d72af5f5920cdabae8b2aee27ecabf3"
	MainnetGenesisMerkle    = "b25d24043ed2bed5c9d8c23ef9ec90429c9e5adc63391c7e6d2f87a920a825d7"
	MainnetGenesisStateRoot = "7ee18b195ffa59fe0c60f24e2885464ada042eae1f16cc6b73153f0ed42a5c33"
)

// BuildGenesis height 0 — network params + alert (no free-claim allocation).
func BuildGenesis() *Block {
	paramsTx := Transaction{
		Type:   TxNetworkParams,
		Sender: "network-genesis",
		Data: EncodeData(NetworkParamsData{
			ChainID:         ChainID,
			Name:            NetworkName,
			Consensus:       "pos-eth-inspired",
			SlotSeconds:     int64(SlotDuration / time.Second),
			EpochLength:     EpochLength,
			MinStake:        MinStake,
			BootstrapSlots:  BootstrapSlots,
			MinerRewardPool: MinerRewardPool,
			GenesisSupply:   GenesisSupply,
			HalvingInterval: HalvingInterval,
			FreeClaim:       false,
		}),
	}
	initTx := Transaction{
		Type:   TxInitAlert,
		Sender: "network-genesis",
		Data:   EncodeData(InitAlertData{PubKey: alert.PublicKeyB64()}),
	}
	b := &Block{
		Header: BlockHeader{
			Version:   ProtocolVersion,
			PrevBlock: hex32zero(),
			Timestamp: NetworkGenesis.UnixMicro(),
			Height:    0,
			Slot:      0,
			Proposer:  "network-genesis",
			MinerID:   "network-genesis",
			TxCount:   2,
			PoWHash:   "genesis",
		},
		Txs: []Transaction{paramsTx, initTx},
	}
	b.Header.MerkleRoot = MerkleRootFromTxs(b.Txs)
	st := NewState()
	_ = st.ApplyBlock(b)
	b.Header.StateRoot = st.Root()
	return b
}

// MainnetGenesis canonical block.
func MainnetGenesis() *Block { return BuildGenesis() }

// GenesisInfo CLI summary.
func GenesisInfo() map[string]any {
	g := MainnetGenesis()
	return map[string]any{
		"network":           NetworkName,
		"chain_id":          ChainID,
		"consensus":         "pos-eth-inspired",
		"slot_duration":     SlotDuration.String(),
		"epoch_length":      EpochLength,
		"min_stake":         MinStake,
		"bootstrap_slots":   BootstrapSlots,
		"free_claim":        false,
		"height":            g.Header.Height,
		"hash":              g.Header.HashHex(),
		"merkle_root":       g.Header.MerkleRoot,
		"state_root":        g.Header.StateRoot,
		"timestamp_us":      g.Header.Timestamp,
		"network_time":      NetworkGenesis.UTC().Format("2006-01-02T15:04:05Z"),
		"tx_count":          len(g.Txs),
		"protocol":          ProtocolVersion,
		"genesis_supply":    GenesisSupply,
		"reward_pool":       MinerRewardPool,
		"note":              "MSP mainnet-2: PoS slots 10m; issuance only via proposer rewards; no free claim",
	}
}

// ValidateMainnetGenesis.
func ValidateMainnetGenesis(b *Block) error {
	if b == nil {
		return fmt.Errorf("nil genesis")
	}
	if b.Header.Height != 0 {
		return fmt.Errorf("genesis height must be 0, got %d", b.Header.Height)
	}
	want := MainnetGenesis()
	// Prefer live BuildGenesis as source of truth if placeholders not yet stamped
	if MainnetGenesisHash == "" || MainnetGenesisHash == "GENESIS_HASH_PLACEHOLDER" {
		if b.Header.HashHex() != want.Header.HashHex() {
			return fmt.Errorf("genesis hash mismatch: got %s want %s", b.Header.HashHex(), want.Header.HashHex())
		}
		return nil
	}
	if b.Header.HashHex() != MainnetGenesisHash {
		return fmt.Errorf("genesis hash mismatch: got %s want %s (wrong network)",
			b.Header.HashHex(), MainnetGenesisHash)
	}
	if b.Header.StateRoot != MainnetGenesisStateRoot {
		return fmt.Errorf("genesis state_root mismatch")
	}
	if b.Header.MerkleRoot != MainnetGenesisMerkle {
		return fmt.Errorf("genesis merkle_root mismatch")
	}
	if b.Header.HashHex() != want.Header.HashHex() {
		return fmt.Errorf("genesis drift: rebuild constants")
	}
	return nil
}

// ExportGenesisJSON.
func ExportGenesisJSON() ([]byte, error) {
	return json.MarshalIndent(MainnetGenesis(), "", "  ")
}

func hex32zero() string {
	return "0000000000000000000000000000000000000000000000000000000000000000"
}
