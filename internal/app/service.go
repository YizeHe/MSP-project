// Package app is the unified service layer used by CLI (-c) and GUI.
package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"net"
	"net/http"
	"net/url"

	"encoding/json"

	"github.com/YizeHe/MSP-project/internal/alert"
	"github.com/YizeHe/MSP-project/internal/chain"
	"github.com/YizeHe/MSP-project/internal/config"
	"github.com/YizeHe/MSP-project/internal/crypto"
	"github.com/YizeHe/MSP-project/internal/identity"
	"github.com/YizeHe/MSP-project/internal/mst"
	"github.com/YizeHe/MSP-project/internal/node"
	"github.com/YizeHe/MSP-project/internal/p2p"
	"github.com/YizeHe/MSP-project/internal/protocol"
	"github.com/YizeHe/MSP-project/internal/securemem"
	"github.com/YizeHe/MSP-project/internal/seedclient"
	"github.com/YizeHe/MSP-project/internal/trust"
	"golang.org/x/net/proxy"
)

// Service is the process-wide MSP controller.
type Service struct {
	mu      sync.Mutex
	DataDir string
	Cfg     *config.Config
	ID      *identity.Identity
	Trust   *trust.Store
	Node    *node.Node
	Chain   *chain.Engine
	Bundle  *crypto.PreKeyBundle
	Log     func(string)
}

// NewService loads config/identity/trust (identity optional).
func NewService(dataDir string) (*Service, error) {
	if dataDir == "" {
		dataDir = config.DefaultDir()
	}
	cfg, err := config.Load(dataDir)
	if err != nil {
		return nil, err
	}
	s := &Service{
		DataDir: dataDir,
		Cfg:     cfg,
		Log:     func(string) {},
	}
	tp := filepath.Join(dataDir, "trust.json")
	ts, err := trust.Open(tp)
	if err != nil {
		return nil, err
	}
	s.Trust = ts
	if _, err := os.Stat(cfg.IdentityPath); err == nil {
		pass := os.Getenv("MSP_PASSPHRASE")
		id, err := identity.Load(cfg.IdentityPath, pass)
		if err != nil {
			return nil, err
		}
		s.ID = id
		b, err := crypto.GenerateBundle(id.EdPrivate(), id.XPrivate())
		if err != nil {
			return nil, err
		}
		s.Bundle = b
	}
	return s, nil
}

// ReloadConfig from disk.
func (s *Service) ReloadConfig() error {
	cfg, err := config.Load(s.DataDir)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.Cfg = cfg
	s.mu.Unlock()
	return nil
}

// InitIdentity generates new identity.
func (s *Service) InitIdentity(printMnemonic bool, passphrase string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.Cfg.IdentityPath); err == nil {
		return nil, fmt.Errorf("identity already exists at %s", s.Cfg.IdentityPath)
	}
	id, err := identity.Generate()
	if err != nil {
		return nil, err
	}
	if err := id.Save(s.Cfg.IdentityPath, passphrase); err != nil {
		return nil, err
	}
	_ = config.Save(s.Cfg)
	s.ID = id
	b, err := crypto.GenerateBundle(id.EdPrivate(), id.XPrivate())
	if err != nil {
		return nil, err
	}
	s.Bundle = b
	out := map[string]string{
		"node_id":     id.NodeID,
		"fingerprint": id.Fingerprint,
		"path":        s.Cfg.IdentityPath,
	}
	if printMnemonic {
		out["mnemonic"] = id.Mnemonic
	}
	return out, nil
}

// ImportIdentity from mnemonic words.
func (s *Service) ImportIdentity(words []string, passphrase string) (map[string]string, error) {
	id, err := identity.FromMnemonic(strings.Join(words, " "), passphrase)
	if err != nil {
		return nil, err
	}
	if err := id.Save(s.Cfg.IdentityPath, passphrase); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ID = id
	b, _ := crypto.GenerateBundle(id.EdPrivate(), id.XPrivate())
	s.Bundle = b
	s.mu.Unlock()
	_ = config.Save(s.Cfg)
	return map[string]string{"node_id": id.NodeID, "fingerprint": id.Fingerprint}, nil
}

