// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package recommendations

import (
	"context"
	crand "crypto/rand"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

// memLeakController grows a never-freed slice while the recs.memleak knob
// is active. Each leaked snapshot pads with a 1 MiB random blob so RSS and
// heap_alloc move visibly — without padding, the underlying productCache
// (~1 KiB total) is too small to leak detectably.
//
// The "leak" is reversible by design (Stop nils the slice + GC) so the
// demo can iterate. A real leak wouldn't recover; we document this trade.
type memLeakController struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup

	leaked []leakedSnapshot
}

type leakedSnapshot struct {
	products []common.Product
	blob     []byte // 1 MiB padding so the snapshot has measurable footprint
}

func newMemLeakController() *memLeakController {
	return &memLeakController{}
}

// Start spawns the leak goroutine. Idempotent.
func (m *memLeakController) Start(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return
	}
	leakCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	slog.InfoContext(ctx, "recs: starting memleak goroutine",
		"snapshot_size_bytes", 1<<20,
		"interval_ms", 100,
	)
	m.wg.Add(1)
	go m.leak(leakCtx)
}

// Stop signals the goroutine, waits for exit, then frees the leaked slice
// and forces a GC so RSS reclaims within ~10s. The plan calls this out as
// reversible by design.
func (m *memLeakController) Stop(ctx context.Context) {
	m.mu.Lock()
	if m.cancel == nil {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()

	cancel()
	m.wg.Wait()

	m.mu.Lock()
	leakedCount := len(m.leaked)
	m.leaked = nil
	m.mu.Unlock()

	runtime.GC()
	slog.InfoContext(ctx, "recs: memleak stopped, snapshots released",
		"snapshots_freed", leakedCount,
	)
}

func (m *memLeakController) leak(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// Capture a snapshot of the current cache. cheap; the cache is
		// small. The padding blob is what makes the leak visible.
		productCacheMutex.RLock()
		snap := make([]common.Product, len(productCache))
		copy(snap, productCache)
		productCacheMutex.RUnlock()

		blob := make([]byte, 1<<20)
		// Touch the first and last bytes so the OS maps the pages.
		_, _ = crand.Read(blob[:64])
		blob[len(blob)-1] = 0xff

		m.mu.Lock()
		if m.cancel == nil {
			// Stop raced with us; drop this allocation.
			m.mu.Unlock()
			return
		}
		m.leaked = append(m.leaked, leakedSnapshot{products: snap, blob: blob})
		m.mu.Unlock()
	}
}
