// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

// Package orderevents simulates an order-events stream processor
// (order.created consumer group) whose consumer lag spikes on a fixed,
// self-triggering cycle — no admin/controlplane knob required. The demo
// Home page's "Correlate Logs with Metrics" tile opens a 6-hour window and
// looks for a real metric anomaly with correlated error logs; a 3-hour
// cycle guarantees any 6-hour lookback always lands on at least one full
// spike-and-recover, usually two.
package orderevents

import (
	"math/rand"
	"os"
	"strconv"
	"time"
)

// cyclePeriod is how often the lag incident recurs. Deterministic and keyed
// off wall-clock time (not an activation timestamp) so every
// replica/restart stays in phase with no coordination. Each stage is
// independently overridable via env var (seconds) so a local run can watch
// a full spike-and-recover in under a minute instead of waiting 3 hours —
// production leaves these unset and gets the real cadence.
var (
	cyclePeriod = envSecondsOr("ORDER_EVENTS_CYCLE_SECONDS", 3*time.Hour)
	rampUp      = envSecondsOr("ORDER_EVENTS_RAMP_UP_SECONDS", 2*time.Minute)
	plateau     = envSecondsOr("ORDER_EVENTS_PLATEAU_SECONDS", 15*time.Minute)
	rampDown    = envSecondsOr("ORDER_EVENTS_RAMP_DOWN_SECONDS", 5*time.Minute)

	// incidentDuration is the total window, starting at the top of each
	// cycle, during which the lag is elevated above baseline.
	incidentDuration = rampUp + plateau + rampDown
)

func envSecondsOr(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return def
	}
	return time.Duration(secs) * time.Second
}

// Range is a [Lo, Hi] sample interval.
type Range struct {
	Lo, Hi float64
}

// Sample returns a deterministic value inside the range for the given seed
// and second-truncated time. Same (seed, t) → same value, so retries within
// a single scrape stay self-consistent.
func (r Range) Sample(seed uint64, t time.Time) float64 {
	rng := rand.New(rand.NewSource(int64(seed) ^ t.Unix()))
	return r.Lo + (r.Hi-r.Lo)*rng.Float64()
}

// phaseElapsed returns how far into the current cycle t falls, in
// [0, cyclePeriod).
func phaseElapsed(t time.Time) time.Duration {
	period := int64(cyclePeriod / time.Second)
	return time.Duration(t.Unix()%period) * time.Second
}

// rampFactor is the trapezoid ramp shape: 0 before the incident starts,
// linear 0..1 during ramp-up, 1 during plateau, linear 1..0 during
// ramp-down, 0 after the incident ends.
func rampFactor(elapsed time.Duration) float64 {
	switch {
	case elapsed < 0 || elapsed >= incidentDuration:
		return 0
	case elapsed < rampUp:
		return elapsed.Seconds() / rampUp.Seconds()
	case elapsed < rampUp+plateau:
		return 1
	default:
		remaining := incidentDuration - elapsed
		return remaining.Seconds() / rampDown.Seconds()
	}
}

// IsActive reports whether t falls inside an elevated-lag window.
func IsActive(t time.Time) bool {
	return rampFactor(phaseElapsed(t)) > 0
}

// RampedValue interpolates between baseline and incident ranges using the
// current cycle's ramp factor. Outside the incident window this is just a
// baseline sample.
func RampedValue(baseline, incident Range, seed uint64, t time.Time) float64 {
	factor := rampFactor(phaseElapsed(t))
	base := baseline.Sample(seed, t)
	if factor <= 0 {
		return base
	}
	inc := incident.Sample(seed, t)
	return base + (inc-base)*factor
}
