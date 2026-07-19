// internal/adapters/stats/buffer.go
package stats

import (
	"context"
	"log"
	"sync"
	"time"
)

// FlushFunc persists accumulated hit deltas. repo.IncrementHitsBatch already
// satisfies this signature — no adapter needed.
type FlushFunc func(ctx context.Context, deltas map[string]int64) error

// StatsBuffer buffers hit counts in memory and periodically flushes them via
// FlushFunc. It never talks to Mongo directly and holds no repo reference —
// it's handed a function value at construction time.
//
// Flush failures are logged and the batch is dropped (best-effort stats, not
// safety-critical — see ADR 0002).
type StatsBuffer struct {
	mu       sync.Mutex
	hits     map[string]int64
	flush    FlushFunc
	interval time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
}

func NewStatsBuffer(flush FlushFunc, interval time.Duration) *StatsBuffer {
	return &StatsBuffer{
		hits:     make(map[string]int64),
		flush:    flush,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// RecordHit is safe for concurrent use. No I/O, no blocking — safe to call
// from a request-handling goroutine.
func (s *StatsBuffer) RecordHit(code string) {
	s.mu.Lock()
	s.hits[code]++
	s.mu.Unlock()
}

// swap atomically replaces the live map with a fresh one and returns the old
// map for flushing. This is what lets RecordHit calls keep landing
// uncontended while the old map's contents are written to Mongo.
func (s *StatsBuffer) swap() map[string]int64 {
	s.mu.Lock()
	old := s.hits
	s.hits = make(map[string]int64)
	s.mu.Unlock()
	return old
}

func (s *StatsBuffer) flushNow(ctx context.Context) {
	deltas := s.swap()
	if len(deltas) == 0 {
		return
	}
	if err := s.flush(ctx, deltas); err != nil {
		log.Printf("statsbuffer: flush failed, dropping %d entries: %v", len(deltas), err)
	}
}

// Start launches the periodic flush loop in a background goroutine. Call
// once per StatsBuffer instance.
func (s *StatsBuffer) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.flushNow(ctx)
			case <-s.done:
				return
			}
		}
	}()
}

// Stop halts the flush loop and performs one final flush so nothing
// buffered is lost on shutdown. Blocks until the background goroutine has
// exited before flushing, so no RecordHit calls can race the final swap
// (caller is expected to have already stopped accepting new requests, e.g.
// via srv.Shutdown, before calling this).
func (s *StatsBuffer) Stop(ctx context.Context) {
	close(s.done)
	s.wg.Wait()
	s.flushNow(ctx)
}
