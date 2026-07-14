package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/YizeHe/MSP-project/internal/chain"
)

func main() {
	g := chain.BuildGenesis()
	raw, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll("genesis", 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile("genesis/mainnet.json", raw, 0o644); err != nil {
		panic(err)
	}
	fmt.Println("height", g.Header.Height)
	fmt.Println("hash", g.Header.HashHex())
	fmt.Println("merkle", g.Header.MerkleRoot)
	fmt.Println("state_root", g.Header.StateRoot)
	fmt.Println("timestamp", g.Header.Timestamp)
	fmt.Println("tx_count", len(g.Txs))
}
