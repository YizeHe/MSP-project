# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

**English** | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

**100% AI-edited** censorship-resistant messaging network: **E2EE DTN/P2P for messages**, **public chain for MST economics only**, **PoS consensus** (not full Ethereum, production-oriented core).

Suitable for **closed beta / internal testing**. Not a claim of bank-grade finality or full ETH Gasper parity.

## Architecture

```
Send message
  → seal ciphertext once (AES-GCM)
  → optional one-time burner burn {MsgType, RefHash}
  → miner/proposer BurnTicket
  → flood over UDP mesh (burn-after-read)

Public chain
  → MST balances, stake, burns (hash only), coinbase
  → never stores message bodies
```

| Layer | Role | Port / transport |
|-------|------|------------------|
| **DTN / UDP P2P** | Chat, flood, store-carry-forward | **UDP** — default OS ephemeral (`udp_port: 0`), configurable |
| **Public chain** | Ledger + burn proofs + PoS blocks | In-process + mesh `chain_block` / `chain_tx` frames |
| **Signaling seed** | Peer discovery & **hole punch only** | **HTTPS/HTTP** (e.g. `:443` or self-hosted `:8787`) |
| **Local GUI** | Browser UI for this node | **TCP** `127.0.0.1:0` (OS picks free port) |

## Mainnet-2 (current)

| | |
|--|--|
| **Version** | `2.0.0-mainnet-pos` |
| **Branch** | `dev` |
| **Chain ID** | **`msp-mainnet-2`** |
| **Consensus** | PoS — 10‑minute slots, stake-weighted proposer, Ed25519 block seal |
| **Free claim** | **Removed** (no 210×128 airdrop) |
| **Issuance** | Proposer coinbase only from pool **4,200,000** MST (halving schedule) |
| **Min stake** (after bootstrap) | 50 MST |
| **Bootstrap** | First 32 slots: activate/propose with 0 stake, then earn & restake |
| **Genesis** | [genesis/mainnet.json](genesis/mainnet.json) · [MAINNET.md](MAINNET.md) |

> Use a **fresh** `MSP_DATA` directory. Old `msp-mainnet-1` / free-claim data is incompatible.

### PoS in one line

`proposer = stake_weighted(SHA256(prevHash || slot))` · block sealed by proposer Ed25519 · rewards = `BlockReward(height)` from pre-allocated pool.

**Not included (by design for now):** full Casper FFG, attestation committees, LMD-GHOST, slashing proofs.

## Build

```bash
git clone https://github.com/YizeHe/MSP-project.git
cd MSP-project
git checkout dev

go build -o bin/msp ./cmd/msp
go build -o bin/msp-seed ./cmd/msp-seed
go build -o bin/msp-p2p-test ./cmd/msp-p2p-test
```

Windows: use `bin/msp.exe`, etc. Binaries under `bin/` are **gitignored** — see [bin/README.md](bin/README.md).

## Quick start (node)

```bash
export MSP_DATA=./msp-mainnet-data-v2   # fresh dir for mainnet-2

./bin/msp -c init
./bin/msp -c seed list
# optional custom seed:
# ./bin/msp -c seed set http://YOUR_PUBLIC_IP:8787

./bin/msp -c mine              # headless mesh + PoS proposer loop
# other terminal:
./bin/msp -c chain-mine        # propose a block when allowed
./bin/msp -c chain             # height, slot, stake, pool
./bin/msp -c chain-stake 50
./bin/msp -c chain-genesis     # frozen genesis info (no start needed)
```

GUI (local only):

```bash
./bin/msp                      # opens http://127.0.0.1:<random>/
```

## Manual seed (you can type any URL)

Seeds are **signaling only** (not message relays).

```bash
./bin/msp -c seed list
./bin/msp -c seed set https://msp.forbiddenx.top
./bin/msp -c seed add http://127.0.0.1:8787
./bin/msp -c seed remove http://127.0.0.1:8787
```

GUI tab **Seeds / 种子**: paste URL → Primary / Add.

### Run your own seed (public IP)

```bash
./bin/msp-seed -addr 0.0.0.0:8787
# open TCP 8787 on firewall; others:
# msp -c seed set http://YOUR_PUBLIC_IP:8787
```

Prefer HTTPS reverse proxy in real deployments.

## Messaging notes

- Data plane: **UDP P2P** after hole punch; CF/seed is not a chat hub (`/v1/packet` → 410).
- Bound UDP port: see `msp -c status` → `udp_port`.
- Optional fixed UDP: set `"udp_port": 9000` in `config.json`.

## CLI map (high level)

| Area | Commands |
|------|----------|
| Identity | `init`, `import`, `identity` |
| Seeds | `seed list\|set\|add\|remove`, `health` |
| Mesh | `start`, `mine` (headless), `status`, `peers`, `send`, `broadcast`, `inbox` |
| Chain / PoS | `chain-genesis`, `chain`, `chain-mine`, `chain-activate`, `chain-stake`, `chain-unstake`, `chain-transfer`, `chain-register` |
| **No** | `chain-claim` free airdrop (disabled) |

## Tests

```bash
go test ./... -count=1 -timeout 180s
```

## Documentation

| Doc | Content |
|-----|---------|
| [MAINNET.md](MAINNET.md) | Mainnet-2 rules, bootstrap, security model |
| [Guide/guide.md](Guide/guide.md) | English user guide |
| [Guide/guide-cn.md](Guide/guide-cn.md) | 中文指南 |
| [index.html](index.html) | Landing page |
| [MSP白皮书.md](MSP白皮书.md) | Protocol whitepaper (Chinese) |
| `代办.md` … | Iteration specs (kept as AI-edit history) |

## License

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors.

## Ethos

This repo is **100% AI-edited**: specs, `.learnings/`, and agent traces are kept on purpose.
