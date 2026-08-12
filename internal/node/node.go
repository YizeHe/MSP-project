// Package node: full MST peer — X3DH, PoW, MST burn, DTN, key flood, clock.
package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/YizeHe/MSP-project/internal/alert"
	"github.com/YizeHe/MSP-project/internal/clocksync"
	"github.com/YizeHe/MSP-project/internal/crypto"
	"github.com/YizeHe/MSP-project/internal/dedup"
	"github.com/YizeHe/MSP-project/internal/dtn"
	"github.com/YizeHe/MSP-project/internal/identity"
	"github.com/YizeHe/MSP-project/internal/mst"
	"github.com/YizeHe/MSP-project/internal/p2p"
	"github.com/YizeHe/MSP-project/internal/pow"
	"github.com/YizeHe/MSP-project/internal/protocol"
	"github.com/YizeHe/MSP-project/internal/securemem"
	"github.com/YizeHe/MSP-project/internal/seedclient"
	"github.com/YizeHe/MSP-project/internal/trust"
)

// Message inbox item (memory only).
type Message struct {
	ReceivedAt time.Time
	Packet     *protocol.Packet
	Text       string
	FromID     string
	ToID       string
	Broadcast  bool
	Alert      bool
	Via        string
}

// Node running peer.
type Node struct {
	ID     *identity.Identity
	Seed   *seedclient.Client
	Mesh   *p2p.Mesh
	Dedup  *dedup.Ring
	Trust  *trust.Store
	Bundle *crypto.PreKeyBundle
	Ledger *mst.Ledger
	PowNet *pow.Network
	Clock  *clocksync.Sync
	DTN    *dtn.Store

	PoWBits          int
	enforceClock     bool
	forwardDelay     bool
	requireFP        bool
	blockAlert       bool
	stopBuiltInAfter int
	usingBuiltIn     bool
	extraSeeds       []string

	mu      sync.Mutex
	inbox   []*Message
	blocked map[string]struct{}
	peerX   map[string]string
	peerEd  map[string]string
	bundles map[string]*crypto.PreKeyBundle

	// last initiator OPK id to stamp on outbound packet
	pendingOPKUsed string

	// TicketValidator validates BurnTicket (set by app when chain is on).
	TicketValidator func(ticketB64, payloadCipher string) error
	RequireTicket   bool

	stopDiscover chan struct{}
	logf         func(string)
}

// Options start options.
type Options struct {
	UDPPort               int
	PoWBits               int
	Log                   func(string)
	EnforceClock          bool
	ForwardDelay          bool
	PadDatagram           bool
	RequireFingerprint    bool
	BlockAlert            bool
	Trust                 *trust.Store
	Bundle                *crypto.PreKeyBundle
	Ledger                *mst.Ledger
	ExtraSeeds            []string
	StopBuiltInAfterPeers int
	UsingBuiltIn          bool
	DataDir               string
}

