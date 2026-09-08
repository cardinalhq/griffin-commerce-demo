// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package orderevents

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRampFactor(t *testing.T) {
	tests := []struct {
		name    string
		elapsed time.Duration
		want    float64
	}{
		{"before cycle start", -time.Second, 0},
		{"cycle start", 0, 0},
		{"midway through ramp up", 1 * time.Minute, 0.5},
		{"end of ramp up", 2 * time.Minute, 1},
		{"middle of plateau", 9*time.Minute + 30*time.Second, 1},
		{"start of ramp down", 17 * time.Minute, 1},
		{"middle of ramp down", 19*time.Minute + 30*time.Second, 0.5},
		{"end of incident", 22 * time.Minute, 0},
		{"well after incident", 1 * time.Hour, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rampFactor(tc.elapsed)
			require.InDelta(t, tc.want, got, 0.01, "rampFactor mismatch")
		})
	}
}

func TestPhaseElapsedWraps(t *testing.T) {
	base := time.Unix(0, 0)
	require.Equal(t, time.Duration(0), phaseElapsed(base))
	require.Equal(t, time.Hour, phaseElapsed(base.Add(time.Hour)))
	// One full cycle later, phase wraps back to 0.
	require.Equal(t, time.Duration(0), phaseElapsed(base.Add(cyclePeriod)))
	// Halfway into the second cycle.
	require.Equal(t, 30*time.Minute, phaseElapsed(base.Add(cyclePeriod+30*time.Minute)))
}

func TestIsActiveOnlyDuringIncidentWindow(t *testing.T) {
	base := time.Unix(1_700_000_000-1_700_000_000%int64(cyclePeriod/time.Second), 0)
	require.True(t, IsActive(base.Add(time.Minute)))
	require.True(t, IsActive(base.Add(incidentDuration-time.Second)))
	require.False(t, IsActive(base.Add(incidentDuration+time.Minute)))
	require.False(t, IsActive(base.Add(cyclePeriod-time.Minute)))
}

func TestRampedValueBaselineOutsideIncident(t *testing.T) {
	base := time.Unix(1_700_000_000-1_700_000_000%int64(cyclePeriod/time.Second), 0)
	t2 := base.Add(cyclePeriod - time.Minute) // well outside the incident window
	v := RampedValue(Range{2, 6}, Range{180, 260}, 42, t2)
	require.GreaterOrEqual(t, v, 2.0)
	require.LessOrEqual(t, v, 6.0)
}

func TestRampedValueAtPlateauReachesIncidentRange(t *testing.T) {
	base := time.Unix(1_700_000_000-1_700_000_000%int64(cyclePeriod/time.Second), 0)
	t2 := base.Add(9 * time.Minute) // inside the plateau
	v := RampedValue(Range{2, 6}, Range{180, 260}, 42, t2)
	require.GreaterOrEqual(t, v, 180.0)
	require.LessOrEqual(t, v, 260.0)
}

func TestRangeSampleDeterministic(t *testing.T) {
	r := Range{10, 20}
	tm := time.Unix(1, 0)
	v1 := r.Sample(99, tm)
	v2 := r.Sample(99, tm)
	require.Equal(t, v1, v2)
	require.GreaterOrEqual(t, v1, 10.0)
	require.LessOrEqual(t, v1, 20.0)
}
