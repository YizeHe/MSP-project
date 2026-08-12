# Errors & security fixes log

## 2026-07-14 DeepSeek-v4-Flash audit (mainnet-2)

### Fixed (CRITICAL / HIGH)

1. **Sender not bound to SenderPub** — attacker could set victim `Sender` + own key.  
   Fix: `Transaction.Verify` requires `NodeIDFromPub(SenderPub) == Sender`.

2. **uint64 overflow on amount+fee** — wrap to 0 mint/stake.  
   Fix: `safeAddUint64` + amount ≤ `GenesisSupply` in transfer/burn/stake.

3. **IssueTicketForBurn inverted guard** — tickets for never-submitted burns.  
   Fix: require burn in mempool or chain.

4. **Clock poisoning before packet auth** — DoS via clock skew.  
   Fix: observe clock only after Verify+PoW; ±10m clamp.

5. **UDP neighbor key spoof** — `touch` trusted any From/EdPub.  
   Fix: require `NodeIDFromPub(EdPub)==From`; don't overwrite conflicting XPub.

6. **Alert PrivateKey() from public seed** — anyone could derive alert key.  
   Fix: `PrivateKey()` only if `MSP_ALERT_SEED` set.

7. **AnonymousBurn orphan transfer** — funding left in mempool after fail.  
   Fix: remove xfer on unfunded path.

8. **GUI CSRF-ish** — no Origin check.  
   Fix: reject non-localhost Origin on `/api/*`.

### Known remaining (not fixed this pass)

- No full fork/reorg / IBD sync
- Chain frames 1-hop only, unauthenticated envelopes
- No multi-process file lock on MSP_DATA
- BurnTicket vs on-chain burn field re-check incomplete
- DTN drop on send failure (no requeue)
