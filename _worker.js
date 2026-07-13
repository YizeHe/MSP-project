/**
 * MST Network seed — SIGNALING / HOLE-PUNCH ONLY (v0.2)
 *
 * Data plane does NOT go through Cloudflare.
 * After peers exchange endpoints and complete UDP punch, the mesh runs P2P-only.
 *
 * Endpoints:
 *   GET  /v1/health
 *   POST /v1/hello     register identity + UDP candidates
 *   GET  /v1/peers     list peers with candidates for punching
 *   GET  /v1/peer/:id
 *   POST /v1/signal    optional punch coordination (relay small signals, not messages)
 *
 * Deploy: npx wrangler deploy
 */

const MAX_PEERS = 500;
const PEER_TTL_MS = 24 * 60 * 60 * 1000;
const MAX_SIGNALS = 500;
const SIGNAL_TTL_MS = 60 * 1000;

const MEM = {
  peers: new Map(),
  signals: [],
  seq: 0,
};

export default {
  async fetch(request, env) {
    if (env && env.HUB) {
      const id = env.HUB.idFromName("msp-global-hub");
      return env.HUB.get(id).fetch(request);
    }
    return handleRequest(request, MEM);
  },
};

export class MspHub {
  constructor(state) {
    this.state = state;
    this.mem = { peers: new Map(), signals: [], seq: 0 };
    this._loaded = false;
  }

  async ensure() {
    if (this._loaded) return;
    await this.state.blockConcurrencyWhile(async () => {
      if (this._loaded) return;
      const peers = await this.state.storage.get("peers");
      const signals = await this.state.storage.get("signals");
      const seq = await this.state.storage.get("seq");
      if (Array.isArray(peers)) this.mem.peers = new Map(peers);
      if (Array.isArray(signals)) this.mem.signals = signals;
      if (typeof seq === "number") this.mem.seq = seq;
      this._loaded = true;
    });
  }

  async persist() {
    await this.state.storage.put({
      peers: [...this.mem.peers.entries()],
      signals: this.mem.signals.slice(-MAX_SIGNALS),
      seq: this.mem.seq,
    });
  }

  async fetch(request) {
    try {
      await this.ensure();
      const res = await handleRequest(request, this.mem);
      if (request.method === "POST") await this.persist();
      return res;
    } catch (err) {
      return cors(json({ error: "hub_exception", message: String(err?.message || err) }, 500));
    }
  }
}