// Start node.
func Start(id *identity.Identity, seed *seedclient.Client, opt Options) (*Node, error) {
	if opt.Log == nil {
		opt.Log = func(string) {}
	}
	seed.NodeID = id.NodeID
	seed.EdPubB64 = id.Ed25519Pub
	seed.X25519B64 = id.X25519Pub

	mesh, err := p2p.Listen(opt.UDPPort)
	if err != nil {
		return nil, err
	}
	mesh.NodeID = id.NodeID
	mesh.EdPub = id.Ed25519Pub
	mesh.XPub = id.X25519Pub
	mesh.Log = opt.Log
	mesh.Quiet = true
	if opt.PadDatagram {
		mesh.PadTo = 2048
	}

	pn := pow.DefaultNetwork()
	if opt.PoWBits > 0 {
		pn.SetBits(opt.PoWBits, opt.PoWBits+2)
	}

	ledger := opt.Ledger
	if ledger == nil && opt.DataDir != "" {
		ledger, _ = mst.Open(opt.DataDir, id.NodeID)
	}

	n := &Node{
		ID:               id,
		Seed:             seed,
		Mesh:             mesh,
		Dedup:            dedup.New(),
		Trust:            opt.Trust,
		Bundle:           opt.Bundle,
		Ledger:           ledger,
		PowNet:           pn,
		Clock:            clocksync.New(),
		DTN: func() *dtn.Store {
			if opt.DataDir != "" {
				s, err := dtn.Open(filepath.Join(opt.DataDir, "dtn-queue.json"), 2048)
				if err == nil {
					return s
				}
			}
			return dtn.New(2048)
		}(),
		PoWBits:          pn.DifficultyBits(pow.ModeNormal),
		enforceClock:     opt.EnforceClock,
		forwardDelay:     opt.ForwardDelay,
		requireFP:        opt.RequireFingerprint,
		blockAlert:       opt.BlockAlert,
		stopBuiltInAfter: opt.StopBuiltInAfterPeers,
		usingBuiltIn:     opt.UsingBuiltIn,
		extraSeeds:       opt.ExtraSeeds,
		blocked:          make(map[string]struct{}),
		peerX:            make(map[string]string),
		peerEd:           make(map[string]string),
		bundles:          make(map[string]*crypto.PreKeyBundle),
		stopDiscover:     make(chan struct{}),
		logf:             opt.Log,
	}
	if n.Bundle == nil {
		n.Bundle, _ = crypto.GenerateBundle(id.EdPrivate(), id.XPrivate())
	}
	mesh.OnPacket = n.onP2PPacket
	return n, nil
}

// Close node.
func (n *Node) Close() {
	select {
	case <-n.stopDiscover:
	default:
		close(n.stopDiscover)
	}
	n.ClearInbox()
	if n.Mesh != nil {
		_ = n.Mesh.Close()
	}
	if n.Ledger != nil {
		_ = n.Ledger.Save()
	}
	if n.Trust != nil {
		_ = n.Trust.Save()
	}
}

// Join signaling + discovery.
func (n *Node) Join() (*seedclient.HelloResponse, error) {
	local := n.Mesh.LocalAddrs()
	hello, err := n.Seed.Hello(n.Mesh.LocalPort(), local)
	if err != nil {
		return nil, err
	}
	_ = n.DiscoverOnce()
	_ = n.FloodKeyBundle()
	go n.discoverLoop()
	go n.dtnLoop()
	return hello, nil
}

// ClaimGenesisMST mines a light genesis proof and credits wallet.
func (n *Node) ClaimGenesisMST() error {
	if n.Ledger == nil {
		return fmt.Errorf("no ledger")
	}
	mat := []byte("MSP-GENESIS-CLAIM|" + n.ID.NodeID)
	bits := 2
	if !n.PowNet.FastMode() {
		bits = 3
	}
	_, _, _ = pow.Mine(mat, bits)
	if err := n.Ledger.ClaimGenesis(true); err != nil {
		return err
	}
	return n.Ledger.Save()
}

func (n *Node) DiscoverOnce() error {
	// multi-seed discovery
	seeds := n.extraSeeds
	if len(seeds) == 0 {
		seeds = []string{n.Seed.BaseURL}
	}
	// stop built-in after peers
	if n.usingBuiltIn && n.stopBuiltInAfter > 0 && n.Mesh.ConnectedCount() >= n.stopBuiltInAfter {
		n.usingBuiltIn = false
		n.logf("main mesh peers present — stop preferring built-in seeds")
	}
	var last error
	for _, u := range seeds {
		c := seedclient.New(u)
		c.NodeID = n.ID.NodeID
		c.EdPubB64 = n.ID.Ed25519Pub
		c.X25519B64 = n.ID.X25519Pub
		// copy proxy transport if any
		c.HTTP = n.Seed.HTTP
		peers, err := c.Peers()
		if err != nil {
			last = err
			continue
		}
		for _, p := range peers {
			n.ingestPeer(p)
		}
	}
	return last
}

