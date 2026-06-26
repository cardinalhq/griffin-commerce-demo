// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package nvcf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCatalogShape(t *testing.T) {
	c := NewCatalog()
	assert.Len(t, c.Functions, 4, "expected 4 seeded functions")
	assert.Len(t, c.Versions, 6, "expected 6 versions (2 each for chat/summarize, 1 each for fraud/embed)")
	assert.Len(t, c.Accounts, 4, "expected 4 accounts")
	assert.Len(t, c.Clusters, 2, "expected 2 clusters")
	// us-west-2-a: 2 nodes × 8 GPUs = 16; us-east-1-a: 2 nodes × 4 GPUs = 8; total 24.
	assert.Len(t, c.Instances, 24, "expected 24 instances across both clusters")
	assert.NotEmpty(t, c.InferenceServers, "expected at least one inference server")
}

func TestVersionByLabel(t *testing.T) {
	c := NewCatalog()
	chat := c.FunctionByName("chat-helpful")
	require.NotNil(t, chat)
	v1 := c.VersionByLabel(chat.FunctionID, "v1")
	v2 := c.VersionByLabel(chat.FunctionID, "v2")
	require.NotNil(t, v1)
	require.NotNil(t, v2)
	assert.NotEqual(t, v1.FunctionVersionID, v2.FunctionVersionID)
}

// TestProfileRegistryAllKnobs locks in the contract that all 12 knob IDs
// from docs/specs/nvcf.md are registered (M1 knob list says 11; we also
// keep quota.exhausted as a 12th since it's a real native NVCF capability).
func TestProfileRegistryAllKnobs(t *testing.T) {
	c := NewCatalog()
	sc := NewScenario(c)
	ids := sc.ProfileIDs()
	expected := []string{
		ProfileFunctionColdStartSpike,
		ProfileFunctionTTFTRegression,
		ProfileFunctionGPUOOMFlap,
		ProfileFunctionTokenRateCollapse,
		ProfileClusterThermalThrottle,
		ProfileClusterPartialOutage,
		ProfileRouterImbalance,
		ProfileTenantNoisyNeighbor,
		ProfileGatewayFDExhaustion,
		ProfileRegistryFetchFail,
		ProfileControlPlaneDispatchLag,
		ProfileQuotaExhausted,
	}
	assert.ElementsMatch(t, expected, ids)
}

// TestTTFTRegressionBendsTargetVersionOnly is the M1 knob acceptance:
// activating ttft-regression for summarize-doc must raise the TTFT value
// observed at selector(summarize-doc v2) and leave summarize-doc v1
// untouched.
func TestTTFTRegressionBendsTargetVersionOnly(t *testing.T) {
	c := NewCatalog()
	sc := NewScenario(c)
	summarize := c.FunctionByName("summarize-doc")
	require.NotNil(t, summarize)
	v1 := c.VersionByLabel(summarize.FunctionID, "v1")
	v2 := c.VersionByLabel(summarize.FunctionID, "v2")
	require.NotNil(t, v1)
	require.NotNil(t, v2)

	baseline := Range{Lo: summarize.BaseTTFTSec * 0.85, Hi: summarize.BaseTTFTSec * 1.15}

	// Before activation: both versions sample at baseline.
	now := time.Now()
	v1Before := sc.RampedValue(selectorFunctionVersion(v1.FunctionVersionID), MetricStargateTTFTSeconds, baseline, 1, now)
	v2Before := sc.RampedValue(selectorFunctionVersion(v2.FunctionVersionID), MetricStargateTTFTSeconds, baseline, 1, now)
	assert.InEpsilon(t, summarize.BaseTTFTSec, v1Before, 0.20, "v1 baseline within 20%")
	assert.InEpsilon(t, summarize.BaseTTFTSec, v2Before, 0.20, "v2 baseline within 20%")

	// Activate ttft-regression for summarize-doc.
	_, err := sc.Activate(ProfileFunctionTTFTRegression, map[string]string{"function": "summarize-doc"})
	require.NoError(t, err)

	// Push the started timestamp back so ramp is at plateau (factor=1).
	ap := sc.active.Load()
	require.NotNil(t, ap)
	pseudoNow := ap.started.Add(rampUp + 10*time.Second)

	v1After := sc.RampedValue(selectorFunctionVersion(v1.FunctionVersionID), MetricStargateTTFTSeconds, baseline, 1, pseudoNow)
	v2After := sc.RampedValue(selectorFunctionVersion(v2.FunctionVersionID), MetricStargateTTFTSeconds, baseline, 1, pseudoNow)

	// v1 unchanged (still at baseline range).
	assert.InEpsilon(t, summarize.BaseTTFTSec, v1After, 0.20, "v1 should stay at baseline")
	// v2 elevated: incident range is BaseTTFT × [2, 3]. At plateau factor=1,
	// the returned value should be at least 1.8× baseline.
	assert.Greater(t, v2After, summarize.BaseTTFTSec*1.8, "v2 should be elevated by ttft-regression")
}

func TestActivateUnknownProfile(t *testing.T) {
	c := NewCatalog()
	sc := NewScenario(c)
	_, err := sc.Activate("definitely.not.a.real.knob", nil)
	assert.Error(t, err)
}

func TestClearReturnsPrevious(t *testing.T) {
	c := NewCatalog()
	sc := NewScenario(c)
	_, err := sc.Activate(ProfileFunctionTTFTRegression, map[string]string{"function": "summarize-doc"})
	require.NoError(t, err)
	prev := sc.Clear()
	assert.Equal(t, ProfileFunctionTTFTRegression, prev)
	assert.Equal(t, "", sc.Clear(), "second clear is a no-op")
}
