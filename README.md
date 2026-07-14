# MSP / MST Network

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](go.mod)
[![100% AI-edited](https://img.shields.io/badge/100%25-AI--edited-purple)](Guide/guide.md)

**English** | [中文](README-cn.md) | [日本語](README-ja.md) | [Français](README-fr.md) | [Русский](README-ru.md) | [Español](README-es.md) | [العربية](README-ar.md) | [Deutsch](README-de.md)

**100% AI-edited** censorship-resistant, end-to-end encrypted messaging with a **production PoS economic chain**.

| Layer | Role |
|-------|------|
| **DTN / UDP P2P** | Encrypted messages, flood, store-carry-forward |
| **Public chain** | MST ledger + burn proofs; **no message bodies** |
| **Consensus** | **Ethereum-inspired PoS** — 10 min slots, stake-weighted proposers |
| **Signaling** | Hole punch only (no chat relay) |

## Version / Mainnet

`2.0.0-mainnet-pos` · branch **`dev`** · chain id **`msp-mainnet-2`**

| | |
|--|--|
| Free claim (210×128) | **Removed** — no airdrop |
| Issuance | Proposer coinbase only (pool **4,200,000** MST) |
| Slot / block time | **10 minutes** |
| Docs | [MAINNET.md](MAINNET.md) · [genesis/mainnet.json](genesis/mainnet.json) · [index.html](index.html) |

## Documentation

| Doc | Language |
|-----|----------|
| **[index.html](index.html)** | Landing page |
| **[Guide/guide.md](Guide/guide.md)** | English guide |
| **[Guide/guide-cn.md](Guide/guide-cn.md)** | 中文 |
| [MAINNET.md](MAINNET.md) | Production mainnet-2 |

## Quick start

```bash
git clone https://github.com/YizeHe/MSP-project.git
cd MSP-project && git checkout dev
go build -o bin/msp ./cmd/msp

export MSP_DATA=./msp-mainnet-data   # fresh directory required for mainnet-2
./bin/msp -c init
./bin/msp -c mine                    # headless PoS proposer
# another terminal:
./bin/msp -c chain-mine              # propose when eligible
./bin/msp -c chain
./bin/msp -c chain-stake 50
```

Windows: build `bin/msp.exe` the same way.

## Tests

```bash
go test ./... -count=1 -timeout 180s
```

## License

[MIT](LICENSE) © 2026 YizeHe / MSP-project contributors.

## Ethos

**100% AI-edited** — specs (`代办*`), `.learnings/`, and agent traces stay in-repo.
