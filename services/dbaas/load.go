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
// at the given time. For most instances this hovers around 30-70% per the
// instance's stable baseline. hdfc-prod-03 is the scenario victim and is
// fully knob-driven (no process-start clock dependence) — see below.
func storageRamp(st *InstanceState, t time.Time) float64 {
	if st.Inst.DBID == "hdfc-prod-03" {
		return hdfcProd03Ratio(st, t)
	}

	// All other instances: deterministic per-instance baseline + slow
	// oscillation. Stable in time so no instance crosses warning unless
	// we explicitly target it via a scenario.
	seed := hashSeed(st.Inst.DBID)
	baseline := 0.30 + float64(seed%40)/100.0 // 0.30 – 0.70 per instance
	radians := float64(t.UTC().Unix()%21600) / 21600.0 * 2 * math.Pi
	return baseline + 0.03*math.Sin(radians)
}

// hdfcProd03 storage ratio states. Fully knob-driven so the dashboard
// state is reproducible from demo run to demo run regardless of pod uptime.
const (
	// idleRatio is the "leading-indicator alert was already firing for
	// weeks and nobody acked" steady state. Sits in the SLO page band so
	// the Storage Headroom (worst) tile reads red on dashboard open.
	hdfcIdleRatio = 0.93
	// peakRatio is the disk-full target the ramp climbs to once the knob
	// activates. Past this point write queries return SQLSTATE 53100.
	hdfcPeakRatio = 0.97
	// rampDuration is how long it takes to climb idle → peak. Tuned for
	// live-demo pacing — short enough to fit in a talk, long enough to
	// narrate the trajectory.
	hdfcRampDuration = 60 * time.Second
	// expandedRatio is where the volume sits after the operator "extends"
	// the volume (i.e., clears the knob). Visibly green so the demo's
	// resolution moment is unambiguous.
	hdfcExpandedRatio = 0.60
)

// hdfcProd03Ratio returns the storage utilization fraction for the demo
// victim instance based on knob state. Three states:
//
//  1. Idle (knob never activated this run, or simulator just started):
//     hdfcIdleRatio — already in page-alert territory, the dashboard tile
//     reads red so the "leading indicator was firing" narrative is baked
//     into the steady state.
//  2. Ramping (knob is active, elapsed < rampDuration since activation):
//     linear from hdfcIdleRatio → hdfcPeakRatio over rampDuration.
//     Activation time comes from the controlplane's Knob.StartedAt so pod
//     restarts mid-scenario resume the ramp from the correct offset.
//  3. Peak (knob is active, elapsed ≥ rampDuration): hdfcPeakRatio.
//     Write queries fail past this point.
//  4. Post-expansion (knob was active and is now cleared): hdfcExpandedRatio.
//     Sticks until the knob is reactivated (or the pod restarts).
func hdfcProd03Ratio(st *InstanceState, t time.Time) float64 {
	if activated := st.diskFullActivatedAt.Load(); activated != nil {
		elapsed := t.Sub(*activated)
		if elapsed < 0 {
			elapsed = 0
		}
		frac := float64(elapsed) / float64(hdfcRampDuration)
		if frac > 1 {
			frac = 1
		}
		return hdfcIdleRatio + (hdfcPeakRatio-hdfcIdleRatio)*frac
	}
	if st.postExpansion.Load() {
		return hdfcExpandedRatio
	}
	return hdfcIdleRatio
}

// load returns the composed multiplier for a typical workload signal at t.
// Result is unbounded but typically in [0.3, 2.0].
func load(t time.Time, seed uint64) float64 {
	return diurnal(t) * weekly(t) * jitter(t, seed)
}