// IdentityInfo public metadata.
func (s *Service) IdentityInfo() (map[string]string, error) {
	if s.ID == nil {
		return nil, fmt.Errorf("no identity — run init")
	}
	return map[string]string{
		"node_id":     s.ID.NodeID,
		"fingerprint": s.ID.Fingerprint,
		"ed25519":     s.ID.Ed25519Pub,
		"x25519":      s.ID.X25519Pub,
	}, nil
}

// SeedList returns seeds.
func (s *Service) SeedList() []string {
	return s.Cfg.AllSeeds(true)
}

// SeedAdd adds custom seed.
func (s *Service) SeedAdd(url string) error {
	url = seedclient.NormalizeBase(url)
	s.Cfg.AddSeed(url)
	return config.Save(s.Cfg)
}

// SeedRemove removes seed.
func (s *Service) SeedRemove(url string) error {
	url = seedclient.NormalizeBase(url)
	if !s.Cfg.RemoveSeed(url) {
		return fmt.Errorf("seed not in list: %s", url)
	}
	return config.Save(s.Cfg)
}

// SeedSet replaces list with single primary.
func (s *Service) SeedSet(url string) error {
	url = seedclient.NormalizeBase(url)
	s.Cfg.Seeds = []string{url}
	return config.Save(s.Cfg)
}

// Health pings primary seed.
func (s *Service) Health() (map[string]any, error) {
	u := s.Cfg.PrimarySeed()
	if u == "" {
		return nil, fmt.Errorf("no seed configured")
	}
	c := seedclient.New(u)
	return c.Health()
}

// StartMesh joins network (P2P + signaling).
func (s *Service) StartMesh() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Node != nil {
		return nil
	}
	if s.ID == nil {
		return fmt.Errorf("no identity")
	}
	primary := s.Cfg.PrimarySeed()
	if primary == "" {
		return fmt.Errorf("no seed — msp -c seed set <url> or enable built-in")
	}
	sc := seedclient.New(primary)
	sc.HTTP = socksAwareHTTPClient()
	// detect built-in usage
	usingBuiltIn := false
	for _, b := range config.BuiltInSeeds {
		if b == primary && len(s.Cfg.Seeds) == 0 {
			usingBuiltIn = true
		}
	}
	ledger, _ := mst.Open(s.DataDir, s.ID.NodeID)
	n, err := node.Start(s.ID, sc, node.Options{
		UDPPort:               s.Cfg.UDPPort,
		PoWBits:               s.Cfg.PoWDifficulty,
		EnforceClock:          true, // whitepaper default on
		ForwardDelay:          s.Cfg.ForwardDelay,
		PadDatagram:           s.Cfg.PadDatagram,
		RequireFingerprint:    false,
		Trust:                 s.Trust,
		Bundle:                s.Bundle,
		Ledger:                ledger,
		ExtraSeeds:            s.Cfg.AllSeeds(true),
		StopBuiltInAfterPeers: s.Cfg.StopBuiltInAfterPeers,
		UsingBuiltIn:          usingBuiltIn || s.Cfg.UseBuiltInSeeds,
		DataDir:               s.DataDir,
		Log:                   s.Log,
	})
	if err != nil {
		return err
	}
	if _, err := n.Join(); err != nil {
		// seed may be temporarily unreachable — still run local mesh + chain
		s.Log("seed join warning: " + err.Error() + " (local mesh/chain continue)")
	}
	// 代办7: do NOT auto-claim DTN MST on start — user must `msp -c claim-genesis`
	// or earn via chain-claim / mining. Local ledger may stay unclaimed until then.
	// open public chain (persisted under dataDir/chain)
	eng, err := chain.OpenEngine(filepath.Join(s.DataDir, "chain"), s.ID)
	if err != nil {
		n.Close()
		return err
	}
	eng.Log = s.Log
	eng.OnBlock = s.onChainBlock
	n.Mesh.OnChainMsg = s.onChainP2P
	// Production: always require BurnTicket on chat when chain is online
	n.RequireTicket = true
	n.TicketValidator = func(ticketB64, payloadCipher string) error {
		t, err := chain.DecodeTicket(ticketB64)
		if err != nil {
			return err
		}
		return chain.ValidateBurnTicket(t, payloadCipher, eng.HasTxHash, time.Now())
	}
	// PoS slot proposer loop (10 min slots; proposes when elected)
	eng.StartMiner(true)
	s.Chain = eng
	s.Node = n
	return nil
}

