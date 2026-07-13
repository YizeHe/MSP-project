package chain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store persists chain + tip on disk.
type Store struct {
	mu   sync.Mutex
	dir  string
	byH  map[uint64]*Block
	byHash map[string]*Block
	tip  *Block
}

// OpenStore loads from dir.
func OpenStore(dir string) (*Store, error) {
	s := &Store{
		dir:    dir,
		byH:    make(map[uint64]*Block),
		byHash: make(map[string]*Block),
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// load blocks_*.json or chain.jsonl
	path := filepath.Join(dir, "blocks.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var blocks []*Block
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	for _, b := range blocks {
		s.index(b)
	}
	return s, nil
}

func (s *Store) index(b *Block) {
	h := b.Header.HashHex()
	s.byH[b.Header.Height] = b
	s.byHash[h] = b
	if s.tip == nil || b.Header.Height >= s.tip.Header.Height {
		s.tip = b
	}
}

// Append block and persist.
func (s *Store) Append(b *Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *b
	// deep copy txs
	cp.Txs = append([]Transaction(nil), b.Txs...)
	s.index(&cp)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	list := make([]*Block, 0, len(s.byH))
	for h := uint64(0); h <= s.tip.Header.Height; h++ {
		if b, ok := s.byH[h]; ok {
			list = append(list, b)
		}
	}
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "blocks.json"), raw, 0o600)
}

// Tip block.
func (s *Store) Tip() *Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tip
}

// GetByHeight block.
func (s *Store) GetByHeight(h uint64) *Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byH[h]
}

// GetByHash block.
func (s *Store) GetByHash(hash string) *Block {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byHash[hash]
}

// Height tip height.
func (s *Store) Height() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tip == nil {
		return 0
	}
	return s.tip.Header.Height
}

// All headers height-ordered.
func (s *Store) Headers() []BlockHeader {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tip == nil {
		return nil
	}
	out := make([]BlockHeader, 0, s.tip.Header.Height+1)
	for h := uint64(0); h <= s.tip.Header.Height; h++ {
		if b, ok := s.byH[h]; ok {
			out = append(out, b.Header)
		}
	}
	return out
}
