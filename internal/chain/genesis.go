package chain

import (
	"encoding/json"
	"fmt"

	"github.com/YizeHe/MSP-project/internal/alert"
)

// Frozen mainnet genesis fingerprints (must match BuildGenesis output).
// See genesis/mainnet.json and MAINNET.md.
const (
	MainnetGenesisHash      = "7305b0979062171e2ee24d7e909efb588014f8c4f28fc0ed576fa9a2c0f4e351"
	MainnetGenesisMerkle    = "095b07d57039c8a9b1e99a19449741230903c2fa4bcf58c48bdf19888cad2a62"
	MainnetGenesisStateRoot = "76819a0d34b61ce28f8dea0e0c095d3265ecfb962060b58bc0f718a88a1cc5ff"
)

// BuildGenesis height 0 — network params + alert init (msp-mainnet-1).
// Canonical fingerprints are derived from this function; see genesis/mainnet.json.
func BuildGenesis() *Block {
	paramsTx := Transaction{
		Type:   TxNetworkParams,
		Sender: "network-genesis",
		Data: EncodeData(NetworkParamsData{
			ChainID:         ChainID,
			Name:            NetworkName,
			GenesisGrant:    GenesisGrant,
			MaxClaimNodes:   MaxClaimNodes,
			MinerRewardPool: MinerRewardPool,
			ClaimableSupply: ClaimableSupply,
			GenesisSupply:   GenesisSupply,
			HalvingInterval: HalvingInterval,
		}),
	}
	initTx := Transaction{
		Type:   TxInitAlert,
		Sender: "network-genesis",
		Data:   EncodeData(InitAlertData{PubKey: alert.PublicKeyB64()}),
	}
	b := &Block{
		Header: BlockHeader{
			Version:    ProtocolVersion,
			PrevBlock:  hex32zero(),
			Timestamp:  NetworkGenesis.UnixMicro(),
			Difficulty: 1,
			Height:     0,
			MinerID:    "network-genesis",
			TxCount:    2,
			PoWHash:    "genesis",
		},
		Txs: []Transaction{paramsTx, initTx},
	}
	b.Header.MerkleRoot = MerkleRootFromTxs(b.Txs)
	st := NewState()
	_ = st.ApplyBlock(b)
	b.Header.StateRoot = st.Root()
	return b
}

// MainnetGenesis returns the canonical height-0 block for msp-mainnet-1.
func MainnetGenesis() *Block {
	return BuildGenesis()
}

// GenesisInfo public summary for CLI / status.
func GenesisInfo() map[string]any {
	g := MainnetGenesis()
	return map[string]any{
		"network":      NetworkName,
		"chain_id":     ChainID,
		"height":       g.Header.Height,
		"hash":         g.Header.HashHex(),
		"merkle_root":  g.Header.MerkleRoot,
		"state_root":   g.Header.StateRoot,
		"timestamp_us": g.Header.Timestamp,
		"network_time": NetworkGenesis.UTC().Format("2006-01-02T15:04:05Z"),
		"tx_count":     len(g.Txs),
		"protocol":     ProtocolVersion,
		"genesis_grant":    GenesisGrant,
		"max_claim_nodes":  MaxClaimNodes,
		"claimable_supply": ClaimableSupply,
		"miner_reward_pool": MinerRewardPool,
		"genesis_supply":   GenesisSupply,
		"note":             "MSP mainnet genesis — economic ledger only; messages are DTN-only",
	}
}

// ValidateMainnetGenesis checks a height-0 block matches the frozen mainnet genesis.
func ValidateMainnetGenesis(b *Block) error {
	if b == nil {
		return fmt.Errorf("nil genesis")
	}
	if b.Header.Height != 0 {
		return fmt.Errorf("genesis height must be 0, got %d", b.Header.Height)
	}
	// primary: frozen hash constants (stable across releases)
	if b.Header.HashHex() != MainnetGenesisHash {
		return fmt.Errorf("genesis hash mismatch: got %s want %s (wrong network or fork)",
			b.Header.HashHex(), MainnetGenesisHash)
	}
	if b.Header.StateRoot != MainnetGenesisStateRoot {
		return fmt.Errorf("genesis state_root mismatch")
	}
	if b.Header.MerkleRoot != MainnetGenesisMerkle {
		return fmt.Errorf("genesis merkle_root mismatch")
	}
	// secondary: must equal live BuildGenesis()
	want := MainnetGenesis()
	if b.Header.HashHex() != want.Header.HashHex() {
		return fmt.Errorf("genesis drift: BuildGenesis hash %s != frozen %s — rebuild constants",
			want.Header.HashHex(), MainnetGenesisHash)
	}
	return nil
}

// ExportGenesisJSON canonical pretty JSON for genesis/mainnet.json.
func ExportGenesisJSON() ([]byte, error) {
	return json.MarshalIndent(MainnetGenesis(), "", "  ")
}

func hex32zero() string {
	return "0000000000000000000000000000000000000000000000000000000000000000"
}
