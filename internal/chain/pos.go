package chain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
)

// SelectProposer returns the NodeID scheduled to propose for slot.
// Algorithm (ETH-inspired stake weighting):
//  1. seed = SHA256(prevBlockHash || slot_be64)
//  2. if total active stake == 0 (bootstrap): round-robin sorted active validators;
//     if no validators: empty string (any activated node may propose — cold start)
//  3. else: weighted pick by stake using seed as unbiased index into cumulative stake
func SelectProposer(st *State, prevBlockHash string, slot uint64) string {
	vals := st.ActiveValidators()
	if len(vals) == 0 {
		return "" // cold start: any proposer OK if they activate in-block or first block
	}
	seed := proposerSeed(prevBlockHash, slot)
	total := uint64(0)
	for _, v := range vals {
		total += v.Stake
	}
	if total == 0 {
		// equal weight round-robin
		idx := binary.BigEndian.Uint64(seed[0:8]) % uint64(len(vals))
		return vals[idx].NodeID
	}
	// weighted
	r := binary.BigEndian.Uint64(seed[8:16]) % total
	var acc uint64
	for _, v := range vals {
		acc += v.Stake
		if r < acc {
			return v.NodeID
		}
	}
	return vals[len(vals)-1].NodeID
}

func proposerSeed(prevHash string, slot uint64) [32]byte {
	var sb [8]byte
	binary.BigEndian.PutUint64(sb[:], slot)
	h := sha256.New()
	_, _ = h.Write([]byte(prevHash))
	_, _ = h.Write(sb[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// MayPropose reports whether nodeID is allowed to produce a block at slot.
func MayPropose(st *State, prevHash, nodeID string, slot, height uint64) bool {
	if nodeID == "" {
		return false
	}
	expected := SelectProposer(st, prevHash, slot)
	if expected == "" {
		// cold start / empty set: allow any proposer during bootstrap slots
		if slot < BootstrapSlots {
			return true
		}
		// after bootstrap still empty: allow activate-and-propose path
		return true
	}
	return expected == nodeID
}

// SignBlockHeader attaches proposer signature.
func SignBlockHeader(h *BlockHeader, priv ed25519.PrivateKey) {
	sig := ed25519.Sign(priv, h.SignMaterial())
	h.ProposerSig = base64.StdEncoding.EncodeToString(sig)
	h.ProposerPub = base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	h.MinerID = h.Proposer
}

// VerifyProposerSig checks Ed25519 over header hash.
func VerifyProposerSig(h *BlockHeader) error {
	if h.ProposerSig == "" || h.ProposerPub == "" {
		return fmt.Errorf("%w: missing proposer sig", errBadProposer)
	}
	pub, err := base64.StdEncoding.DecodeString(h.ProposerPub)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errBadProposer
	}
	sig, err := base64.StdEncoding.DecodeString(h.ProposerSig)
	if err != nil {
		return errBadProposer
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), h.SignMaterial(), sig) {
		return errBadProposer
	}
	return nil
}

// ValidatorView for selection (sorted by NodeID for determinism).
type ValidatorView struct {
	NodeID string
	Stake  uint64
}

// ActiveValidators sorted list of active accounts with stake or bootstrap active flag.
func (s *State) ActiveValidators() []ValidatorView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ValidatorView
	for id, a := range s.Accounts {
		if !a.Active {
			continue
		}
		out = append(out, ValidatorView{NodeID: id, Stake: a.Stake})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

// TotalActiveStake sum.
func (s *State) TotalActiveStake() uint64 {
	var t uint64
	for _, v := range s.ActiveValidators() {
		t += v.Stake
	}
	return t
}
