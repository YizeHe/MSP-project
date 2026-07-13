// Local signaling seed (same API as Cloudflare _worker.js v0.2). No packet relay.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	flag.Parse()
	h := newHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handleHealth)
	mux.HandleFunc("/v1/health", h.handleHealth)
	mux.HandleFunc("/v1/hello", h.handleHello)
	mux.HandleFunc("/v1/peers", h.handlePeers)
	mux.HandleFunc("/v1/peer/", h.handlePeer)
	mux.HandleFunc("/v1/packet", h.gone)
	mux.HandleFunc("/v1/packets", h.gone)
	mux.HandleFunc("/v1/signal", h.handleSignal)
	mux.HandleFunc("/v1/signals", h.handleSignals)
	log.Printf("msp signaling seed on http://%s (no data-plane relay)", *addr)
	log.Fatal(http.ListenAndServe(*addr, withCORS(mux)))
}

type peer struct {
	NodeID     string   `json:"node_id"`
	Ed25519Pub string   `json:"ed25519_pub"`
	X25519Pub  string   `json:"x25519_pub"`
	UDPPort    int      `json:"udp_port"`
	PublicIP   string   `json:"public_ip"`
	LocalAddrs []string `json:"local_addrs"`
	Candidates []string `json:"candidates"`
	SeenAt     int64    `json:"seen_at"`
}

type hub struct {
	mu      sync.Mutex
	peers   map[string]peer
	signals []map[string]any
	seq     int
}

func newHub() *hub { return &hub{peers: make(map[string]peer)} }

func (h *hub) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	writeJSON(w, map[string]any{
		"ok": true, "service": "msp-seed-signal-local", "version": "0.2.0-test",
		"role": "signaling-only", "data_plane": "p2p-udp",
		"peers": len(h.peers), "signals": len(h.signals),
		"time_us": time.Now().UnixMicro(),
	})
}

func (h *hub) gone(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(410)
	writeJSON(w, map[string]any{
		"error": "data_plane_removed",
		"message": "signaling only; use P2P UDP after hole punch",
	})
}

func (h *hub) handleHello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var body struct {
		NodeID     string   `json:"node_id"`
		Ed25519Pub string   `json:"ed25519_pub"`
		X25519Pub  string   `json:"x25519_pub"`
		UDPPort    int      `json:"udp_port"`
		LocalAddrs []string `json:"local_addrs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	cands := []string{}
	if ip != "" && body.UDPPort > 0 {
		cands = append(cands, fmt.Sprintf("%s:%d", ip, body.UDPPort))
	}
	for _, a := range body.LocalAddrs {
		if a != "" {
			cands = append(cands, a)
		}
	}
	h.mu.Lock()
	h.peers[body.NodeID] = peer{
		NodeID: body.NodeID, Ed25519Pub: body.Ed25519Pub, X25519Pub: body.X25519Pub,
		UDPPort: body.UDPPort, PublicIP: ip, LocalAddrs: body.LocalAddrs,
		Candidates: cands, SeenAt: time.Now().UnixMilli(),
	}
	n := len(h.peers)
	h.mu.Unlock()
	writeJSON(w, map[string]any{
		"ok": true, "node_id": body.NodeID, "observed_ip": ip,
		"candidates": cands, "peers": n,
		"note": "signaling only",
	})
}

func (h *hub) handlePeers(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := make([]peer, 0, len(h.peers))
	for _, p := range h.peers {
		list = append(list, p)
	}
	writeJSON(w, map[string]any{"peers": list, "role": "signaling-only"})
}

func (h *hub) handlePeer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/peer/")
	h.mu.Lock()
	p, ok := h.peers[id]
	h.mu.Unlock()
	if !ok {
		http.Error(w, `{"error":"peer not found"}`, 404)
		return
	}
	writeJSON(w, p)
}

func (h *hub) handleSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	h.mu.Lock()
	h.seq++
	body["seq"] = h.seq
	body["at"] = time.Now().UnixMilli()
	h.signals = append(h.signals, body)
	seq := h.seq
	h.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "seq": seq})
}

func (h *hub) handleSignals(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	since := 0
	fmt.Sscanf(r.URL.Query().Get("since"), "%d", &since)
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []map[string]any{}
	for _, s := range h.signals {
		seq, _ := s["seq"].(int)
		if seq == 0 {
			if f, ok := s["seq"].(float64); ok {
				seq = int(f)
			}
		}
		if seq <= since {
			continue
		}
		if to != "" {
			sf, _ := s["from"].(string)
			st, _ := s["to"].(string)
			if st != to && sf != to {
				continue
			}
		}
		out = append(out, s)
	}
	next := since
	if len(h.signals) > 0 {
		if s, ok := h.signals[len(h.signals)-1]["seq"].(int); ok {
			next = s
		}
	}
	writeJSON(w, map[string]any{"signals": out, "next_cursor": fmt.Sprintf("%d", next)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "GET,POST,OPTIONS")
		w.Header().Set("access-control-allow-headers", "content-type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
