// Package dedup implements the 65536 dual-hash ring cache from the whitepaper.
package dedup

// Capacity fixed by design.
const Capacity = 65536

// Key is dual hash identity of a packet.
type Key struct {
	SHA1   string
	SHA512 string
}

// Ring is a fixed-capacity FIFO ring of seen packet dual-hashes.
type Ring struct {
	order []Key
	set   map[string]struct{}
	cap   int
	head  int
	full  bool
}

// New creates an empty ring with whitepaper capacity.
func New() *Ring {
	return &Ring{
		order: make([]Key, Capacity),
		set:   make(map[string]struct{}, Capacity),
		cap:   Capacity,
	}
}

func composite(k Key) string { return k.SHA1 + "|" + k.SHA512 }

// Seen reports whether dual hash was already recorded.
func (r *Ring) Seen(k Key) bool {
	_, ok := r.set[composite(k)]
	return ok
}

// Add records a key. If already present, returns false (duplicate).
// When full, evicts the oldest entry.
func (r *Ring) Add(k Key) bool {
	c := composite(k)
	if _, ok := r.set[c]; ok {
		return false
	}
	if r.full {
		old := r.order[r.head]
		delete(r.set, composite(old))
	}
	r.order[r.head] = k
	r.set[c] = struct{}{}
	r.head = (r.head + 1) % r.cap
	if r.head == 0 {
		r.full = true
	}
	return true
}

// Len approximate number of stored keys.
func (r *Ring) Len() int {
	if r.full {
		return r.cap
	}
	return r.head
}
