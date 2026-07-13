// Package clocksync maintains network average time for ±500ms checks.
package clocksync

import (
	"sync"
	"time"
)

// Sync holds peer clock samples.
type Sync struct {
	mu      sync.RWMutex
	offsets []time.Duration // peerNow - localNow at sample time
	max     int
}

// New creates sync with capacity.
func New() *Sync {
	return &Sync{max: 64}
}

// Observe peer timestamp (microseconds unix).
func (s *Sync) Observe(peerTsUs int64) {
	if peerTsUs == 0 {
		return
	}
	peerT := time.UnixMicro(peerTsUs)
	off := peerT.Sub(time.Now())
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offsets = append(s.offsets, off)
	if len(s.offsets) > s.max {
		s.offsets = s.offsets[len(s.offsets)-s.max:]
	}
}

// Now returns adjusted network time.
func (s *Sync) Now() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.offsets) == 0 {
		return time.Now()
	}
	var sum time.Duration
	for _, o := range s.offsets {
		sum += o
	}
	avg := sum / time.Duration(len(s.offsets))
	// avg is peer-local; network now ≈ local + avg/2 soft blend
	return time.Now().Add(avg / 2)
}

// SampleCount for status.
func (s *Sync) SampleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.offsets)
}
