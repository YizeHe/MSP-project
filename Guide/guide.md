# MSP / MST Network User Guide

**English** ← current | [中文](guide-cn.md)

> **Version**: 1.3.2-daiban7  
> **License**: [MIT](../LICENSE)  
> **Ethos**: This repository is **100% AI-edited** (specs `代办*.md`, `.learnings/`, `.claude/` traces are kept in-repo).  
> **Repo**: [YizeHe/MSP-project](https://github.com/YizeHe/MSP-project)

Full guide for users and contributors: install, build, dual-node messaging, on-chain claim and mining.

---

## 1. What is MSP?

**MSP (Message Security Protocol / MST Network)** is a censorship-resistant, end-to-end encrypted decentralized messaging prototype:

| Layer | Role | Message bodies? |
|-------|------|-----------------|
| **DTN / UDP P2P mesh** | E2EE messages, flood, store-carry-forward | ✅ Memory / local queue only; burn-after-read |
| **Public chain (economy-only)** | MST ledger, burn proofs, coinbase miner incentives | ❌ Hashes and amounts only |
| **Signaling seed (CF Worker / local msp-seed)** | Peer discovery + **hole punch** | ❌ No chat relay (`/v1/packet` → 410) |

Design principles (see `MSP白皮书.md` and `代办4.md`–`代办7.md`):

- Messages are never permanently stored; nothing decryptable on-chain  
- MST only constrains sending (burn); no inflation (pre-allocated miner pool, scheme B)  
- After successful punch, peers can operate without further signaling  

---

## 2. Architecture

```
Sender
  1. Seal plaintext once (AES-GCM)
  2. One-time burner: main → transfer → burner submits burn{MsgType, RefHash}
  3. Local miner issues BurnTicket (pre-confirmation)
  4. DTN/P2P sends Packet + BurnTicket
Receiver
  1. Validate ticket (sig / RefHash / amount / TTL / burn in mempool or chain)
  2. Decrypt → inbox → burn-after-read
```

### 2.1 Tokenomics (代办6)

| Parameter | Value |
|-----------|-------|
| Total supply (no inflation) | **4,226,880** MST |
| Claim pool | **26,880** (**210** nodes × **128** MST) |
| Miner reward pool | **4,200,000** |
| Claim window | 2 years |
| Block reward | From miner pool; ~50 MST/block, ~halving every 210240 blocks |

### 2.2 On-chain message burn costs

| Type | Meaning | ~MST |
|------|---------|------|
| `direct` | Direct after punch | 2 + fee 1 |
| `dtn` | Flood / relay | 10 + 1 |
| `broadcast` | Broadcast | 5 + 1 |
| `alert` | Network alert (Alert key required) | 20 + 1 |

Sends also use a **one-time burner** (fund transfer + fee, then burn).

### 2.3 Local DTN wallet (代办7)

`msp -c mst` / `claim-genesis` touch **local** `mst-ledger.json` (parallel to chain accounts):

- `grant = 128`, `genesis_supply = 4_226_880`  
- **`msp -c start` does not auto-claim**; run `claim-genesis` or on-chain `chain-claim` / mine  

---

## 3. Requirements

- **Go** 1.22+ (match `go.mod`)  
- Windows / Linux / macOS  
- Optional: Cloudflare account (public signaling); or local `msp-seed`  
- Optional: SOCKS5 (Tor onion seeds)  

---

## 4. Clone and build

```bash
git clone https://github.com/YizeHe/MSP-project.git
cd MSP-project
git checkout dev
```

### 4.1 Build into `bin/`

**Windows (PowerShell):**

```powershell
$env:CGO_ENABLED = "0"
go build -o bin/msp.exe ./cmd/msp
go build -o bin/msp-seed.exe ./cmd/msp-seed
go build -o bin/msp-p2p-test.exe ./cmd/msp-p2p-test
```

**Linux / macOS:**

```bash
go build -o bin/msp ./cmd/msp
go build -o bin/msp-seed ./cmd/msp-seed
go build -o bin/msp-p2p-test ./cmd/msp-p2p-test
```

See [`bin/README.md`](../bin/README.md). **Binaries are not committed**; build locally.

### 4.2 Lab environment variables

```powershell
$env:MSP_POW_FAST = "1"       # easier PoW for labs
$env:MSP_CHAIN_FAST = "1"     # shorter block interval (~15s)
$env:MSP_REQUIRE_TICKET = "0" # disable mandatory BurnTicket on receive (lab only)
$env:MSP_DATA = ".\msp-data"  # data directory (default ./msp-data)
```

---

## 5. Five-minute single-node path

```powershell
# 0) Optional local signaling
.\bin\msp-seed.exe -addr 127.0.0.1:8787

# 1) Create identity (backup mnemonic offline)
$env:MSP_DATA = ".\msp-data-demo"
.\bin\msp.exe -c init --print-mnemonic

# 2) Version / identity
.\bin\msp.exe -c version
.\bin\msp.exe -c identity

# 3) Seeds (public CF seeds are built-in by default)
.\bin\msp.exe -c seed list
.\bin\msp.exe -c seed set http://127.0.0.1:8787

# 4) Start mesh + chain (no auto claim)
.\bin\msp.exe -c start
.\bin\msp.exe -c status

# 5) Optional local DTN wallet +128 MST
.\bin\msp.exe -c claim-genesis
.\bin\msp.exe -c mst

# 6) On-chain claim +128 MST (max 210 network-wide) and mine
.\bin\msp.exe -c chain-claim
.\bin\msp.exe -c chain

# 7) Mining (primary path after claim slots are full)
.\bin\msp.exe -c chain-mine force
```

Typical `chain` fields after claim:

- `genesis_grant`: 128  
- `max_claim_nodes`: 210  
- `claimable_supply`: 26880  
- `my_balance`: often **178** (128 + first coinbase 50 when you are the solo miner)  

---

## 6. Dual-node messaging

### 6.1 One-shot smoke test

```powershell
# Terminal A: seed
.\bin\msp-seed.exe -addr 127.0.0.1:8790

# Terminal B:
$env:MSP_SEED = "http://127.0.0.1:8790"
$env:MSP_POW_FAST = "1"
.\bin\msp-p2p-test.exe
```

Success looks like:

```
neighbors 1 1
unicast true broadcast true
PASS full stack
```

> Old local seeds (`0.1.0-test` with `packets` field) are incompatible. Use a freshly built `msp-seed` (`0.2.0-test`, signaling-only).

### 6.2 Manual two-process flow

**Node A:**

```powershell
$env:MSP_DATA = ".\msp-data-a"
$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c seed set http://127.0.0.1:8790
.\bin\msp.exe -c start
.\bin\msp.exe -c identity   # note node_id
.\bin\msp.exe -c chain-claim
```

**Node B:**

```powershell
$env:MSP_DATA = ".\msp-data-b"
$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
.\bin\msp.exe -c init
.\bin\msp.exe -c seed set http://127.0.0.1:8790
.\bin\msp.exe -c start
.\bin\msp.exe -c discover
.\bin\msp.exe -c peers
.\bin\msp.exe -c chain-claim
.\bin\msp.exe -c send <A_node_id> hello-from-B
```

**Node A inbox:**

```powershell
.\bin\msp.exe -c inbox
```

Production defaults enforce BurnTicket; dual nodes need burn/block sync. Lab: `MSP_REQUIRE_TICKET=0`.

---

## 7. CLI reference

Form: `msp -c <command> [args...]`  
Without `-c`, launches the **GUI** (local HTTP + embedded UI).

### 7.1 Identity

| Command | Description |
|---------|-------------|
| `init [--print-mnemonic] [--passphrase=...]` | New BIP39 identity |
| `import <words...>` | Import mnemonic |
| `identity` | Show NodeID / fingerprint |

### 7.2 Signaling seeds

| Command | Description |
|---------|-------------|
| `seed list` | List seed URLs |
| `seed set <url>` | Primary seed |
| `seed add / remove <url>` | Add / remove |
| `health` | Probe seed health |

Default public seeds (config-dependent):

- `https://msp.forbiddenx.top`  
- `https://msp-seed.tangent2533.workers.dev`  

### 7.3 Mesh / messages

| Command | Description |
|---------|-------------|
| `start` / `stop` | Start/stop mesh + chain |
| `status` / `discover` / `peers` | Status and neighbors |
| `send <node_id> <text...>` | Unicast (burner + ticket + DTN) |
| `broadcast <text...>` | Broadcast |
| `alert <text...>` | Alert (Alert key required) |
| `inbox` / `clear-inbox` | Inbox |
| `block` / `unblock <id>` | Blocklist |
| `verify-fingerprint <id> <fp16>` | Out-of-band fingerprint check |
| `trust` | Trust store |

### 7.4 Local MST and public chain

| Command | Description |
|---------|-------------|
| `mst` | Local wallet snapshot (grant=128) |
| `claim-genesis` | Local one-time +128 (optional) |
| `chain` / `chain-status` | Height, balance, claim slots, miner pool |
| `chain-claim` | On-chain claim 128 MST |
| `chain-mine [force]` | Mine a block |
| `chain-transfer <to> <amount>` | Transfer |
| `chain-register` | Register pubkeys on-chain |

### 7.5 Other

| Command | Description |
|---------|-------------|
| `version` / `-v` | Version |
| `block-alert` / `alert-info` | Alert key info |

---

## 8. GUI

```powershell
.\bin\msp.exe
$env:MSP_DATA = ".\msp-data-gui"
.\bin\msp.exe
```

Open the printed local URL (embedded `internal/gui`). Features mirror CLI: identity, seeds, peers, send/receive, chain status.

---

## 9. Self-hosted signaling

### 9.1 Local

```powershell
.\bin\msp-seed.exe -addr 127.0.0.1:8787
curl http://127.0.0.1:8787/v1/health
```

### 9.2 Cloudflare Workers

- `_worker.js` — signaling (Durable Object shared peers)  
- `wrangler.toml` — bindings and routes  

```bash
npx wrangler deploy
```

Free-plan DO migrations need `new_sqlite_classes` (see `.learnings/LEARNINGS.md`).  
**Tor / SOCKS:** `$env:SOCKS5_PROXY = "socks5://127.0.0.1:9050"`

---

## 10. Tests

```powershell
$env:MSP_POW_FAST = "1"
$env:MSP_CHAIN_FAST = "1"
go test ./... -count=1 -timeout 180s
```

Coverage highlights: chain claim/burn/ticket/burner/coinbase/210-cap; local mst constants; pow; dedup. Integration: `bin/msp-p2p-test` (§6.1).

---

## 11. Data directory and safety

Default `./msp-data` (`MSP_DATA` overrides): `identity.json`, `config.json`, `trust.json`, `mst-ledger.json`, `chain/`, optional `dtn-queue.json`.

**Never** commit `msp-data*` or post keys publicly. Backup mnemonics offline; loss is unrecoverable.

---

## 12. Troubleshooting

| Symptom | Action |
|---------|--------|
| `POST /v1/hello` 400 / old seed | Rebuild `msp-seed`; health should be `0.2.0-test` + `signaling-only` |
| `neighbors 0` | UDP firewall, seed reachability, `discover` |
| `supply exhausted` | 210 claims full → `chain-mine` |
| `insufficient` | `chain-claim` or mine; cover burn+fees+burner fund |
| Ticket validation fails | Sync chain mempool/blocks; lab: `MSP_REQUIRE_TICKET=0` |
| Slow mining | `MSP_POW_FAST=1` `MSP_CHAIN_FAST=1` |
| Seed TLS timeout | Local mesh/chain can continue; try local seed or proxy |
| CLI state not shared | Each `msp -c` is a new process; use GUI for a long session |

---

## 13. Document map

| Path | Notes |
|------|-------|
| `Guide/guide.md` | **This file** (English, default) |
| `Guide/guide-cn.md` | Chinese guide |
| `README.md` | English overview |
| `README-cn.md` … `README-de.md` | Localized READMEs |
| `LICENSE` | MIT |
| `代办.md` … `代办7.md` | Iteration specs |
| `.learnings/` | Learnings and errors |

---

## 14. License

[MIT](../LICENSE) © 2026 YizeHe / MSP-project contributors.  
Research/prototype network only — **no mainnet asset guarantees**. Do not treat MST as real money.

---

*Generated and maintained as part of the 100% AI-edited MSP project workflow.*
