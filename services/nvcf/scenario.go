// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package nvcf

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// scenario.go drives the 11 NVCF chaos knobs (docs/specs/nvcf.md
// §"Chaos knobs"). Mirrors services/dbaas/scenario.go: a single
// active Profile at a time, each Profile carries per-entity per-metric
// IncidentRanges with onset offsets, and RampedValue interpolates between
// baseline and incident via a trapezoid factor.
//
// For M1 only function.ttft-regression has impact specs wired in.
// Remaining profiles are stubs that activate cleanly (so the playbook
// + controlplane UI surface them) but don't yet bend any metric.

// --- Knob IDs ---
// Match the spec's knob_id field verbatim so dashboard panels and
// controlplane UI line up by string.
const (
	ProfileFunctionColdStartSpike    = "function.cold-start-spike"
	ProfileFunctionTTFTRegression    = "function.ttft-regression"
	ProfileFunctionGPUOOMFlap        = "function.gpu-oom-flap"
	ProfileFunctionTokenRateCollapse = "function.token-rate-collapse"
	ProfileClusterThermalThrottle    = "cluster.thermal-throttle"
	ProfileClusterPartialOutage      = "cluster.partial-outage"
	ProfileRouterImbalance           = "router.imbalance"
	ProfileTenantNoisyNeighbor       = "tenant.noisy-neighbor"
	ProfileGatewayFDExhaustion       = "gateway.fd-exhaustion"
	ProfileRegistryFetchFail         = "registry.fetch-fail"
	ProfileControlPlaneDispatchLag   = "control-plane.dispatch-lag"
	ProfileQuotaExhausted            = "quota.exhausted"
)

const (
	defaultDuration = 5 * time.Minute // knobs auto-clear after 5min per spec
	rampUp          = 30 * time.Second
	rampDown        = 60 * time.Second
)

// Range is a [Lo, Hi] sample interval.
type Range struct {
	Lo, Hi float64
}

// Sample returns a deterministic value inside the range for the given seed
// and second-truncated time. Mirrors dbaas.Range.Sample semantics.
func (r Range) Sample(seed uint64, t time.Time) float64 {
	rng := rand.New(rand.NewSource(int64(seed) ^ t.Unix()))
	return r.Lo + (r.Hi-r.Lo)*rng.Float64()
}

// Mid is the arithmetic midpoint of the range.
func (r Range) Mid() float64 { return (r.Lo + r.Hi) / 2 }

// --- Entity selectors ---
//
// Selectors address one entity for impact lookups. The string form keeps
// Impact lookups simple; selector functions eliminate typos at call sites.
//
// NB: cohort granularity differs per scenario. ttft-regression targets
// a function_version_id; noisy-neighbor targets an account_name;
// thermal-throttle targets a cluster.

func selectorFunctionVersion(fvid string) string { return "fvid:" + fvid }
func selectorFunction(fid string) string         { return "fid:" + fid }
func selectorAccount(name string) string         { return "acct:" + name }
func selectorCluster(name string) string         { return "cluster:" + name }
func selectorInstance(cluster, device string) string {
	return "inst:" + cluster + "/" + device
}

// selectorInferenceServer is reserved for router.imbalance (M2). Until then
// keeping it out of the public-but-unused surface keeps lint clean.

// --- Metric keys ---
//
// Match scenario impact specs to observation sites by string. Names track
// the NVCF native metric names from Table A of the spec for readability.

const (
	MetricStargateTTFTSeconds         = "stargate_client_request_time_to_first_token_seconds"
	MetricStargateOutputTPS           = "stargate_client_model_output_tps"
	MetricStargateKVCacheUsed         = "stargate_client_model_kv_cache_used_tokens"
	MetricStargateKVCacheCapacity     = "stargate_client_model_kv_cache_capacity_tokens"
	MetricStargateRequestsInflight    = "stargate_client_requests_inflight"
	MetricFunctionRequestTotal        = "function_request_total"
	MetricFunctionRequestLatency      = "function_request_latency"
	MetricFunctionQueueDepth          = "nvcf_function_queue_depth"
	MetricAutoscalerCurrentInstances  = "nvcf_autoscaler.scaling.current_instances"
	MetricAutoscalerDesiredInstances  = "nvcf_autoscaler.scaling.desired_instances"
	MetricNVCAContainerCrashTotal     = "nvca_container_crash_total"
	MetricDCGMGPUUtil                 = "DCGM_FI_DEV_GPU_UTIL"
	MetricDCGMSMActive                = "DCGM_FI_PROF_SM_ACTIVE"
	MetricDCGMPowerUsage              = "DCGM_FI_DEV_POWER_USAGE"
	MetricGRPCProxyActiveConnections  = "nvcf_grpc_proxy_service_active_connections_total"
	MetricGRPCProxySessionInitSeconds = "nvcf_grpc_proxy_service_session_init_seconds_total"
	MetricLLMRouterRequestsTotal      = "llm_request_router_requests_total"
	MetricLLMGatewayHTTPRequestsTotal = "llm_api_gateway_http_requests_total"
	MetricLLMGatewayRateLimitApplied  = "llm_api_gateway_rate_limit_events_applied_total"
	MetricNVCAImagePullIssueTotal     = "nvca_image_pull_issue_total"
)