func (s *Service) onChainP2P(from, kind string, raw []byte) {
	if s.Chain == nil {
		return
	}
	switch kind {
	case p2p.TypeChainBlock:
		var b chain.Block
		if json.Unmarshal(raw, &b) != nil {
			return
		}
		if err := s.Chain.AcceptBlock(&b); err != nil {
			s.Log("chain accept: " + err.Error())
		}
	case p2p.TypeChainTx:
		var tx chain.Transaction
		if json.Unmarshal(raw, &tx) != nil {
			return
		}
		if err := s.Chain.SubmitTx(&tx); err != nil {
			// ignore dup/invalid from peers
			_ = err
		}
	}
}

func (s *Service) onChainBlock(b *chain.Block) {
	// 代办4: chain has NO message delivery — only propagate blocks for ledger sync
	if s.Node != nil {
		raw, _ := json.Marshal(b)
		_, _ = s.Node.Mesh.BroadcastChain(p2p.TypeChainBlock, raw)
	}
}

// broadcastTxToMesh helper.
func (s *Service) broadcastTxToMesh(tx *chain.Transaction) {
	if s.Node == nil {
		return
	}
	raw, _ := json.Marshal(tx)
	_, _ = s.Node.Mesh.BroadcastChain(p2p.TypeChainTx, raw)
}

func socksAwareHTTPClient() *http.Client {
	// Tor / onion: ALL_PROXY or SOCKS5_PROXY=socks5://127.0.0.1:9050
	proxyURL := os.Getenv("ALL_PROXY")
	if proxyURL == "" {
		proxyURL = os.Getenv("SOCKS5_PROXY")
	}
	if proxyURL == "" {
		// still honor HTTP_PROXY via default transport
		return &http.Client{Timeout: 30 * time.Second}
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return &http.Client{Timeout: 30 * time.Second}
	}
	var dialer proxy.Dialer = proxy.Direct
	if u.Scheme == "socks5" || u.Scheme == "socks5h" {
		d, err := proxy.FromURL(u, proxy.Direct)
		if err == nil {
			dialer = d
		}
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	// also env HTTP proxy
	tr.Proxy = http.ProxyFromEnvironment
	return &http.Client{Timeout: 45 * time.Second, Transport: tr}
}

// StopMesh stops P2P + chain miner.
func (s *Service) StopMesh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Chain != nil {
		s.Chain.Close()
		s.Chain = nil
	}
	if s.Node != nil {
		s.Node.Close()
		s.Node = nil
	}
	_ = s.Trust.Save()
}

// EnsureMesh starts if needed.
func (s *Service) EnsureMesh() error {
	s.mu.Lock()
	ready := s.Node != nil
	s.mu.Unlock()
	if ready {
		return nil
	}
	return s.StartMesh()
}

// Status map.
func (s *Service) Status() map[string]any {
	out := map[string]any{
		"version":  Version,
		"data_dir": s.DataDir,
		"seeds":    s.Cfg.AllSeeds(true),
		"primary":  s.Cfg.PrimarySeed(),
		"running":  s.Node != nil,
	}
	if s.ID != nil {
		out["node_id"] = s.ID.NodeID
		out["fingerprint"] = s.ID.Fingerprint
	}
	if s.Node != nil {
		for k, v := range s.Node.Status() {
			out[k] = v
		}
	}
	if s.Chain != nil {
		out["chain"] = s.Chain.Status()
	}
	return out
}

// Discover peers.
func (s *Service) Discover() error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	return s.Node.DiscoverOnce()
}

// Peers punched + discovery.
func (s *Service) Peers() (punched []map[string]string, discovered []seedclient.PeerInfo, err error) {
	if err := s.EnsureMesh(); err != nil {
		return nil, nil, err
	}
	for _, nb := range s.Node.Mesh.Neighbors() {
		punched = append(punched, map[string]string{
			"node_id": nb.NodeID,
			"addr":    nb.Addr.String(),
		})
	}
	discovered, err = s.Node.Seed.Peers()
	return
}

// Send: 代办5 — seal once → burner 匿名 burn(RefHash=cipher) → ticket → PoW/sign/send.
// Chain holds no message body; DTN carries ciphertext only.
func (s *Service) Send(to, text string) (*protocol.Packet, error) {
	if err := s.EnsureMesh(); err != nil {
		return nil, err
	}
	s.Node.WaitPeers(1, 8*time.Second)
	msgType := chain.MsgDTN
	if s.Node.Mesh.HasNeighbor(to) {
		msgType = chain.MsgDirect
	}
	return s.Node.SendUnicastWithTicketFn(to, text, 24*time.Hour, func(cipherB64 string) (string, error) {
		ref := chain.RefHashPayloadCipher(cipherB64)
		ticket, err := s.anonymousBurnTicket(msgType, ref)
		if err != nil {
			return "", err
		}
		return ticket.Encode(), nil
	})
}

