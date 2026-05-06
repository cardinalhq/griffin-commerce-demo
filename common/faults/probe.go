// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package faults

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

// Probe checks whether a knob with the given key is active and (for
// probability-gated kinds) whether the dice came up. Returns the active
// Knob and true when the fault should fire; nil and false otherwise.
//
// A knob with Probability ≤ 0 always fires when active — that's the
// reasonable default for a demo where the user just toggled it on and
// expects to see the effect.
func Probe(c *Client, key string) (*Knob, bool) {
	if c == nil {
		return nil, false
	}
	k := c.Active()
	if k == nil || k.Key != key {
		return nil, false
	}
	if k.Probability > 0 && rand.Float64() >= k.Probability {
		return nil, false
	}
	return k, true
}

// SlowMiddleware sleeps for k.LatencyMs whenever the active knob matches
// knobKey. Used by services whose only fault behavior is "slow every
// request" — catalog.slow, images.slow. cart.outlier is similar but lives
// at the handler level so per-operation metrics still get emitted.
//
// Returns nil-op middleware when the knob isn't active.
func SlowMiddleware(c *Client, knobKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if k, fired := Probe(c, knobKey); fired && k.LatencyMs > 0 {
				time.Sleep(time.Duration(k.LatencyMs) * time.Millisecond)
				Record(r.Context(), k, float64(k.LatencyMs))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaybeOutlier sleeps for k.LatencyMs if the active knob matches knobKey
// and the per-request dice come up. Distinct from SlowMiddleware in that
// caller is responsible for the operation-level metric — outlier latency
// must land on the per-operation duration histogram, which only the call
// site knows the labels for.
func MaybeOutlier(ctx context.Context, c *Client, knobKey string) time.Duration {
	k, fired := Probe(c, knobKey)
	if !fired {
		return 0
	}
	d := time.Duration(k.LatencyMs) * time.Millisecond
	if d <= 0 {
		return 0
	}
	time.Sleep(d)
	Record(ctx, k, float64(k.LatencyMs))
	return d
}