// ImpactSpec specifies how a single metric on a single entity behaves
// during a profile. OnsetOffset pushes the ramp-up start back from
// profile activation; the metric stays at baseline until that offset
// elapses.
type ImpactSpec struct {
	IncidentRange Range
	OnsetOffset   time.Duration
}

// Profile is a named chaos knob.
type Profile struct {
	ID       string
	Duration time.Duration
	// Impacts is keyed [entity_selector][metric_key] → ImpactSpec.
	Impacts map[string]map[string]ImpactSpec
}

type activeProfile struct {
	profile *Profile
	started time.Time
	// args is the optional cohort selector passed at activate time
	// (e.g., function_name=summarize-doc).  Resolved against the catalog
	// at activate time so the profile's selector-map can be patched.
	args map[string]string
}

// Scenario owns the single active profile, exactly mirroring dbaas.
type Scenario struct {
	active   atomic.Pointer[activeProfile]
	mu       sync.Mutex
	profiles map[string]*Profile
	catalog  *Catalog
}

// NewScenario builds the 11 profiles. Activate(id, args) selects one;
// args are profile-specific (see resolveArgs).
func NewScenario(catalog *Catalog) *Scenario {
	return &Scenario{
		profiles: buildProfileRegistry(),
		catalog:  catalog,
	}
}

// Activate switches the active profile to id and stores the args map.
// Returns the activation time.
func (s *Scenario) Activate(id string, args map[string]string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[id]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown profile %q", id)
	}
	resolved, err := s.resolveArgs(id, p, args)
	if err != nil {
		return time.Time{}, err
	}
	now := time.Now()
	s.active.Store(&activeProfile{profile: resolved, started: now, args: args})
	return now, nil
}

// Clear stops the active profile. Returns the previous ID, or "".
func (s *Scenario) Clear() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.active.Swap(nil)
	if prev == nil {
		return ""
	}
	return prev.profile.ID
}

// Status returns the active profile ID, started-at, and elapsed duration.
func (s *Scenario) Status() (string, time.Time, time.Duration, map[string]string) {
	ap := s.active.Load()
	if ap == nil {
		return "", time.Time{}, 0, nil
	}
	return ap.profile.ID, ap.started, time.Since(ap.started), ap.args
}

// ProfileIDs returns all registered knob IDs.
func (s *Scenario) ProfileIDs() []string {
	out := make([]string, 0, len(s.profiles))
	for k := range s.profiles {
		out = append(out, k)
	}
	return out
}

// Lookup returns a profile by ID if it exists.
func (s *Scenario) Lookup(id string) (*Profile, bool) {
	p, ok := s.profiles[id]
	return p, ok
}