async function handleRequest(request, store) {
  const url = new URL(request.url);
  const path = url.pathname.replace(/\/+$/, "") || "/";

  if (request.method === "OPTIONS") {
    return cors(new Response(null, { status: 204 }));
  }

  try {
    if (path === "/" || path === "/v1/health") {
      prune(store);
      return cors(
        json({
          ok: true,
          service: "msp-seed-signal",
          version: "0.2.0-test",
          role: "signaling-only",
          data_plane: "p2p-udp",
          peers: store.peers.size,
          signals: store.signals.length,
          time_us: Date.now() * 1000,
        })
      );
    }

    // Legacy data-plane routes: explicitly rejected so clients cannot depend on CF relay.
    if (path === "/v1/packet" || path === "/v1/packets") {
      return cors(
        json(
          {
            error: "data_plane_removed",
            message:
              "Cloudflare seed is signaling/hole-punch only. Exchange packets over P2P UDP after punch.",
          },
          410
        )
      );
    }

    if (path === "/v1/hello" && request.method === "POST") {
      const body = await request.json();
      if (!body.node_id || !body.ed25519_pub || !body.x25519_pub) {
        return cors(json({ error: "node_id, ed25519_pub, x25519_pub required" }, 400));
      }
      const clientIP =
        request.headers.get("CF-Connecting-IP") ||
        request.headers.get("True-Client-IP") ||
        request.headers.get("X-Forwarded-For")?.split(",")[0]?.trim() ||
        "";
      const udpPort = Number(body.udp_port) || 0;
      const localAddrs = Array.isArray(body.local_addrs)
        ? body.local_addrs.map(String).slice(0, 16)
        : [];

      const candidates = [];
      if (clientIP && udpPort > 0 && udpPort < 65536) {
        candidates.push(`${clientIP}:${udpPort}`);
      }
      for (const a of localAddrs) {
        if (a && !candidates.includes(a)) candidates.push(a);
      }

      store.peers.set(body.node_id, {
        node_id: body.node_id,
        ed25519_pub: body.ed25519_pub,
        x25519_pub: body.x25519_pub,
        udp_port: udpPort,
        public_ip: clientIP,
        local_addrs: localAddrs,
        candidates,
        seen_at: Date.now(),
      });
      prune(store);
      return cors(
        json({
          ok: true,
          node_id: body.node_id,
          observed_ip: clientIP,
          candidates,
          peers: store.peers.size,
          note: "signaling only; start UDP punch toward peer candidates",
        })
      );
    }

    if (path === "/v1/peers" && request.method === "GET") {
      prune(store);
      const peers = [...store.peers.values()].map(publicPeer);
      return cors(json({ peers, role: "signaling-only" }));
    }

    if (path.startsWith("/v1/peer/") && request.method === "GET") {
      const nodeId = decodeURIComponent(path.slice("/v1/peer/".length));
      const p = store.peers.get(nodeId);
      if (!p) return cors(json({ error: "peer not found" }, 404));
      return cors(json(publicPeer(p)));
    }

    // Small punch coordination signals (not chat payloads).
    if (path === "/v1/signal" && request.method === "POST") {
      const body = await request.json();
      if (!body.from || !body.to || !body.type) {
        return cors(json({ error: "from, to, type required" }, 400));
      }
      // Hard size limit: signals are for punch meta only.
      const raw = JSON.stringify(body);
      if (raw.length > 2048) {
        return cors(json({ error: "signal too large (max 2KB)" }, 413));
      }
      store.seq += 1;
      store.signals.push({
        seq: store.seq,
        at: Date.now(),
        from: String(body.from),
        to: String(body.to),
        type: String(body.type).slice(0, 32),
        payload: body.payload ?? null,
      });
      if (store.signals.length > MAX_SIGNALS) {
        store.signals.splice(0, store.signals.length - MAX_SIGNALS);
      }
      return cors(json({ ok: true, seq: store.seq }));
    }

    if (path === "/v1/signals" && request.method === "GET") {
      prune(store);
      const to = url.searchParams.get("to") || "";
      const since = parseInt(url.searchParams.get("since") || "0", 10) || 0;
      const out = [];
      for (const s of store.signals) {
        if (s.seq <= since) continue;
        if (to && s.to !== to && s.from !== to) continue;
        out.push(s);
        if (out.length >= 100) break;
      }
      const next =
        store.signals.length === 0
          ? String(since)
          : String(store.signals[store.signals.length - 1].seq);
      return cors(json({ signals: out, next_cursor: next }));
    }

    return cors(json({ error: "not found", path }, 404));
  } catch (err) {
    return cors(json({ error: String(err?.message || err) }, 500));
  }
}

function publicPeer(p) {
  return {
    node_id: p.node_id,
    ed25519_pub: p.ed25519_pub,
    x25519_pub: p.x25519_pub,
    udp_port: p.udp_port,
    public_ip: p.public_ip,
    local_addrs: p.local_addrs || [],
    candidates: p.candidates || [],
    seen_at: p.seen_at,
  };
}

function prune(store) {
  const now = Date.now();
  for (const [k, v] of store.peers) {
    if (now - (v.seen_at || 0) > PEER_TTL_MS) store.peers.delete(k);
  }
  while (store.peers.size > MAX_PEERS) {
    store.peers.delete(store.peers.keys().next().value);
  }
  store.signals = store.signals.filter((s) => now - s.at <= SIGNAL_TTL_MS);
}

function json(obj, status = 200) {
  return new Response(JSON.stringify(obj), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" },
  });
}

function cors(res) {
  const h = new Headers(res.headers);
  h.set("access-control-allow-origin", "*");
  h.set("access-control-allow-methods", "GET,POST,OPTIONS");
  h.set("access-control-allow-headers", "content-type");
  return new Response(res.body, { status: res.status, headers: h });
}
