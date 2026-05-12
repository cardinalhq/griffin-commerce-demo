// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"math"
	"time"
)

// Load model: value(t) = baseline * diurnal(t) * weekly(t) * jitter().
// Functions here are pure — no instance state. Per-instance state lives in
// state.go; this file only computes shape from wall clock.

// diurnal returns a multiplier on [0.4, 1.6] over a 24h cycle peaking
// around 14:00 IST. Sine wave with a fixed phase.
func diurnal(t time.Time) float64 {
	// Asia/Kolkata is UTC+5:30. Operate in that zone so the peak is local-IST.
	hour := float64((t.UTC().Unix()+(5*3600+1800))%86400) / 3600.0
	// Peak at hour 14, trough at hour 02 → use cos centered on 14h.
	radians := (hour - 14) / 24.0 * 2 * math.Pi
	return 1.0 + 0.6*math.Cos(radians)
}

// weekly returns a multiplier on [0.8, 1.05]: weekend dip for enterprise
// workload shapes. Saturday and Sunday return ~0.8, weekdays ~1.0.
func weekly(t time.Time) float64 {
	switch t.UTC().Weekday() {
	case time.Saturday, time.Sunday:
		return 0.82
	default:
		return 1.0
	}
}

// jitter is a deterministic-ish bumpy multiplier on [0.92, 1.08] driven by
// (t, seed). Same seed + same minute → same jitter, so the same instance
// gets a smooth-looking curve across collection cycles within a minute.
func jitter(t time.Time, seed uint64) float64 {
	minute := t.UTC().Unix() / 60
	// Mix the seed and minute into a pseudo-uniform [-1,1] number. Cheap,
	// deterministic, no RNG state.
	x := (seed*2654435761 + uint64(minute)*1442695040888963407) >> 32
	u := float64(x%10000) / 10000.0 // [0,1)
	return 1.0 + (u*2-1)*0.08
}

// storageRamp returns the fraction-of-capacity used for the given instance
// at the given time. For most instances this hovers around 30-60% with
// slow drift. For hdfc-prod-03 specifically, the value is shaped to be
// approaching 97% at "demo time" so the operator dashboard shows the
// linear-growth narrative.
//
// The simulator can't fabricate 30 days of OTLP history, but the function
// is parameterized so that after the simulator has been running for some
// time, the recorded 30d series shows a rising curve. For demo rehearsals
// we run the simulator for at least an hour before showtime so the live
// dashboard shows the climb.
func storageRamp(st *InstanceState, t time.Time) float64 {
	const (
		// hdfcWarningHorizon controls how fast the demo scenario "matures."
		// 90 minutes from simulator start → the ramp crosses 90% (page).
		// 120 minutes → 97% (writes start to fail when knob is active).
		hdfcWarningHorizon = 90 * time.Minute
		hdfcCrisisHorizon  = 120 * time.Minute
	)

	if st.Inst.DBID == "hdfc-prod-03" {
		elapsed := t.Sub(st.startedAt)
		if elapsed < 0 {
			elapsed = 0
		}
		// Linear from 0.68 at t=0 to 0.97 at t=hdfcCrisisHorizon.
		frac := float64(elapsed) / float64(hdfcCrisisHorizon)
		if frac > 1 {
			frac = 1
		}
		_ = hdfcWarningHorizon // documents the warning crossing point
		return 0.68 + 0.29*frac
	}

	// All other instances: a slow synthetic walk around a per-instance
	// baseline. Deterministic per-instance so the dashboard looks the same
	// on rerun, and stable in time so no instance ever crosses warning.
	seed := hashSeed(st.Inst.DBID)
	baseline := 0.30 + float64(seed%40)/100.0 // 0.30 – 0.70 per instance
	// Slow oscillation: ±3% over 6 hours.
	radians := float64(t.UTC().Unix()%21600) / 21600.0 * 2 * math.Pi
	return baseline + 0.03*math.Sin(radians)
}

// load returns the composed multiplier for a typical workload signal at t.
// Result is unbounded but typically in [0.3, 2.0].
func load(t time.Time, seed uint64) float64 {
	return diurnal(t) * weekly(t) * jitter(t, seed)
}
