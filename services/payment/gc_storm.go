// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package payment

import (
	"context"
	crand "crypto/rand"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// gcStormController generates real GC pressure on the payment service.
//
// The fidelity insight from the plan: runtime.GC() alone produces no
// visible STW pause on a small idle heap because the collector has nothing
// to do. This controller pairs forced GC with steady allocation churn so
// each runtime.GC() call is a real STW pause.
//
// Two goroutines:
//
//  1. Churn — maintains a sliding window of 20 × 5 MiB byte slices
//     (~100 MiB working set). Every 50 ms it allocates a fresh 5 MiB
//     slice, writes a few bytes (so the OS actually maps the pages), and
//     overwrites the oldest slot. ~100 MiB/s allocation rate.
//
//  2. Trigger — calls runtime.GC() every Knob.LatencyMs (default 200 ms).
//     With churn running, each call is a real STW pause of multiple ms.
//
// Visibility: STW pauses surface in process.runtime.go.gc.pause_ns
// (already exported via iruntime.Start in common/telemetry.go) and as
// p99 jitter on payment.charge because handlers occasionally land inside
// a pause.
type gcStormController struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// window is the heap-resident working set. Held under mu for
	// inspection but written/read by the churn goroutine without locking
	// (the slice header is reassigned in place; concurrent reads are not
	// expected).
	window [][]byte
}

func newGCStormController() *gcStormController {
	return &gcStormController{}
}

// Start spawns the churn and trigger goroutines. Idempotent.
func (g *gcStormController) Start(ctx context.Context, intervalMs int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		return
	}
	if intervalMs <= 0 {
		intervalMs = 200
	}
	stormCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.window = make([][]byte, 20)

	slog.InfoContext(ctx, "payment: starting gc-storm",
		"interval_ms", intervalMs,
		"window_slots", len(g.window),
		"slot_bytes", 5<<20,
	)

	g.wg.Add(2)
	go g.churn(stormCtx)
	go g.trigger(stormCtx, intervalMs)
}

// Stop signals both goroutines, waits for exit, and frees the working
// set so RSS comes back down within a couple of GC cycles.
func (g *gcStormController) Stop(ctx context.Context) {
	g.mu.Lock()
	if g.cancel == nil {
		g.mu.Unlock()
		return
	}
	cancel := g.cancel
	g.cancel = nil
	g.mu.Unlock()

	cancel()
	g.wg.Wait()

	g.mu.Lock()
	g.window = nil
	g.mu.Unlock()
	runtime.GC()
	slog.InfoContext(ctx, "payment: gc-storm stopped, window released")
}

func (g *gcStormController) churn(ctx context.Context) {
	defer g.wg.Done()
	const slotBytes = 5 << 20 // 5 MiB
	idx := 0
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		buf := make([]byte, slotBytes)
		// Force the OS to actually map the pages by writing a few bytes
		// at random spots. crypto/rand is overkill but it's just every
		// 50ms, the overhead is negligible compared to the allocation.
		_, _ = crand.Read(buf[:64])
		buf[len(buf)-1] = byte(idx & 0xff)

		g.mu.Lock()
		if g.window != nil {
			g.window[idx%len(g.window)] = buf
		}
		g.mu.Unlock()
		idx++
	}
}

func (g *gcStormController) trigger(ctx context.Context, intervalMs int) {
	defer g.wg.Done()
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtime.GC()
		}
	}
}
