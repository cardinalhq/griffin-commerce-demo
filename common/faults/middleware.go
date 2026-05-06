// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// Middleware applies the active knob's request-time effects:
//   - global.cpu-burn-traffic: spin loop for LatencyMs
//   - global slow:             sleep LatencyMs
//   - global error:            short-circuit with StatusCode
//
// Service-targeted knobs (catalog.error, cart.error, payment.fail, etc.)
// are handled at site-specific hook points where the request has access
// to product_id / processor / carrier / cart_id, not in this generic
// middleware. We deliberately keep the middleware narrow so it never
// surprises a service whose Service field doesn't match.
func Middleware(c *Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			k := c.Active()
			if k == nil || k.Service != ServiceGlobal {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			switch k.Kind {
			case KindCPUBurn:
				burnCPU(time.Duration(k.LatencyMs) * time.Millisecond)
			case KindSlow:
				time.Sleep(time.Duration(k.LatencyMs) * time.Millisecond)
			case KindError:
				if rand.Float64() < probability(k) {
					code := k.StatusCode
					if code == 0 {
						code = http.StatusInternalServerError
					}
					Record(r.Context(), k, 0)
					http.Error(w, http.StatusText(code), code)
					return
				}
			}
			added := time.Since(start)
			Record(r.Context(), k, float64(added.Milliseconds()))
			next.ServeHTTP(w, r)
		})
	}
}

// burnCPU runs an un-optimizable arithmetic loop for d so the compiler
// can't elide it and the OS can't park the goroutine.
func burnCPU(d time.Duration) {
	if d <= 0 {
		return
	}
	deadline := time.Now().Add(d)
	x := rand.Float64()
	for time.Now().Before(deadline) {
		x = math.Sqrt(x + 1.0)
	}
	// Force a side-effect so the compiler can't optimize the loop away.
	if math.IsNaN(x) {
		panic("unreachable: cpu burn produced NaN")
	}
}

// probability returns Probability with 0 treated as 1.0 (fault always fires
// when no probability is set — the simplest demo default).
func probability(k *Knob) float64 {
	if k.Probability <= 0 {
		return 1.0
	}
	return k.Probability
}