func (n *Node) ingestPeer(p seedclient.PeerInfo) {
	if p.NodeID == n.ID.NodeID {
		return
	}
	if n.Trust != nil {
		if err := n.Trust.Observe(p.NodeID, p.Ed25519Pub, p.X25519Pub); err != nil {
			n.logf(err.Error())
			return
		}
	}
	n.mu.Lock()
	n.peerX[p.NodeID] = p.X25519Pub
	n.peerEd[p.NodeID] = p.Ed25519Pub
	n.mu.Unlock()
	cands := append([]string{}, p.Candidates...)
	cands = append(cands, p.LocalAddrs...)
	if p.PublicIP != "" && p.UDPPort > 0 {
		cands = append(cands, fmt.Sprintf("%s:%d", p.PublicIP, p.UDPPort))
	}
	if p.UDPPort > 0 {
		cands = append(cands, fmt.Sprintf("127.0.0.1:%d", p.UDPPort), fmt.Sprintf("[::1]:%d", p.UDPPort))
	}
	n.Mesh.Punch(p.NodeID, p.Ed25519Pub, p.X25519Pub, cands)
}

func (n *Node) discoverLoop() {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-n.stopDiscover:
			return
		case <-t.C:
			local := n.Mesh.LocalAddrs()
			_, _ = n.Seed.Hello(n.Mesh.LocalPort(), local)
			_ = n.DiscoverOnce()
			_ = n.FloodKeyBundle()
			_ = n.floodTimeSync()
		}
	}
}

func (n *Node) dtnLoop() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-n.stopDiscover:
			return
		case <-t.C:
			if n.Mesh.ConnectedCount() == 0 {
				continue
			}
			for _, it := range n.DTN.PopReady() {
				if it.Target != "" {
					_ = n.Mesh.SendToNode(it.Target, it.Raw)
				} else {
					_, _ = n.Mesh.SendPacket(it.Raw)
				}
			}
		}
	}
}

// FloodKeyBundle MITM layer 2 — announce keys to mesh.
func (n *Node) FloodKeyBundle() error {
	if n.Mesh.ConnectedCount() == 0 {
		return nil
	}
	pub := n.Bundle.PublicJSON()
	rawB, _ := json.Marshal(pub)
	p := &protocol.Packet{
		Version:         protocol.PacketVersion,
		Kind:            protocol.KindKeyFlood,
		TimestampUs:     n.Clock.Now().UnixMicro(),
		SenderID:        n.ID.NodeID,
		Broadcast:       true,
		PoWAlgo:         pow.AlgoName,
		PoWDifficulty:   1,
		TTLMaxHops:      protocol.MaxHops,
		SenderPubKey:    n.ID.Ed25519Pub,
		SenderX25519Pub: n.ID.X25519Pub,
		KeyBundle:       rawB,
		ClockSampleUs:   n.Clock.Now().UnixMicro(),
	}
	// light PoW for control
	mat := p.PoWMaterial()
	nonce, h, _ := pow.Mine(mat, 1)
	p.PoWNonce = nonce
	p.PoWHash = h
	p.Sign(n.ID.EdPrivate())
	raw, _ := protocol.MarshalJSONPacket(p)
	_, err := n.Mesh.SendPacket(raw)
	return err
}

