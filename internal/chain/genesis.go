package chain

import "github.com/YizeHe/MSP-project/internal/alert"

// BuildGenesis height 0 — economy + alert init only.
func BuildGenesis() *Block {
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
			TxCount:    1,
			PoWHash:    "genesis",
		},
		Txs: []Transaction{initTx},
	}
	b.Header.MerkleRoot = MerkleRootFromTxs(b.Txs)
	st := NewState()
	_ = st.ApplyBlock(b)
	b.Header.StateRoot = st.Root()
	return b
}

func hex32zero() string {
	return "0000000000000000000000000000000000000000000000000000000000000000"
}