// resolveArgs takes the user-supplied args (e.g., {"function":"summarize-doc"})
// and patches the profile's selector map so the right cohort is targeted.
// Returns a copy of the profile with the resolved selector keys; never
// mutates the registry.
func (s *Scenario) resolveArgs(id string, p *Profile, args map[string]string) (*Profile, error) {
	switch id {
	case ProfileFunctionTTFTRegression:
		funcName := defaultArg(args, "function", "summarize-doc")
		fn := s.catalog.FunctionByName(funcName)
		if fn == nil {
			return nil, fmt.Errorf("unknown function %q (try chat-helpful, summarize-doc, fraud-detect, embed-text)", funcName)
		}
		// Target the v2 of the named function. If v2 doesn't exist (e.g.,
		// fraud-detect/embed-text are v1-only), this is a config error —
		// fall back to v1 with a warning baked into the impact key.
		target := s.catalog.VersionByLabel(fn.FunctionID, "v2")
		if target == nil {
			target = s.catalog.VersionByLabel(fn.FunctionID, "v1")
		}
		if target == nil {
			return nil, fmt.Errorf("function %q has no resolvable versions", funcName)
		}
		// 2.5× multiplier on TTFT baseline for the targeted version only.
		// Baseline TTFT comes from the Function spec; we encode the
		// incident range as a multiple here.
		incidentLo := fn.BaseTTFTSec * 2.0
		incidentHi := fn.BaseTTFTSec * 3.0
		patched := &Profile{
			ID:       p.ID,
			Duration: p.Duration,
			Impacts: map[string]map[string]ImpactSpec{
				selectorFunctionVersion(target.FunctionVersionID): {
					MetricStargateTTFTSeconds: {
						IncidentRange: Range{Lo: incidentLo, Hi: incidentHi},
						OnsetOffset:   0,
					},
				},
			},
		}
		return patched, nil
	}
	// For all other knobs in M1: return the profile unchanged (impacts
	// empty). The controlplane UI still surfaces them; the synth ignores
	// them. M2+ wire the actual impact specs.
	return p, nil
}

func defaultArg(args map[string]string, key, fallback string) string {
	if v, ok := args[key]; ok && v != "" {
		return v
	}
	return fallback
}

// trapezoidFactor mirrors dbaas: 0 before onset, linear ramp-up,
// plateau at 1, linear ramp-down, 0 after end.
func trapezoidFactor(t, start time.Time, offset, duration time.Duration) float64 {
	elapsed := t.Sub(start) - offset
	if elapsed <= 0 || elapsed >= duration {
		return 0
	}
	plateauEnd := duration - rampDown
	switch {
	case elapsed < rampUp:
		return elapsed.Seconds() / rampUp.Seconds()
	case elapsed < plateauEnd:
		return 1
	default:
		return 1 - (elapsed-plateauEnd).Seconds()/rampDown.Seconds()
	}
}

// RampedValue is the synth's central API. It returns baseline or
// baseline-blended-with-incident depending on whether the active profile
// targets (selector, metricKey) at time t.
func (s *Scenario) RampedValue(selector, metricKey string, baseline Range, seed uint64, t time.Time) float64 {
	ap := s.active.Load()
	if ap == nil {
		return baseline.Sample(seed, t)
	}
	mset, ok := ap.profile.Impacts[selector]
	if !ok {
		return baseline.Sample(seed, t)
	}
	spec, ok := mset[metricKey]
	if !ok {
		return baseline.Sample(seed, t)
	}
	factor := trapezoidFactor(t, ap.started, spec.OnsetOffset, ap.profile.Duration)
	if factor <= 0 {
		return baseline.Sample(seed, t)
	}
	baseSample := baseline.Sample(seed, t)
	incidentSample := spec.IncidentRange.Sample(seed, t)
	return baseSample + (incidentSample-baseSample)*factor
}

// IsActiveOn reports whether the active profile targets (selector, metricKey)
// and the ramp factor is non-zero.
func (s *Scenario) IsActiveOn(selector, metricKey string, t time.Time) bool {
	ap := s.active.Load()
	if ap == nil {
		return false
	}
	mset, ok := ap.profile.Impacts[selector]
	if !ok {
		return false
	}
	spec, ok := mset[metricKey]
	if !ok {
		return false
	}
	return trapezoidFactor(t, ap.started, spec.OnsetOffset, ap.profile.Duration) > 0
}

// ActiveProfileID returns "" when no profile is active.
func (s *Scenario) ActiveProfileID() string {
	ap := s.active.Load()
	if ap == nil {
		return ""
	}
	return ap.profile.ID
}

// buildProfileRegistry creates an entry for every knob id. M1 only
// ttft-regression has a resolveArgs implementation that fills Impacts;
// remaining profiles are present so /faults/profiles lists them, but
// their Impacts stay empty until M2/M3 wire them in.
func buildProfileRegistry() map[string]*Profile {
	ids := []string{
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
	out := make(map[string]*Profile, len(ids))
	for _, id := range ids {
		out[id] = &Profile{
			ID:       id,
			Duration: defaultDuration,
			Impacts:  map[string]map[string]ImpactSpec{},
		}
	}
	return out
}