func (n *Node) floodTimeSync() error {
	if n.Mesh.ConnectedCount() == 0 {
		return nil
	}
	p := &protocol.Packet{
		Version:         protocol.PacketVersion,
		Kind:            protocol.KindTimeSync,
		TimestampUs:     n.Clock.Now().UnixMicro(),
		SenderID:        n.ID.NodeID,
		Broadcast:       true,
		PoWAlgo:         pow.AlgoName,
		PoWDifficulty:   1,
		TTLMaxHops:      8,
		SenderPubKey:    n.ID.Ed25519Pub,
		SenderX25519Pub: n.ID.X25519Pub,
		ClockSampleUs:   n.Clock.Now().UnixMicro(),
		DiffHintBits:    n.PowNet.DifficultyBits(pow.ModeNormal),
	}
	mat := p.PoWMaterial()
	nonce, h, _ := pow.Mine(mat, 1)
	p.PoWNonce, p.PoWHash = nonce, h
	p.Sign(n.ID.EdPrivate())
	raw, _ := protocol.MarshalJSONPacket(p)
	_, err := n.Mesh.SendPacket(raw)
	return err
}

// Block / Unblock / IsBlocked
func (n *Node) Block(id string) {
	n.mu.Lock()
	n.blocked[id] = struct{}{}
	n.mu.Unlock()
}
func (n *Node) Unblock(id string) {
	n.mu.Lock()
	delete(n.blocked, id)
	n.mu.Unlock()
}
func (n *Node) IsBlocked(id string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.blocked[id]
	return ok
}

// BlockAlert shields official alert ID.
func (n *Node) BlockAlert() {
	n.blockAlert = true
	n.Block(alert.NodeID())
}

// InjectInbox adds a synthetic inbox row (e.g. chain direct_msg).
func (n *Node) InjectInbox(text, via string, broadcast bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.inbox = append(n.inbox, &Message{
		ReceivedAt: time.Now(),
		Text:       text,
		Via:        via,
		Broadcast:  broadcast,
		FromID:     "chain",
	})
}

// Inbox sorted.
func (n *Node) Inbox() []*Message {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]*Message, len(n.inbox))
	copy(out, n.inbox)
	sort.SliceStable(out, func(i, j int) bool {
		return protocol.LessForSort(out[i].Packet, out[j].Packet)
	})
	return out
}

// ClearInbox secure wipe.
func (n *Node) ClearInbox() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for i := range n.inbox {
		if n.inbox[i] != nil {
			securemem.WipeString(&n.inbox[i].Text)
			n.inbox[i] = nil
		}
	}
	n.inbox = nil
}

