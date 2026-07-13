package chain

import (
	"sync"
)

// Mempool pending transactions.
type Mempool struct {
	mu   sync.Mutex
	byID map[string]*Transaction
	order []string
	max  int
}

// NewMempool creates pool.
func NewMempool(max int) *Mempool {
	if max <= 0 {
		max = 5000
	}
	return &Mempool{byID: make(map[string]*Transaction), max: max}
}

// Add tx if new.
func (m *Mempool) Add(tx *Transaction) error {
	if err := tx.Verify(); err != nil && tx.Type != TxInitAlert {
		return err
	}
	id := tx.TxID()
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; ok {
		return errDupTx
	}
	if len(m.order) >= m.max {
		// drop oldest
		old := m.order[0]
		m.order = m.order[1:]
		delete(m.byID, old)
	}
	cp := *tx
	m.byID[id] = &cp
	m.order = append(m.order, id)
	return nil
}

// PopBatch takes up to n txs (does not remove until Remove).
func (m *Mempool) Peek(n int) []*Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 || n > len(m.order) {
		n = len(m.order)
	}
	out := make([]*Transaction, 0, n)
	for i := 0; i < n; i++ {
		tx := m.byID[m.order[i]]
		if tx != nil {
			cp := *tx
			out = append(out, &cp)
		}
	}
	return out
}

// Remove by ids.
func (m *Mempool) Remove(ids ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rm := map[string]struct{}{}
	for _, id := range ids {
		rm[id] = struct{}{}
		delete(m.byID, id)
	}
	dst := m.order[:0]
	for _, id := range m.order {
		if _, ok := rm[id]; !ok {
			dst = append(dst, id)
		}
	}
	m.order = dst
}

// Len pool size.
func (m *Mempool) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.order)
}

// Has tx id.
func (m *Mempool) Has(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.byID[id]
	return ok
}

// Get tx by id.
func (m *Mempool) Get(id string) *Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	tx := m.byID[id]
	if tx == nil {
		return nil
	}
	cp := *tx
	return &cp
}