// Broadcast 代办5 — same seal-once ticket path for flood/alert.
func (s *Service) Broadcast(text string, isAlert bool) (*protocol.Packet, error) {
	if err := s.EnsureMesh(); err != nil {
		return nil, err
	}
	msgType := chain.MsgBroadcast
	if isAlert {
		msgType = chain.MsgAlert
	}
	s.Node.WaitPeers(1, 8*time.Second)
	return s.Node.SendBroadcastWithTicketFn(text, 24*time.Hour, isAlert, func(cipherB64 string) (string, error) {
		ref := chain.RefHashPayloadCipher(cipherB64)
		ticket, err := s.anonymousBurnTicket(msgType, ref)
		if err != nil {
			return "", err
		}
		return ticket.Encode(), nil
	})
}

// anonymousBurnTicket: fund one-time burner → burn → IssueBurnTicket (RefHash provided).
func (s *Service) anonymousBurnTicket(msgType, refHash string) (*chain.BurnTicket, error) {
	if s.Chain == nil {
		return nil, fmt.Errorf("chain required for burn ticket")
	}
	ticket, burnTx, err := s.Chain.AnonymousBurn(msgType, refHash)
	if err != nil {
		// fallback: non-anonymous burn from main identity
		s.Log("anonymous burn failed, fallback main: " + err.Error())
		tx, err2 := s.Chain.BurnForMessage(msgType, refHash)
		if err2 != nil {
			return nil, err2
		}
		if err2 = s.Chain.SubmitTx(tx); err2 != nil {
			return nil, err2
		}
		s.broadcastTxToMesh(tx)
		ticket, err2 = s.Chain.IssueTicketForBurn(tx)
		if err2 != nil {
			return nil, err2
		}
		return ticket, nil
	}
	s.broadcastTxToMesh(burnTx)
	return ticket, nil
}

// Inbox messages.
func (s *Service) Inbox() ([]*node.Message, error) {
	if s.Node == nil {
		return nil, fmt.Errorf("mesh not running — start first")
	}
	return s.Node.Inbox(), nil
}

// ClearInbox secure wipe.
func (s *Service) ClearInbox() {
	if s.Node != nil {
		s.Node.ClearInbox()
	}
}

// Block peer.
func (s *Service) Block(id string) error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	s.Node.Block(id)
	return nil
}

// Unblock peer.
func (s *Service) Unblock(id string) error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	s.Node.Unblock(id)
	return nil
}

// VerifyFingerprint user confirms fp.
func (s *Service) VerifyFingerprint(nodeID, fp string) error {
	fp = strings.TrimSpace(strings.ToLower(fp))
	rec, ok := s.Trust.Get(nodeID)
	if !ok {
		// try compute from discovery
		if err := s.EnsureMesh(); err == nil {
			if p, err := s.Node.Seed.LookupPeer(nodeID); err == nil {
				_ = s.Trust.Observe(p.NodeID, p.Ed25519Pub, p.X25519Pub)
				rec, ok = s.Trust.Get(nodeID)
			}
		}
	}
	if !ok {
		return fmt.Errorf("unknown peer")
	}
	raw, err := base64.StdEncoding.DecodeString(rec.Ed25519Pub)
	if err != nil {
		return err
	}
	want := protocol.Fingerprint16(raw)
	if !strings.HasPrefix(want, fp) && want != fp {
		return fmt.Errorf("fingerprint mismatch: expected %s", want)
	}
	if err := s.Trust.SetVerified(nodeID, want); err != nil {
		return err
	}
	return s.Trust.Save()
}

// TrustList records.
func (s *Service) TrustList() []*trust.Record {
	return s.Trust.List()
}

// WaitPeers helper.
func (s *Service) WaitPeers(n int, d time.Duration) int {
	if s.Node == nil {
		return 0
	}
	return s.Node.WaitPeers(n, d)
}

// WipeSensitive best-effort on shutdown.
func (s *Service) WipeSensitive() {
	s.StopMesh()
	if s.ID != nil {
		securemem.WipeString(&s.ID.Mnemonic)
	}
}