// WaitPeers until count or timeout.
func (n *Node) WaitPeers(want int, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for {
		c := n.Mesh.ConnectedCount()
		if c >= want || time.Now().After(deadline) {
			return c
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// TicketFn issues a BurnTicket for the sealed ciphertext (base64).
// Called once after AES-GCM seal so RefHash matches the packet PayloadCipher.
type TicketFn func(cipherB64 string) (ticketB64 string, err error)

// SendUnicast DTN path only. Chain burn is handled by app.Service (代办4).
func (n *Node) SendUnicast(targetID, text string, ttl time.Duration) (*protocol.Packet, error) {
	return n.SendUnicastWithTicketFn(targetID, text, ttl, nil)
}

// SendUnicastWithTicket same as unicast but attaches a pre-issued burn ticket before sign.
// Prefer SendUnicastWithTicketFn so ticket RefHash matches the sealed ciphertext.
func (n *Node) SendUnicastWithTicket(targetID, text string, ttl time.Duration, ticketB64 string) (*protocol.Packet, error) {
	return n.SendUnicastWithTicketFn(targetID, text, ttl, func(string) (string, error) {
		return ticketB64, nil
	})
}

// SendUnicastWithTicketFn seals once, then ticketFn(cipher) → attach ticket → PoW + sign + send.
func (n *Node) SendUnicastWithTicketFn(targetID, text string, ttl time.Duration, ticketFn TicketFn) (*protocol.Packet, error) {
	if n.requireFP && n.Trust != nil {
		if err := n.Trust.RequireVerified(targetID, true); err != nil {
			return nil, err
		}
	}
	bits := n.PowNet.DifficultyBits(pow.ModeNormal)
	key, ephB64, err := n.sessionKeyFor(targetID)
	if err != nil {
		return nil, err
	}
	return n.buildAndSend(targetID, false, false, text, ttl, key, ephB64, bits, 0, ticketFn)
}

// SendBroadcast DTN flood only. Chain burn by app layer.
func (n *Node) SendBroadcast(text string, ttl time.Duration, isAlert bool) (*protocol.Packet, error) {
	return n.SendBroadcastWithTicketFn(text, ttl, isAlert, nil)
}

// SendBroadcastWithTicket attaches a pre-issued burn ticket.
// Prefer SendBroadcastWithTicketFn so ticket RefHash matches the sealed ciphertext.
func (n *Node) SendBroadcastWithTicket(text string, ttl time.Duration, isAlert bool, ticketB64 string) (*protocol.Packet, error) {
	return n.SendBroadcastWithTicketFn(text, ttl, isAlert, func(string) (string, error) {
		return ticketB64, nil
	})
}

// SendBroadcastWithTicketFn seals once, then ticketFn(cipher) → attach ticket → PoW + sign + send.
func (n *Node) SendBroadcastWithTicketFn(text string, ttl time.Duration, isAlert bool, ticketFn TicketFn) (*protocol.Packet, error) {
	mode := pow.ModeNormal
	if isAlert {
		mode = pow.ModeAlert
		if !alert.IsAlertSender(n.ID.Ed25519Pub) {
			return nil, fmt.Errorf("alert flag requires network Alert key (node %s)", alert.NodeID())
		}
	}
	bits := n.PowNet.DifficultyBits(mode)
	return n.buildAndSend("", true, isAlert, text, ttl, crypto.BroadcastKey(), "", bits, 0, ticketFn)
}

func (n *Node) sessionKeyFor(targetID string) (key []byte, ephB64 string, err error) {
	// Prefer full X3DH if we have remote bundle
	n.mu.Lock()
	b := n.bundles[targetID]
	n.mu.Unlock()
	if b != nil {
		ek, err := crypto.NewEphemeral()
		if err != nil {
			return nil, "", err
		}
		key, ephPub, usedOPK, opkID, err := crypto.InitiatorSecret(n.ID.XPrivate(), ek, b)
		if err != nil {
			return nil, "", err
		}
		// OPK single-use on remote cache: strip so next session is 3-DH until new flood
		if usedOPK {
			b.StripRemoteOPK()
			// stash opk id on a side channel via packet field — set on next buildAndSend
			n.mu.Lock()
			n.pendingOPKUsed = opkID
			n.mu.Unlock()
		}
		return key, base64.StdEncoding.EncodeToString(ephPub[:]), nil
	}
	// fallback ECDH long-term
	xB64, ok := n.lookupX(targetID)
	if !ok {
		p, err := n.Seed.LookupPeer(targetID)
		if err != nil {
			return nil, "", fmt.Errorf("peer unknown: %w", err)
		}
		n.ingestPeer(*p)
		xB64 = p.X25519Pub
		n.WaitPeers(1, 3*time.Second)
	}
	xRaw, err := base64.StdEncoding.DecodeString(xB64)
	if err != nil {
		return nil, "", err
	}
	key, err = crypto.DeriveSessionKey(n.ID.XPrivate(), xRaw, "unicast:"+n.ID.NodeID+":"+targetID)
	return key, "", err
}

func (n *Node) lookupX(id string) (string, bool) {
	if x, ok := n.Mesh.LookupXPub(id); ok {
		return x, true
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	x, ok := n.peerX[id]
	return x, ok && x != ""
}

func (n *Node) buildAndSend(targetID string, broadcast, isAlert bool, text string, ttl time.Duration, key []byte, eph string, bits int, burn uint64, ticketFn TicketFn) (*protocol.Packet, error) {
	if len(text) > protocol.MaxPayloadBytes {
		return nil, fmt.Errorf("payload too large")
	}
	if n.Mesh.ConnectedCount() == 0 {
		// queue for DTN carry-forward
		// still build packet and store
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	expire := n.Clock.Now().Add(ttl).UnixMicro()
	pl := protocol.Payload{Text: text, ExpireAtUs: expire}
	plain, err := json.Marshal(pl)
	if err != nil {
		return nil, err
	}
	// Seal once — ticket RefHash must bind this exact ciphertext (代办5 RefHash fix).
	nonceB64, cipherB64, macB64, err := crypto.Seal(key, plain)
	if err != nil {
		return nil, err
	}
	ticketB64 := ""
	if ticketFn != nil {
		ticketB64, err = ticketFn(cipherB64)
		if err != nil {
			return nil, err
		}
	}
	n.mu.Lock()
	opkUsed := n.pendingOPKUsed
	n.pendingOPKUsed = ""
	n.mu.Unlock()

	p := &protocol.Packet{
		Version:         protocol.PacketVersion,
		Kind:            protocol.KindChat,
		TimestampUs:     n.Clock.Now().UnixMicro(),
		SenderID:        n.ID.NodeID,
		TargetID:        targetID,
		Broadcast:       broadcast,
		Alert:           isAlert,
		PoWAlgo:         pow.AlgoName,
		PoWDifficulty:   bits,
		TTLMaxHops:      protocol.MaxHops,
		SenderPubKey:    n.ID.Ed25519Pub,
		SenderX25519Pub: n.ID.X25519Pub,
		EphemeralX:      eph,
		OPKUsed:         opkUsed,
		PayloadNonce:    nonceB64,
		PayloadCipher:   cipherB64,
		PayloadMAC:      macB64,
		ExpireAtUs:      expire,
		MSTBurn:         burn,
		ClockSampleUs:   n.Clock.Now().UnixMicro(),
		DiffHintBits:    bits,
		BurnTicket:      ticketB64,
	}
	mat := p.PoWMaterial()
	start := time.Now()
	nonce, h, took := pow.Mine(mat, bits)
	p.PoWNonce, p.PoWHash = nonce, h
	mode := pow.ModeNormal
	if isAlert {
		mode = pow.ModeAlert
	}
	n.PowNet.ObserveMine(mode, took)
	if !pow.Verify(mat, nonce, bits, h) {
		return nil, fmt.Errorf("pow verify failed")
	}
	p.Sign(n.ID.EdPrivate())
	_ = n.Dedup.Add(dedup.Key{SHA1: p.PacketSHA1, SHA512: p.PacketSHA512})
	raw, err := protocol.MarshalJSONPacket(p)
	if err != nil {
		return nil, err
	}
	if n.Mesh.ConnectedCount() == 0 {
		n.DTN.Push(raw, time.UnixMicro(expire), targetID, "")
		n.logf("no neighbors — queued in DTN store-carry-forward")
		return p, nil
	}
	sent, err := n.Mesh.SendPacket(raw)
	if err != nil {
		n.DTN.Push(raw, time.UnixMicro(expire), targetID, "")
		return p, fmt.Errorf("send failed, queued DTN: %w", err)
	}
	n.logf(fmt.Sprintf("p2p flood %d neighbors pow=%s bits=%d took=%s burn=%d", sent, took.Round(time.Millisecond), bits, took.Round(time.Millisecond), burn))
	_ = start
	return p, nil
}

func (n *Node) onP2PPacket(fromNode string, raw []byte, _ *net.UDPAddr) {
	p, err := protocol.UnmarshalJSONPacket(raw)
	if err != nil {
		return
	}
	p.Hops++
	now := n.Clock.Now()
	if p.DiffHintBits > 0 {
		// soft absorb peer difficulty
	}
	k := dedup.Key{SHA1: p.PacketSHA1, SHA512: p.PacketSHA512}
	if n.Dedup.Seen(k) {
		return
	}
	// STRICT PoW
	if p.PoWDifficulty < 1 || !pow.Verify(p.PoWMaterial(), p.PoWNonce, p.PoWDifficulty, p.PoWHash) {
		return
	}
	checkClock := n.enforceClock && n.Clock.SampleCount() > 0
	if err := p.Verify(now, checkClock); err != nil {
		return
	}
	// Only trust clock samples from authenticated packets (anti clock poisoning)
	if p.ClockSampleUs > 0 {
		// clamp: ignore samples more than 10 minutes off local wall clock
		wall := time.Now().UnixMicro()
		delta := p.ClockSampleUs - wall
		if delta < 0 {
			delta = -delta
		}
		if delta <= int64(10*time.Minute/time.Microsecond) {
			n.Clock.Observe(p.ClockSampleUs)
		}
	}
	if !n.Dedup.Add(k) {
		return
	}
	if n.IsBlocked(p.SenderID) {
		return
	}
	if p.Alert && n.blockAlert {
		return
	}
	if n.Trust != nil {
		if err := n.Trust.Observe(p.SenderID, p.SenderPubKey, p.SenderX25519Pub); err != nil {
			n.logf(err.Error())
			return
		}
	}
	if p.Alert && !alert.IsAlertSender(p.SenderPubKey) {
		n.logf("drop forged alert")
		return
	}

	// key flood
	if p.Kind == protocol.KindKeyFlood && len(p.KeyBundle) > 0 {
		var raw map[string]any
		if json.Unmarshal(p.KeyBundle, &raw) == nil {
			m := map[string]string{}
			for k, v := range raw {
				if s, ok := v.(string); ok {
					m[k] = s
				}
			}
			edRaw, _ := base64.StdEncoding.DecodeString(p.SenderPubKey)
			if len(edRaw) == ed25519.PublicKeySize {
				if b, err := crypto.ParseBundle(m, ed25519.PublicKey(edRaw)); err == nil {
					n.mu.Lock()
					n.bundles[p.SenderID] = b
					n.mu.Unlock()
				}
			}
		}
	}

	// forward
	if p.Hops < p.TTLMaxHops && p.Kind != protocol.KindTimeSync {
		raw2, err := protocol.MarshalJSONPacket(p)
		if err == nil {
			if n.forwardDelay {
				go func() {
					ms, _ := rand.Int(rand.Reader, big.NewInt(2990))
					time.Sleep(time.Duration(10+ms.Int64()) * time.Millisecond)
					n.Mesh.ForwardPacket(raw2, fromNode)
				}()
			} else {
				n.Mesh.ForwardPacket(raw2, fromNode)
			}
		}
	}

	if p.Kind == protocol.KindKeyFlood || p.Kind == protocol.KindTimeSync || p.Kind == protocol.KindDiffHint {
		return
	}

	// 代办5: BurnTicket validation for chat
	if p.Kind == protocol.KindChat || p.Kind == "" {
		if n.RequireTicket || p.BurnTicket != "" {
			if n.TicketValidator != nil {
				if err := n.TicketValidator(p.BurnTicket, p.PayloadCipher); err != nil {
					n.logf("drop: burn ticket: " + err.Error())
					return
				}
			} else if n.RequireTicket {
				n.logf("drop: ticket required but no validator")
				return
			}
		}
	}

	forMe := p.Broadcast || p.TargetID == n.ID.NodeID
	if !forMe {
		return
	}
	text, err := n.decrypt(p)
	if err != nil {
		return
	}
	msg := &Message{ReceivedAt: now, Packet: p, Text: text, FromID: p.SenderID, ToID: p.TargetID, Broadcast: p.Broadcast, Alert: p.Alert, Via: fromNode}
	n.mu.Lock()
	n.inbox = append(n.inbox, msg)
	n.mu.Unlock()
	n.logf(fmt.Sprintf("inbox+ from=%s", short(p.SenderID)))
}

func (n *Node) decrypt(p *protocol.Packet) (string, error) {
	var key []byte
	var err error
	if p.Broadcast {
		key = crypto.BroadcastKey()
	} else if p.EphemeralX != "" && n.Bundle != nil {
		// X3DH responder
		var remoteIK, remoteEK [32]byte
		ik, _ := base64.StdEncoding.DecodeString(p.SenderX25519Pub)
		ek, _ := base64.StdEncoding.DecodeString(p.EphemeralX)
		if len(ik) != 32 || len(ek) != 32 {
			return "", fmt.Errorf("bad x3dh keys")
		}
		copy(remoteIK[:], ik)
		copy(remoteEK[:], ek)
		// If peer referenced an OPK we no longer have, try 3-DH; else use current OPK only if OPKUsed matches or empty
		spk, opk, has := n.Bundle.BundlePriv()
		// Prefer 4-DH when we still have an OPK and peer may have used it
		useOPK := has && (p.OPKUsed != "" || has)
		key, err = crypto.ResponderSecret(n.ID.XPrivate(), spk, opk, useOPK, remoteIK, remoteEK)
		if err != nil && useOPK {
			// peer may have used older OPK already rotated — try 3-DH
			key, err = crypto.ResponderSecret(n.ID.XPrivate(), spk, opk, false, remoteIK, remoteEK)
			useOPK = false
		}
		if err != nil {
			return "", err
		}
		// OPK single-use: destroy after successful 4-DH, mint new, re-flood
		if useOPK {
			if n.Bundle.ConsumeOPK() {
				_ = n.Bundle.RotateOPK()
				go func() { _ = n.FloodKeyBundle() }()
			}
		}
	} else {
		xRaw, err2 := base64.StdEncoding.DecodeString(p.SenderX25519Pub)
		if err2 != nil {
			return "", err2
		}
		key, err = crypto.DeriveSessionKey(n.ID.XPrivate(), xRaw, "unicast:"+p.SenderID+":"+n.ID.NodeID)
		if err != nil {
			return "", err
		}
	}
	plain, err := crypto.Open(key, p.PayloadNonce, p.PayloadCipher, p.PayloadMAC)
	if err != nil {
		return "", err
	}
	var pl protocol.Payload
	if err := json.Unmarshal(plain, &pl); err != nil {
		return "", err
	}
	return pl.Text, nil
}

// Status map.
func (n *Node) Status() map[string]any {
	n.mu.Lock()
	defer n.mu.Unlock()
	bal, burned, claimed := uint64(0), uint64(0), false
	if n.Ledger != nil {
		bal, burned, claimed = n.Ledger.Snapshot()
	}
	ids := []string{}
	for _, nb := range n.Mesh.Neighbors() {
		ids = append(ids, nb.NodeID)
	}
	return map[string]any{
		"node_id": n.ID.NodeID, "fingerprint": n.ID.Fingerprint,
		"seed": n.Seed.BaseURL, "seed_role": "signaling-only", "data_plane": "p2p-udp",
		"udp_port": n.Mesh.LocalPort(), "local_addrs": n.Mesh.LocalAddrs(),
		"p2p_neighbors": n.Mesh.ConnectedCount(), "neighbor_ids": ids,
		"pow_algo": pow.AlgoName, "pow_bits_normal": n.PowNet.DifficultyBits(pow.ModeNormal),
		"pow_bits_alert": n.PowNet.DifficultyBits(pow.ModeAlert),
		"inbox": len(n.inbox), "dedup": n.Dedup.Len(), "blocked": len(n.blocked),
		"dtn_pending": n.DTN.Len(), "clock_samples": n.Clock.SampleCount(),
		"mst_balance": bal, "mst_burned": burned, "mst_claimed": claimed,
		"alert_node": alert.NodeID(), "block_alert": n.blockAlert,
		"enforce_clock": n.enforceClock, "bundles_cached": len(n.bundles),
	}
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
