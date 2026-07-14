# Canonical genesis artifacts

| File | Network |
|------|---------|
| [`mainnet.json`](mainnet.json) | **msp-mainnet-1** (production) |

Regenerate from source of truth (`internal/chain.BuildGenesis`):

```bash
go run ./cmd/msp-genesis-dump
```

If the hash changes, update constants in `internal/chain/genesis.go` (`MainnetGenesisHash`, …) and `MAINNET.md`.

See [MAINNET.md](../MAINNET.md).
