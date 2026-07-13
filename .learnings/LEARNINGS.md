# MSP Project Learnings

## v0.2 architecture (2026-07-13) — CF = hole punch only

- **Data plane**: UDP P2P mesh (`internal/p2p`) — punch, keepalive, flood packets between neighbors.
- **Control plane**: CF seed HTTPS — register candidates, list peers, optional tiny signals. **No chat relay**.
- `/v1/packet` returns **410 data_plane_removed**.
- After punch, nodes keep working without further seed traffic (verified in `msp-p2p-test`).
- `HTTP_PROXY` only affects seed HTTP; UDP is direct.

## v0.1 lesson

- Early v0.1 incorrectly used CF as message flood hub; user corrected: CF is for hole punching only.

## PoW simplification

Whitepaper RandomX (~2min) is deferred. v0.1 uses SHA-512 leading-zero Hashcash (default 16 bits) so CLI self-tests finish in milliseconds.

## E2E verified

Local seed `127.0.0.1:8787`: two identities unicast E2EE + broadcast decrypt OK; dual-hash dedup ring + Ed25519 verify path exercised.

## Cloudflare deploy notes

- Live seed custom domain: `https://msp.forbiddenx.top` (zone active on same CF account).
- Fallback: `https://msp-seed.tangent2533.workers.dev` (wrangler deploy).
- Bind custom domain via `routes = [{ pattern = "msp.forbiddenx.top", custom_domain = true }]` in wrangler.toml.
- **Durable Object `HUB` / `MspHub` required** so peers/packets are shared across isolates.
- Free plan: migration must use `new_sqlite_classes` (not `new_classes`) — API error 10097 otherwise.
- `state.storage.get(["a","b"])` returns a **Map** — use `get("key")` per key or `.get()` on the Map.
- Verified 2026-07-13: health, join, peers, unicast E2EE, broadcast all OK on CF.
- **China mainland:** CF Free has no China Network. Custom domain is better than `workers.dev`, but anycast CF IPs can still be unstable in CN. If blocked, run domestic `msp-seed` or multi-seed failover later.

## ECDH session info

Unicast AES key: `HKDF-SHA512(X25519(a,b), info="unicast:<sender_id>:<target_id>")`. Both sides must use the same info string (sender:target order).

## 代办5 — BurnTicket / burner / miner pool (2026-07-14)

- **RefHash must bind final ciphertext**: trial-seal then re-seal with a new AES-GCM nonce breaks `ValidateBurnTicket`. Use `Send*WithTicketFn`: seal once → `ticketFn(cipherB64)` → attach ticket → PoW/sign/send.
- **AnonymousBurn**: main → one-time burner transfer, then `MineOnce` (so burner has balance), then burn from burner + `IssueTicketForBurn`. Burn stays in mempool until next mine; ticket validates via `HasTxHash`.
- **Scheme B miner pool**: `ClaimableSupply=16.8M`, `MinerRewardPool=4.2M`; coinbase drains pool with `BlockReward` halvings; total supply still 21M (no inflation beyond genesis allocation).
- **Solo miner tests**: fee returns to same node; balances = grants − burns + coinbases. Account for coinbase in every `MineOnce` after height 0.
- Lab: `MSP_REQUIRE_TICKET=0` disables receive-side ticket enforcement; keep `MSP_POW_FAST=1` / `MSP_CHAIN_FAST=1` for tests.

## 代办6 — reduced free claim (2026-07-14)

- `GenesisGrant=128`, `MaxClaimNodes=210`, `ClaimableSupply=26_880`, `MinerRewardPool=4_200_000`, `GenesisSupply=4_226_880`.
- `applyGenesisClaim` rejects when `TotalClaimed/GenesisGrant >= MaxClaimNodes` (same exhausted error as pool empty).
- Free claim is bootstrapping only (~12 DTN msgs); MST mainly from mining coinbase.

## 代办7 — DTN mst sync + no auto-claim (2026-07-14)

- `mst.GenesisSupply=4_226_880`, `mst.GenesisGrant=128` (was 21M / 1000) so local DTN wallet matches chain economics.
- `StartMesh` no longer auto `ClaimGenesisMST`; user runs `msp -c claim-genesis` or uses chain claim/mine.
- `msp-p2p-test` still manually claims for integration (needs balance to send).
