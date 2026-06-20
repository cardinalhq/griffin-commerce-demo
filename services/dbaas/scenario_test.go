// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrapezoidFactor(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	dur := 35 * time.Minute
	tests := []struct {
		name   string
		at     time.Duration
		offset time.Duration
		want   float64
	}{
		{"before activation", -time.Second, 0, 0},
		{"before offset", 30 * time.Second, time.Minute, 0},
		{"midway through ramp up", time.Minute, 0, 0.5},
		{"end of ramp up", 2 * time.Minute, 0, 1},
		{"middle of plateau", 15 * time.Minute, 0, 1},
		{"middle of ramp down", 32*time.Minute + 30*time.Second, 0, 0.5},
		{"after end", 36 * time.Minute, 0, 0},
		{"offset shifts ramp", 6 * time.Minute, 5 * time.Minute, 0.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := trapezoidFactor(start.Add(tc.at), start, tc.offset, dur)
			require.InDelta(t, tc.want, got, 0.01, "trapezoidFactor mismatch")
		})
	}
}

func TestScenarioBaselineWithoutProfile(t *testing.T) {
	sc := NewScenario()
	seed := uint64(42)
	tm := time.Now()
	v := sc.RampedValue(selectorPG("pg-bajaj-01"), MetricPGQueryLatencyP95Ms, Range{35, 90}, seed, tm)
	require.GreaterOrEqual(t, v, 35.0)
	require.LessOrEqual(t, v, 90.0)
}

func TestScenarioPrimaryAtPlateau(t *testing.T) {
	sc := NewScenario()
	_, err := sc.Activate(ProfileDatastoreInfra)
	require.NoError(t, err)
	// The active profile's start is "now"; rewind by 20m to push us into
	// plateau territory by hacking the activeProfile pointer through Status.
	ap := sc.active.Load()
	ap.started = time.Now().Add(-20 * time.Minute)
	sc.active.Store(ap)

	v := sc.RampedValue(selectorPG("pg-bajaj-01"), MetricPGQueryLatencyP95Ms, Range{35, 90}, 1, time.Now())
	require.GreaterOrEqual(t, v, 250.0)
	require.LessOrEqual(t, v, 850.0)
}

func TestScenarioActivateReplaces(t *testing.T) {
	sc := NewScenario()
	_, err := sc.Activate(ProfileDiskSaturation)
	require.NoError(t, err)
	require.Equal(t, ProfileDiskSaturation, sc.ActiveProfileID())
	_, err = sc.Activate(ProfileCPUReady)
	require.NoError(t, err)
	require.Equal(t, ProfileCPUReady, sc.ActiveProfileID())
}

func TestScenarioActivateUnknown(t *testing.T) {
	sc := NewScenario()
	_, err := sc.Activate("not_a_profile")
	require.Error(t, err)
}

func TestScenarioClear(t *testing.T) {
	sc := NewScenario()
	_, err := sc.Activate(ProfileDiskSaturation)
	require.NoError(t, err)
	prev := sc.Clear()
	require.Equal(t, ProfileDiskSaturation, prev)
	require.Equal(t, "", sc.ActiveProfileID())
	// Clear when none active.
	require.Equal(t, "", sc.Clear())
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

func TestProfilesRegistered(t *testing.T) {
	sc := NewScenario()
	ids := sc.ProfileIDs()
	require.Len(t, ids, 4)
	want := map[string]bool{
		ProfileDiskSaturation: true,
		ProfileCPUReady:       true,
		ProfileMemoryPressure: true,
		ProfileDatastoreInfra: true,
	}
	for _, id := range ids {
		require.True(t, want[id], "unexpected profile id %s", id)
	}
}
