# MSP Mainnet

**Status:** live (code-defined)  
**Network:** `mainnet`  
**Chain ID:** `msp-mainnet-1`  
**Version:** `1.4.0-mainnet`

## Genesis

- **Time:** `2026-07-13T00:00:00Z` (`NetworkGenesis`)
- **Height:** 0
- **Hash:** `7305b0979062171e2ee24d7e909efb588014f8c4f28fc0ed576fa9a2c0f4e351`
- **Merkle:** `095b07d57039c8a9b1e99a19449741230903c2fa4bcf58c48bdf19888cad2a62`
- **State root:** `76819a0d34b61ce28f8dea0e0c095d3265ecfb962060b58bc0f718a88a1cc5ff`
- **Canonical file:** [`genesis/mainnet.json`](genesis/mainnet.json)
- **Contents:**
  1. `network_params` — frozen economics (128×210 claim, 4.2M miner pool, total 4,226,880 MST)
  2. `init_alert` — network Alert authority public key

Inspect without starting a node:

```bash
msp -c chain-genesis
# aliases: -c mainnet | -c genesis-block
```

## Launch a mainnet node

```bash
# fresh data directory (old lab chains with pre-mainnet genesis will be rejected)
export MSP_DATA=./msp-mainnet-data   # PowerShell: $env:MSP_DATA=".\msp-mainnet-data"

msp -c init
msp -c start
msp -c chain              # network=mainnet, chain_id=msp-mainnet-1, genesis_hash=...
msp -c chain-claim        # optional free claim (128 MST, 210 slots network-wide)
msp -c chain-mine force   # mine for rewards from the pre-allocated pool
```

> **Lab overrides** (`MSP_POW_FAST=1`, `MSP_CHAIN_FAST=1`) still work for local testing but do **not** change genesis or chain_id. Production operators should leave them unset (10-minute target block interval).

## Validation rules

- Every node installs the same height-0 block from `BuildGenesis()` / `MainnetGenesis()`.
- Loading a store whose height-0 hash ≠ mainnet genesis **fails** (prevents mixing lab forks).
- Incoming blocks at height 0 must pass `ValidateMainnetGenesis`.

## Economics (unchanged from 代办6)

| Parameter | Value |
|-----------|-------|
| Total supply | 4,226,880 MST (no inflation) |
| Claim pool | 26,880 (210 × 128) |
| Miner pool | 4,200,000 |
| Claim window | 2 years from network genesis |

## Messages

Still **DTN-only**. The chain never carries message bodies — only MST ledger and burn proofs (+ BurnTicket pre-confirmation off-chain / in mempool).

## License

[MIT](LICENSE)
