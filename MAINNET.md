# MSP Mainnet 2 — Production PoS

**Version:** `2.0.0-mainnet-pos`  
**Chain ID:** `msp-mainnet-2`  
**Consensus:** Ethereum-inspired **Proof of Stake** (slots, stake-weighted proposer election, Ed25519 block seals)  
**Block / slot time:** **10 minutes** (fixed)  
**Free claim:** **disabled** (no 210×128 airdrop)  
**Issuance:** only via proposer coinbase from pre-allocated pool (**4,200,000 MST**)

> Old `msp-mainnet-1` (PoW + free claim) is **obsolete**. Use a **fresh** `MSP_DATA` directory.

## What “Ethereum-like” means here

| Ethereum idea | MSP mainnet-2 |
|---------------|---------------|
| Slot time | 10 minutes (`SlotDuration`) |
| Epoch | 32 slots |
| Validators | Accounts with `Active=true` and bonded `Stake` |
| Proposer election | `SHA256(prevHash \|\| slot)` → stake-weighted pick (equal RR if stake=0) |
| Block seal | Proposer **Ed25519** signature over header hash |
| Issuance | Scheduled coinbase `BlockReward(height)` from pool (halving) |
| Free airdrop | **None** |

**Not yet** (honest scope): full Casper FFG finality, attestations/committees, LMD-GHOST fork choice, slashing proofs, validator deposits of 32 ETH semantics. This is a **production-oriented PoS core**, not a line-for-line Ethereum clone.

## Genesis

- Time: `2026-07-13T00:00:00Z`
- Hash: `e170f62c27e603720ae740edfed485781d72af5f5920cdabae8b2aee27ecabf3`
- File: [`genesis/mainnet.json`](genesis/mainnet.json)
- Txs: `network_params` (free_claim=false, pos-eth-inspired) + `init_alert`

```bash
msp -c chain-genesis
```

## Bootstrap (cold start — no free MST)

1. `msp -c init` / `msp -c start` or `msp -c mine` (headless)
2. First epoch (`BootstrapSlots=32`): any node may **propose** and auto-**activate** with 0 stake
3. Proposer earns **50 MST** coinbase (while pool lasts / pre-halving)
4. `msp -c chain-stake <amt>` to bond stake → higher election weight
5. After bootstrap: activation requires `Stake >= MinStake` (50 MST)

```bash
export MSP_DATA=./msp-mainnet-data   # fresh dir
msp -c init
msp -c mine                          # headless proposer loop
# other terminal:
msp -c chain-mine                    # force propose if elected / bootstrap
msp -c chain
msp -c chain-stake 50
```

## Production flags

**Removed** lab shortcuts (no longer affect consensus):

- ~~`MSP_POW_FAST`~~ — ignored for production difficulty targets on DTN adaptive PoW
- ~~`MSP_CHAIN_FAST`~~ — chain slots always 10 minutes
- ~~`MSP_REQUIRE_TICKET=0`~~ — tickets always required when chain is on

## Security model (open source)

| Attack | Result |
|--------|--------|
| Edit local code to print free claim | Your fork only; mainnet-2 nodes reject `genesis_claim` |
| Propose when not elected | Rejected (`errBadProposer`) |
| Inflated coinbase | Rejected |
| Soft “difficulty” / env FAST | Not used for block consensus (PoS signatures) |
| Majority runs modified rules | Social/client consensus — same as any open chain |

## CLI map

| Command | Role |
|---------|------|
| `chain-genesis` | Show frozen genesis |
| `chain` | Height, slot, stake, validators |
| `chain-mine` / `chain-propose` | Propose block |
| `chain-activate` | Join validator set |
| `chain-stake` / `chain-unstake` | Bond / unbond |
| `chain-transfer` / `chain-register` | Economy |

## License

[MIT](LICENSE)
