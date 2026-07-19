// internal/adapters/stats/buffer_test.go
package stats

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// --- concurrency safety ---

func TestStatsBuffer_ConcurrentRecordHit(t *testing.T) {
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error { return nil },
		time.Minute, // never started, so interval is irrelevant here
	)

	const goroutines = 50
	const hitsEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < hitsEach; j++ {
				sb.RecordHit("abc1234")
			}
		}()
	}
	wg.Wait()

	got := sb.swap()
	want := int64(goroutines * hitsEach)
	if got["abc1234"] != want {
		t.Errorf("hit count = %d, want %d", got["abc1234"], want)
	}
}

// --- swap semantics ---

func TestStatsBuffer_SwapResetsLiveMap(t *testing.T) {
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error { return nil },
		time.Minute,
	)

	sb.RecordHit("abc1234")
	sb.RecordHit("abc1234")
	sb.RecordHit("xyz9999")

	first := sb.swap()
	if len(first) != 2 {
		t.Fatalf("first swap len = %d, want 2", len(first))
	}
	if first["abc1234"] != 2 {
		t.Errorf("abc1234 count = %d, want 2", first["abc1234"])
	}

	// live map must be fresh and empty immediately after swap — hits recorded
	// AFTER this point must not appear in `first`, and a second swap on an
	// untouched buffer must come back empty.
	second := sb.swap()
	if len(second) != 0 {
		t.Errorf("second swap len = %d, want 0 (live map should have been reset)", len(second))
	}
}

func TestStatsBuffer_HitsAfterSwapGoToNewMap(t *testing.T) {
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error { return nil },
		time.Minute,
	)

	sb.RecordHit("abc1234")
	old := sb.swap()

	sb.RecordHit("abc1234") // lands in the new map, not `old`

	if old["abc1234"] != 1 {
		t.Errorf("old map count = %d, want 1 (should be frozen at swap time)", old["abc1234"])
	}

	current := sb.swap()
	if current["abc1234"] != 1 {
		t.Errorf("current map count = %d, want 1", current["abc1234"])
	}
}

// --- flush behavior ---

func TestStatsBuffer_FlushNow_EmptyBufferSkipsFlushFunc(t *testing.T) {
	called := false
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			called = true
			return nil
		},
		time.Minute,
	)

	sb.flushNow(context.Background())

	if called {
		t.Error("FlushFunc was called on an empty buffer, expected it to be skipped")
	}
}

func TestStatsBuffer_FlushNow_CallsFlushFuncWithDeltas(t *testing.T) {
	var captured map[string]int64
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			captured = deltas
			return nil
		},
		time.Minute,
	)

	sb.RecordHit("abc1234")
	sb.RecordHit("abc1234")
	sb.RecordHit("xyz9999")

	sb.flushNow(context.Background())

	if captured["abc1234"] != 2 {
		t.Errorf("captured abc1234 = %d, want 2", captured["abc1234"])
	}
	if captured["xyz9999"] != 1 {
		t.Errorf("captured xyz9999 = %d, want 1", captured["xyz9999"])
	}
}

func TestStatsBuffer_FlushNow_ErrorDropsButDoesNotPanic(t *testing.T) {
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			return errors.New("mongo write failed")
		},
		time.Minute,
	)

	sb.RecordHit("abc1234")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("flushNow panicked on FlushFunc error: %v", r)
		}
	}()
	sb.flushNow(context.Background())

	// the dropped batch should not reappear on a later flush — buffer was
	// already swapped out before the FlushFunc error occurred, so the next
	// swap must be empty, not a retry of the failed batch.
	after := sb.swap()
	if len(after) != 0 {
		t.Errorf("buffer after failed flush = %v, want empty (dropped, not requeued)", after)
	}
}

// --- Start/Stop lifecycle ---

func TestStatsBuffer_Stop_PerformsFinalFlush(t *testing.T) {
	var captured map[string]int64
	var mu sync.Mutex
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			mu.Lock()
			captured = deltas
			mu.Unlock()
			return nil
		},
		time.Hour, // long enough that the ticker never fires during this test
	)

	sb.Start(context.Background())
	sb.RecordHit("abc1234")
	sb.RecordHit("abc1234")

	sb.Stop(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if captured["abc1234"] != 2 {
		t.Errorf("final flush captured abc1234 = %d, want 2", captured["abc1234"])
	}
}

func TestStatsBuffer_Stop_NoHitsRecordedSkipsFlushFunc(t *testing.T) {
	called := false
	var mu sync.Mutex
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			mu.Lock()
			called = true
			mu.Unlock()
			return nil
		},
		time.Hour,
	)

	sb.Start(context.Background())
	sb.Stop(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if called {
		t.Error("FlushFunc was called on Stop with no recorded hits, expected it to be skipped")
	}
}

func TestStatsBuffer_TickerFlushesPeriodically(t *testing.T) {
	flushed := make(chan map[string]int64, 1)
	sb := NewStatsBuffer(
		func(ctx context.Context, deltas map[string]int64) error {
			flushed <- deltas
			return nil
		},
		20*time.Millisecond,
	)

	sb.Start(context.Background())
	defer sb.Stop(context.Background())

	sb.RecordHit("abc1234")

	select {
	case deltas := <-flushed:
		if deltas["abc1234"] != 1 {
			t.Errorf("ticker flush captured abc1234 = %d, want 1", deltas["abc1234"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ticker did not flush within timeout")
	}
}
