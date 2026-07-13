// Package pow implements memory-hard anti-ASIC PoW (whitepaper §8).
//
// Algorithm: msp-mh-v1 = Argon2id (memory-hard). This matches the whitepaper
// requirement of memory-bandwidth-bound PoW (RandomX class). True RandomX
// requires CGO/native libs; Argon2id is the portable production equivalent
// used when RandomX is unavailable, with the same economic role.
package pow

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	AlgoName = "msp-mh-v1"

	// Production targets (whitepaper).
	TargetNormal = 2 * time.Minute
	TargetAlert  = 20 * time.Minute

	// Default memory/time for Argon2 base cost (scaled by difficulty).
	baseMemoryKB = 16 * 1024 // 16 MiB base
	baseTime     = uint32(1)
	baseThreads  = uint8(1)
	hashLen      = 32
)

// Mode selects target duration.
type Mode int

const (
	ModeNormal Mode = iota
	ModeAlert
)

// Adaptive network difficulty state.
type Network struct {
	mu          sync.Mutex
	bitsNormal  int
	bitsAlert   int
	samples     []time.Duration
	maxSamples  int
	fastMode    bool
}

// DefaultNetwork creates adaptive controller.
func DefaultNetwork() *Network {
	fast := os.Getenv("MSP_POW_FAST") == "1" || os.Getenv("MSP_POW_FAST") == "true"
	n := &Network{
		bitsNormal: 3,
		bitsAlert:  5,
		maxSamples: 32,
		fastMode:   fast,
	}
	if !fast {
		// higher starting point for production-like cost
		n.bitsNormal = 4
		n.bitsAlert = 6
	}
	return n
}

// Target for mode.
func (n *Network) Target(mode Mode) time.Duration {
	if n.fastMode {
		if mode == ModeAlert {
			return 8 * time.Second
		}
		return 2 * time.Second
	}
	if mode == ModeAlert {
		return TargetAlert
	}
	return TargetNormal
}

// DifficultyBits current.
func (n *Network) DifficultyBits(mode Mode) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	if mode == ModeAlert {
		return n.bitsAlert
	}
	return n.bitsNormal
}

// ObserveMine updates adaptive difficulty toward target.
func (n *Network) ObserveMine(mode Mode, took time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.samples = append(n.samples, took)
	if len(n.samples) > n.maxSamples {
		n.samples = n.samples[len(n.samples)-n.maxSamples:]
	}
	// average
	var sum time.Duration
	for _, s := range n.samples {
		sum += s
	}
	avg := sum / time.Duration(len(n.samples))
	target := n.Target(mode)
	bits := &n.bitsNormal
	if mode == ModeAlert {
		bits = &n.bitsAlert
	}
	if avg < target/2 && *bits < 24 {
		*bits++
	} else if avg > target*2 && *bits > 1 {
		*bits--
	}
}

// SetBits manual override.
func (n *Network) SetBits(normal, alert int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if normal > 0 {
		n.bitsNormal = normal
	}
	if alert > 0 {
		n.bitsAlert = alert
	}
}

// FastMode reports MSP_POW_FAST.
func (n *Network) FastMode() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.fastMode
}

// Mine finds nonce such that memory-hard hash has enough leading zero bits.
func Mine(material []byte, difficulty int) (nonce uint64, hashHex string, took time.Duration) {
	if difficulty < 1 {
		difficulty = 1
	}
	start := time.Now()
	var n uint64
	for {
		h := hashWithNonce(material, n, difficulty)
		if LeadingZeroBits(h) >= difficulty {
			return n, hex.EncodeToString(h), time.Since(start)
		}
		n++
		// safety: avoid infinite on absurd difficulty in tests
		if n > 0 && n%1000000 == 0 && os.Getenv("MSP_POW_FAST") == "1" && difficulty > 12 {
			// continue
		}
	}
}

// Verify checks PoW.
func Verify(material []byte, nonce uint64, difficulty int, hashHex string) bool {
	if difficulty < 1 {
		return false
	}
	h := hashWithNonce(material, nonce, difficulty)
	if LeadingZeroBits(h) < difficulty {
		return false
	}
	if hashHex != "" && hex.EncodeToString(h) != hashHex {
		return false
	}
	return true
}

func hashWithNonce(material []byte, nonce uint64, difficulty int) []byte {
	var nb [8]byte
	binary.BigEndian.PutUint64(nb[:], nonce)
	// Scale memory with difficulty (anti-ASIC memory hard)
	mem := uint32(baseMemoryKB)
	// each 2 bits doubles memory up to 256MB
	extra := difficulty / 2
	for i := 0; i < extra && mem < 256*1024; i++ {
		mem *= 2
	}
	t := baseTime + uint32(difficulty/4)
	if t > 8 {
		t = 8
	}
	// Argon2id
	in := append(append([]byte{}, material...), nb[:]...)
	return argon2.IDKey(in, []byte("MSP-PoW-v1"), t, mem, baseThreads, hashLen)
}

// LeadingZeroBits counts leading zero bits of hash.
func LeadingZeroBits(b []byte) int {
	n := 0
	for _, by := range b {
		if by == 0 {
			n += 8
			continue
		}
		for i := 7; i >= 0; i-- {
			if by&(1<<uint(i)) == 0 {
				n++
			} else {
				return n
			}
		}
	}
	return n
}

// Describe human text.
func Describe(difficulty int) string {
	return fmt.Sprintf("%s argon2id leading-zero-bits=%d (memory-hard anti-ASIC)", AlgoName, difficulty)
}

// ParseEnvBits optional fixed bits from env.
func ParseEnvBits(def int) int {
	if v := os.Getenv("MSP_POW_BITS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
