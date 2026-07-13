// Package p2p: UDP dual-stack neighbor mesh + hole punch + padding.
package p2p

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	maxDatagram = 60 * 1024
	TypePunch      = "punch"
	TypePunchOK    = "punch_ok"
	TypePacket     = "packet"
	TypeKeep       = "keepalive"
	TypeChainBlock = "chain_block"
	TypeChainTx    = "chain_tx"
)

// Envelope UDP frame.
type Envelope struct {
	Type      string          `json:"t"`
	From      string          `json:"from"`
	To        string          `json:"to,omitempty"`
	EdPub     string          `json:"ed,omitempty"`
	XPub      string          `json:"x,omitempty"`
	Packet    json.RawMessage `json:"pkt,omitempty"`
	Chain     json.RawMessage `json:"chain,omitempty"` // block or tx JSON
	Timestamp int64           `json:"ts"`
}

// Neighbor punched peer.
type Neighbor struct {
	NodeID     string
	Addr       *net.UDPAddr
	EdPub      string
	XPub       string
	LastSeen   time.Time
	Punched    bool
	Candidates []string
}

// OnPacket callback for DTN protocol packets.
type OnPacket func(fromNode string, raw []byte, via *net.UDPAddr)

// OnChainMsg callback for chain block/tx frames (JSON body).
type OnChainMsg func(fromNode string, kind string, raw []byte)

// Mesh UDP layer.
type Mesh struct {
	NodeID string
	EdPub  string
	XPub   string

	conn4 *net.UDPConn
	conn6 *net.UDPConn

	mu        sync.RWMutex
	neighbors map[string]*Neighbor
	pending   map[string][]string

	OnPacket   OnPacket
	OnChainMsg OnChainMsg
	Log        func(string)
	PadTo      int
	Quiet      bool
	stop       chan struct{}
}

// Listen dual-stack when possible.
func Listen(port int) (*Mesh, error) {
	m := &Mesh{
		neighbors: make(map[string]*Neighbor),
		pending:   make(map[string][]string),
		stop:      make(chan struct{}),
		Log:       func(string) {},
	}
	a4 := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	c4, err := net.ListenUDP("udp4", a4)
	if err != nil {
		return nil, err
	}
	m.conn4 = c4
	_ = c4.SetReadBuffer(1 << 20)
	_ = c4.SetWriteBuffer(1 << 20)

	// IPv6 optional
	p6 := c4.LocalAddr().(*net.UDPAddr).Port
	a6 := &net.UDPAddr{IP: net.IPv6zero, Port: p6}
	if c6, err := net.ListenUDP("udp6", a6); err == nil {
		m.conn6 = c6
		_ = c6.SetReadBuffer(1 << 20)
		_ = c6.SetWriteBuffer(1 << 20)
		go m.readLoop(c6)
	}
	go m.readLoop(c4)
	go m.keepAliveLoop()
	return m, nil
}

// LocalPort bound port.
func (m *Mesh) LocalPort() int {
	return m.conn4.LocalAddr().(*net.UDPAddr).Port
}

// LocalAddrs candidates.
func (m *Mesh) LocalAddrs() []string {
	port := m.LocalPort()
	out := []string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("[::1]:%d", port)}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				out = append(out, fmt.Sprintf("%s:%d", v4.String(), port))
			} else if ip.To16() != nil {
				out = append(out, fmt.Sprintf("[%s]:%d", ip.String(), port))
			}
		}
	}
	return out
}

// Close mesh.
func (m *Mesh) Close() error {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	if m.conn4 != nil {
		_ = m.conn4.Close()
	}
	if m.conn6 != nil {
		_ = m.conn6.Close()
	}
	return nil
}

// Neighbors snapshot.
func (m *Mesh) Neighbors() []*Neighbor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Neighbor, 0, len(m.neighbors))
	for _, n := range m.neighbors {
		if n.Punched {
			cp := *n
			out = append(out, &cp)
		}
	}
	return out
}

// ConnectedCount punched peers.
func (m *Mesh) ConnectedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, p := range m.neighbors {
		if p.Punched {
			n++
		}
	}
	return n
}

