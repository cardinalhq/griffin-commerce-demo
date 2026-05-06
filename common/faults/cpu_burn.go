// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

// CPUBurnController spawns NumCPU() background goroutines running an
// un-optimizable spin loop while the global.cpu-burn-bg knob is active.
//
// Duty cycle is controlled by Knob.LatencyMs:
//   - LatencyMs == 0  → continuous spin (full saturation per core)
//   - LatencyMs > 0   → spin LatencyMs ms then sleep LatencyMs ms (50% duty)
//
// Each polling client owns one controller instance; Start/Stop are
// idempotent so duplicate transitions don't double-spawn or leak goroutines.
type CPUBurnController struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCPUBurnController returns an idle controller. Call Start to spawn the
// burn goroutines and Stop to halt them.
func NewCPUBurnController() *CPUBurnController {
	return &CPUBurnController{}
}

// Start spawns NumCPU() burn goroutines. Idempotent: a second call while
// already running does nothing.
func (c *CPUBurnController) Start(ctx context.Context, k *Knob) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return // already running
	}
	burnCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	dutyMs := 0
	if k != nil {
		dutyMs = k.LatencyMs
	}
	n := runtime.NumCPU()
	slog.InfoContext(ctx, "faults: starting cpu-burn-bg goroutines",
		"num_goroutines", n, "duty_ms", dutyMs)

	c.wg.Add(n)
	for i := 0; i < n; i++ {
		go c.burn(burnCtx, dutyMs)
	}
}

// Stop signals all burn goroutines and waits for them to exit. Idempotent.
func (c *CPUBurnController) Stop(ctx context.Context) {
	c.mu.Lock()
	if c.cancel == nil {
		c.mu.Unlock()
		return
	}
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()

	cancel()
	c.wg.Wait()
	slog.InfoContext(ctx, "faults: cpu-burn-bg goroutines stopped")
}

func (c *CPUBurnController) burn(ctx context.Context, dutyMs int) {
	defer c.wg.Done()

	x := rand.Float64()
	if dutyMs <= 0 {
		// Full saturation: spin until cancelled, periodically checking ctx.
		const checkEvery = 1 << 16
		i := 0
		for {
			x = math.Sqrt(x + 1.0)
			i++
			if i&(checkEvery-1) == 0 {
				select {
				case <-ctx.Done():
					if math.IsNaN(x) {
						panic("unreachable: cpu burn produced NaN")
					}
					return
				default:
				}
			}
		}
	}

	// Duty-cycled: spin dutyMs then sleep dutyMs.
	dur := time.Duration(dutyMs) * time.Millisecond
	for {
		deadline := time.Now().Add(dur)
		for time.Now().Before(deadline) {
			x = math.Sqrt(x + 1.0)
		}
		select {
		case <-ctx.Done():
			if math.IsNaN(x) {
				panic("unreachable: cpu burn produced NaN")
			}
			return
		case <-time.After(dur):
		}
	}
}