// Version of app layer.
const Version = "2.0.0-mainnet-pos"

// ChainStatus public chain status.
func (s *Service) ChainStatus() map[string]any {
	if s.Chain == nil {
		return map[string]any{"error": "chain not running — start mesh first"}
	}
	return s.Chain.Status()
}

// ChainGenesis returns frozen mainnet genesis info (no mesh required).
func (s *Service) ChainGenesis() map[string]any {
	return chain.GenesisInfo()
}

// ChainClaimGenesis disabled on mainnet-2.
func (s *Service) ChainClaimGenesis() error {
	return fmt.Errorf("free claim disabled on %s — use stake/propose (msp -c chain-mine / chain-activate)", chain.ChainID)
}

// ChainMine proposes a PoS block when eligible (force for bootstrap).
func (s *Service) ChainMine(force bool) (*chain.Block, error) {
	if err := s.EnsureMesh(); err != nil {
		return nil, err
	}
	return s.Chain.ProposeOnce(force)
}

// ChainStake locks MST for PoS weight.
func (s *Service) ChainStake(amount uint64) error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	tx, err := s.Chain.Stake(amount)
	if err != nil {
		return err
	}
	if err := s.Chain.SubmitTx(tx); err != nil {
		return err
	}
	s.broadcastTxToMesh(tx)
	_, _ = s.Chain.ProposeOnce(true)
	return nil
}

// ChainUnstake unlocks stake to liquid.
func (s *Service) ChainUnstake(amount uint64) error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	tx, err := s.Chain.Unstake(amount)
	if err != nil {
		return err
	}
	if err := s.Chain.SubmitTx(tx); err != nil {
		return err
	}
	s.broadcastTxToMesh(tx)
	_, _ = s.Chain.ProposeOnce(true)
	return nil
}

// ChainActivate joins validator set.
func (s *Service) ChainActivate() error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	tx, err := s.Chain.Activate()
	if err != nil {
		return err
	}
	if err := s.Chain.SubmitTx(tx); err != nil {
		return err
	}
	s.broadcastTxToMesh(tx)
	_, _ = s.Chain.ProposeOnce(true)
	return nil
}

// ChainTransfer MST on ledger.
func (s *Service) ChainTransfer(to string, amount uint64) error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	tx, err := s.Chain.Transfer(to, amount)
	if err != nil {
		return err
	}
	if err := s.Chain.SubmitTx(tx); err != nil {
		return err
	}
	s.broadcastTxToMesh(tx)
	_, _ = s.Chain.MineOnce(false)
	return nil
}

// ChainRegister registers public keys once.
func (s *Service) ChainRegister() error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	tx, err := s.Chain.Register()
	if err != nil {
		return err
	}
	if err := s.Chain.SubmitTx(tx); err != nil {
		return err
	}
	s.broadcastTxToMesh(tx)
	_, _ = s.Chain.MineOnce(false)
	return nil
}

// ChainHeight tip.
func (s *Service) ChainHeight() uint64 {
	if s.Chain == nil {
		return 0
	}
	return s.Chain.TipHeight()
}

// ClaimGenesis disabled (local DTN free grant removed for production).
func (s *Service) ClaimGenesis() error {
	return fmt.Errorf("local free MST claim disabled — earn on-chain via PoS proposing")
}

// BlockAlert shields official alert.
func (s *Service) BlockAlert() error {
	if err := s.EnsureMesh(); err != nil {
		return err
	}
	s.Node.BlockAlert()
	return nil
}

// AlertInfo returns alert authority status.
func (s *Service) AlertInfo() map[string]any {
	return alert.Status()
}

// MSTInfo balance.
func (s *Service) MSTInfo() map[string]any {
	if s.Node == nil || s.Node.Ledger == nil {
		return map[string]any{"error": "mesh not running"}
	}
	b, burned, claimed := s.Node.Ledger.Snapshot()
	return map[string]any{
		"balance": b, "burned": burned, "claimed": claimed,
		"genesis_supply": mst.GenesisSupply, "grant": mst.GenesisGrant,
	}
}

// SetRequireFingerprint toggles strict FP.
func (s *Service) SetRequireFingerprint(v bool) {
	if s.Node != nil {
		// field unexported — restart mesh with config later
		_ = v
	}
}