// Punch toward candidates.
func (m *Mesh) Punch(nodeID, ed, x string, candidates []string) {
	if nodeID == "" || nodeID == m.NodeID {
		return
	}
	seen := map[string]struct{}{}
	var list []string
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		list = append(list, c)
	}
	if len(list) == 0 {
		return
	}
	m.mu.Lock()
	m.pending[nodeID] = list
	if _, ok := m.neighbors[nodeID]; !ok {
		m.neighbors[nodeID] = &Neighbor{NodeID: nodeID, EdPub: ed, XPub: x, Candidates: list}
	} else {
		nb := m.neighbors[nodeID]
		if ed != "" {
			nb.EdPub = ed
		}
		if x != "" {
			nb.XPub = x
		}
		nb.Candidates = list
	}
	m.mu.Unlock()
	go m.punchBurst(nodeID, list)
}

func (m *Mesh) punchBurst(nodeID string, candidates []string) {
	env := Envelope{Type: TypePunch, From: m.NodeID, To: nodeID, EdPub: m.EdPub, XPub: m.XPub, Timestamp: time.Now().UnixMicro()}
	raw, _ := json.Marshal(env)
	for i := 0; i < 10; i++ {
		for _, c := range candidates {
			addr, err := net.ResolveUDPAddr("udp", c)
			if err != nil {
				continue
			}
			_ = m.writeTo(m.maybePad(raw), addr)
		}
		select {
		case <-m.stop:
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func (m *Mesh) writeTo(b []byte, addr *net.UDPAddr) error {
	if addr.IP.To4() != nil {
		_, err := m.conn4.WriteToUDP(b, addr)
		return err
	}
	if m.conn6 != nil {
		_, err := m.conn6.WriteToUDP(b, addr)
		return err
	}
	// fallback try conn4
	_, err := m.conn4.WriteToUDP(b, addr)
	return err
}

// SendPacket floods to punched neighbors.
func (m *Mesh) SendPacket(raw []byte) (int, error) {
	env := Envelope{Type: TypePacket, From: m.NodeID, Packet: raw, Timestamp: time.Now().UnixMicro()}
	frame, err := json.Marshal(env)
	if err != nil {
		return 0, err
	}
	if len(frame) > maxDatagram {
		return 0, fmt.Errorf("datagram too large: %d", len(frame))
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	var last error
	for _, nb := range m.neighbors {
		if !nb.Punched || nb.Addr == nil {
			continue
		}
		if err := m.writeTo(m.maybePad(frame), nb.Addr); err != nil {
			last = err
			continue
		}
		n++
	}
	if n == 0 {
		if last != nil {
			return 0, last
		}
		return 0, fmt.Errorf("no punched neighbors")
	}
	return n, nil
}

// ForwardPacket to others except node.
func (m *Mesh) ForwardPacket(raw []byte, exceptNode string) {
	env := Envelope{Type: TypePacket, From: m.NodeID, Packet: raw, Timestamp: time.Now().UnixMicro()}
	frame, _ := json.Marshal(env)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, nb := range m.neighbors {
		if !nb.Punched || nb.Addr == nil || id == exceptNode {
			continue
		}
		_ = m.writeTo(m.maybePad(frame), nb.Addr)
	}
}

// SendToNode tries a specific neighbor.
func (m *Mesh) SendToNode(nodeID string, raw []byte) error {
	env := Envelope{Type: TypePacket, From: m.NodeID, Packet: raw, Timestamp: time.Now().UnixMicro()}
	frame, _ := json.Marshal(env)
	m.mu.RLock()
	nb := m.neighbors[nodeID]
	m.mu.RUnlock()
	if nb == nil || !nb.Punched || nb.Addr == nil {
		return fmt.Errorf("neighbor not punched: %s", nodeID)
	}
	return m.writeTo(m.maybePad(frame), nb.Addr)
}

func (m *Mesh) maybePad(raw []byte) []byte {
	if m.PadTo <= 0 {
		return raw
	}
	return framePad(raw, m.PadTo)
}

func framePad(raw []byte, size int) []byte {
	if size < 4+len(raw) {
		size = 4 + len(raw)
	}
	out := make([]byte, size)
	out[0] = byte(len(raw) >> 24)
	out[1] = byte(len(raw) >> 16)
	out[2] = byte(len(raw) >> 8)
	out[3] = byte(len(raw))
	copy(out[4:], raw)
	return out
}

func unframe(data []byte) []byte {
	if len(data) >= 5 && data[0] == 0 {
		n := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
		if n > 0 && n+4 <= len(data) && n < maxDatagram {
			return data[4 : 4+n]
		}
	}
	return data
}

func (m *Mesh) readLoop(conn *net.UDPConn) {
	buf := make([]byte, maxDatagram)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
				continue
			}
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		m.handleDatagram(unframe(data), addr)
	}
}

func (m *Mesh) handleDatagram(data []byte, addr *net.UDPAddr) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	if env.From == "" || env.From == m.NodeID {
		return
	}
	switch env.Type {
	case TypePunch:
		m.onPunch(env, addr)
	case TypePunchOK:
		m.onPunchOK(env, addr)
	case TypePacket:
		m.touch(env.From, env.EdPub, env.XPub, addr, true)
		if m.OnPacket != nil && len(env.Packet) > 0 {
			m.OnPacket(env.From, env.Packet, addr)
		}
	case TypeChainBlock, TypeChainTx:
		m.touch(env.From, env.EdPub, env.XPub, addr, true)
		if m.OnChainMsg != nil && len(env.Chain) > 0 {
			m.OnChainMsg(env.From, env.Type, env.Chain)
		}
	case TypeKeep:
		m.touch(env.From, env.EdPub, env.XPub, addr, true)
	}
}

func (m *Mesh) onPunch(env Envelope, addr *net.UDPAddr) {
	if env.To != "" && env.To != m.NodeID {
		return
	}
	m.touch(env.From, env.EdPub, env.XPub, addr, true)
	ok := Envelope{Type: TypePunchOK, From: m.NodeID, To: env.From, EdPub: m.EdPub, XPub: m.XPub, Timestamp: time.Now().UnixMicro()}
	raw, _ := json.Marshal(ok)
	_ = m.writeTo(m.maybePad(raw), addr)
	if !m.Quiet {
		m.Log(fmt.Sprintf("punched with %s via %s", short(env.From), addr.String()))
	}
}

func (m *Mesh) onPunchOK(env Envelope, addr *net.UDPAddr) {
	if env.To != "" && env.To != m.NodeID {
		return
	}
	m.touch(env.From, env.EdPub, env.XPub, addr, true)
}

func (m *Mesh) touch(nodeID, ed, x string, addr *net.UDPAddr, punched bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	nb, ok := m.neighbors[nodeID]
	if !ok {
		nb = &Neighbor{NodeID: nodeID}
		m.neighbors[nodeID] = nb
	}
	if ed != "" {
		nb.EdPub = ed
	}
	if x != "" {
		nb.XPub = x
	}
	nb.Addr = addr
	nb.LastSeen = time.Now()
	if punched {
		nb.Punched = true
	}
}

func (m *Mesh) keepAliveLoop() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			env := Envelope{Type: TypeKeep, From: m.NodeID, EdPub: m.EdPub, XPub: m.XPub, Timestamp: time.Now().UnixMicro()}
			raw, _ := json.Marshal(env)
			m.mu.RLock()
			for _, nb := range m.neighbors {
				if nb.Punched && nb.Addr != nil {
					_ = m.writeTo(m.maybePad(raw), nb.Addr)
				}
			}
			var jobs []struct {
				id string
				c  []string
			}
			for id, cands := range m.pending {
				nb := m.neighbors[id]
				if nb != nil && nb.Punched {
					continue
				}
				jobs = append(jobs, struct {
					id string
					c  []string
				}{id, cands})
			}
			m.mu.RUnlock()
			for _, j := range jobs {
				go m.punchBurst(j.id, j.c)
			}
		}
	}
}

// BroadcastChain floods a chain frame to all punched neighbors.
func (m *Mesh) BroadcastChain(kind string, raw []byte) (int, error) {
	env := Envelope{Type: kind, From: m.NodeID, Chain: raw, Timestamp: time.Now().UnixMicro()}
	frame, err := json.Marshal(env)
	if err != nil {
		return 0, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, nb := range m.neighbors {
		if !nb.Punched || nb.Addr == nil {
			continue
		}
		if err := m.writeTo(m.maybePad(frame), nb.Addr); err == nil {
			n++
		}
	}
	return n, nil
}

// HasNeighbor true if punched with node.
func (m *Mesh) HasNeighbor(nodeID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nb, ok := m.neighbors[nodeID]
	return ok && nb.Punched
}

// LookupXPub neighbor x pub.
func (m *Mesh) LookupXPub(nodeID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nb, ok := m.neighbors[nodeID]
	if !ok || nb.XPub == "" {
		return "", false
	}
	return nb.XPub, true
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
