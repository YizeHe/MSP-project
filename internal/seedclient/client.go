// Package seedclient talks HTTP to the seed for SIGNALING only (hole punch).
// Message payloads never go through the seed after P2P is up.
package seedclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PeerInfo is discovery metadata from the seed (not a live connection).
type PeerInfo struct {
	NodeID     string   `json:"node_id"`
	Ed25519Pub string   `json:"ed25519_pub"`
	X25519Pub  string   `json:"x25519_pub"`
	UDPPort    int      `json:"udp_port"`
	PublicIP   string   `json:"public_ip"`
	LocalAddrs []string `json:"local_addrs"`
	Candidates []string `json:"candidates"`
	SeenAt     int64    `json:"seen_at"`
}

// HelloRequest registers this node for discovery / punching.
type HelloRequest struct {
	NodeID     string   `json:"node_id"`
	Ed25519Pub string   `json:"ed25519_pub"`
	X25519Pub  string   `json:"x25519_pub"`
	UDPPort    int      `json:"udp_port"`
	LocalAddrs []string `json:"local_addrs"`
}

// HelloResponse is seed observation after register.
type HelloResponse struct {
	OK          bool     `json:"ok"`
	NodeID      string   `json:"node_id"`
	ObservedIP  string   `json:"observed_ip"`
	Candidates  []string `json:"candidates"`
	Peers       int      `json:"peers"`
	Note        string   `json:"note"`
}

// Client is an HTTP signaling client.
type Client struct {
	BaseURL   string
	HTTP      *http.Client
	NodeID    string
	EdPubB64  string
	X25519B64 string
}

// New creates a client. base may be host, host:port, or full URL.
func New(base string) *Client {
	base = NormalizeBase(base)
	return &Client{
		BaseURL: base,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NormalizeBase ensures scheme and no trailing slash.
func NormalizeBase(base string) string {
	base = strings.TrimSpace(base)
	base = strings.TrimRight(base, "/")
	if base == "" {
		return base
	}
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		return base
	}
	if strings.HasPrefix(base, "[") {
		return "http://" + base
	}
	host := base
	if i := strings.Index(base, "/"); i >= 0 {
		host = base[:i]
	}
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	if host == "localhost" || host == "127.0.0.1" || isIPv4(host) {
		return "http://" + base
	}
	return "https://" + base
}

func isIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// Health checks seed signaling service.
func (c *Client) Health() (map[string]any, error) {
	var out map[string]any
	err := c.get("/v1/health", &out)
	return out, err
}

// Hello registers endpoints for hole punch.
func (c *Client) Hello(udpPort int, localAddrs []string) (*HelloResponse, error) {
	body := HelloRequest{
		NodeID:     c.NodeID,
		Ed25519Pub: c.EdPubB64,
		X25519Pub:  c.X25519B64,
		UDPPort:    udpPort,
		LocalAddrs: localAddrs,
	}
	var out HelloResponse
	if err := c.post("/v1/hello", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Peers lists discovery peers from seed.
func (c *Client) Peers() ([]PeerInfo, error) {
	var out struct {
		Peers []PeerInfo `json:"peers"`
	}
	if err := c.get("/v1/peers", &out); err != nil {
		return nil, err
	}
	return out.Peers, nil
}

// LookupPeer returns one peer's crypto + candidates.
func (c *Client) LookupPeer(nodeID string) (*PeerInfo, error) {
	var out PeerInfo
	if err := c.get("/v1/peer/"+url.PathEscape(nodeID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PostSignal sends a small punch coordination signal (not a chat message).
func (c *Client) PostSignal(to, typ string, payload any) error {
	body := map[string]any{
		"from":    c.NodeID,
		"to":      to,
		"type":    typ,
		"payload": payload,
	}
	var out map[string]any
	return c.post("/v1/signal", body, &out)
}

// PullSignals fetches punch signals for this node.
func (c *Client) PullSignals(since string) ([]map[string]any, string, error) {
	q := url.Values{}
	q.Set("to", c.NodeID)
	if since != "" {
		q.Set("since", since)
	}
	var out struct {
		Signals    []map[string]any `json:"signals"`
		NextCursor string           `json:"next_cursor"`
	}
	if err := c.get("/v1/signals?"+q.Encode(), &out); err != nil {
		return nil, "", err
	}
	return out.Signals, out.NextCursor, nil
}

func (c *Client) get(path string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, truncate(body, 200))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(body, dest)
}

func (c *Client) post(path string, payload any, dest any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s: %s: %s", path, resp.Status, truncate(body, 200))
	}
	if dest == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dest)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
