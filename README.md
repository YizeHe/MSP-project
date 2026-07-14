# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

**English** | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

A **100% AI-edited** censorship-resistant, end-to-end encrypted messaging prototype.

| Layer | Role |
|-------|------|
| **DTN / UDP P2P mesh** | Encrypted messages, flood, store-carry-forward |
| **Public chain** | MST ledger + burn proofs only (no message bodies) |
| **Signaling seed** | Peer discovery & **hole punch** only (no chat relay) |

## Documentation

| Doc | Language |
|-----|----------|
| **[index.html](index.html)** | Project landing page (English) |
| **[Guide/guide.md](Guide/guide.md)** | English (default) |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | 中文 |
| [MSP白皮书.md](MSP白皮书.md) | Protocol whitepaper (Chinese) |
| `代办.md` … `代办7.md` | Iteration specs (AI collaboration traces, kept in-repo) |

## Version / Mainnet

`1.4.0-mainnet` — prefer branch **`dev`**.

| | |
|--|--|
| **Network** | `mainnet` |
| **Chain ID** | `msp-mainnet-1` |
| **Genesis** | [`genesis/mainnet.json`](genesis/mainnet.json) · [`MAINNET.md`](MAINNET.md) |
| **CLI** | `msp -c chain-genesis` |

## One-line architecture

```
Seal ciphertext → anonymous burner burn → BurnTicket → DTN send
Chain: MST ledger + burn hash · Signaling: hole punch only
```

## Tokenomics (代办6)

| Parameter | Value |
|-----------|-------|
| Total supply (no inflation) | **4,226,880** MST |
| Free claim pool | **26,880** (210 nodes × **128** MST) |
| Miner reward pool | **4,200,000** MST |
| Claim window | 2 years |

Message burn costs (approx.): direct 2+1 · dtn 10+1 · broadcast 5+1 · alert 20+1 MST.

## Quick start

```powershell
# Build into bin/ (binaries are gitignored)
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed

$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c start
.\bin\msp.exe -c claim-genesis   # optional local DTN wallet +128
.\bin\msp.exe -c chain-claim     # on-chain +128 (max 210 slots)
```

Linux / macOS:

```bash
go build -o bin/msp ./cmd/msp
go build -o bin/msp-seed ./cmd/msp-seed
./bin/msp -c init && ./bin/msp -c start
```

Full tutorial (CLI, dual-node, troubleshooting): **[Guide/guide.md](Guide/guide.md)**.

## Tests

```powershell
$env:MSP_POW_FAST = "1"; $env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

## License

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors.

## Project ethos

This repository proudly ships as **100% AI-edited**: specs (`代办*`), `.learnings/`, and agent traces are **not** scrubbed from history.
