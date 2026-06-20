// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// scenario.go drives the four §29 failure profiles. metrics.go and logs.go
// ask the active scenario "what value does this (entity, metric) take at
// time t" and scenario.go either returns a baseline sample or interpolates
// toward the profile's incident range using a trapezoid factor.

// Profile identifiers — keep aligned with /faults/activate?profile=<id>.
const (
	ProfileDiskSaturation = "vm_local_disk_saturation"
	ProfileCPUReady       = "vm_cpu_ready_contention"
	ProfileMemoryPressure = "vm_memory_pressure_swap"
	ProfileDatastoreInfra = "datastore_latency_shared_infra"
)

const (
	defaultDuration = 35 * time.Minute
	rampUp          = 2 * time.Minute
	rampDown        = 5 * time.Minute
)

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

// Mid is the arithmetic midpoint of the range.
func (r Range) Mid() float64 { return (r.Lo + r.Hi) / 2 }

// Entity selectors. The string form keeps Impact lookups simple; the
// `selectorXxx` helpers eliminate typos at call sites.
func selectorVM(name string) string      { return "vm:" + name }
func selectorPG(name string) string      { return "pg:" + name }
func selectorHost(id string) string      { return "host:" + id }
func selectorDatastore(id string) string { return "datastore:" + id }
func selectorTenant(id string) string    { return "tenant:" + id }

// ImpactSpec specifies how a single metric on a single entity behaves during
// a failure profile. OnsetOffsetMin pushes the ramp-up start back from
// profile activation; the metric stays at baseline until that offset elapses.
type ImpactSpec struct {
	IncidentRange  Range
	OnsetOffsetMin float64
}

// Profile is a named failure scenario.
type Profile struct {
	ID       string
	Duration time.Duration
	// Impacts is keyed [entity_selector][metric_key] → ImpactSpec.
	Impacts map[string]map[string]ImpactSpec
}

type activeProfile struct {
	profile *Profile
	started time.Time
}

// Scenario owns the currently-active profile (at most one). Reads via
// atomic pointer; writes serialized under mu so Activate/Clear race-safely.
type Scenario struct {
	active   atomic.Pointer[activeProfile]
	mu       sync.Mutex
	profiles map[string]*Profile
}

// NewScenario builds the registry of the four §29 profiles and returns a
// Scenario with no profile active.
func NewScenario() *Scenario {
	return &Scenario{profiles: buildProfileRegistry()}
}

// Activate switches the active profile to id. If a profile is already
// running it is replaced (single-active semantics). Returns the time the
// new profile started.
func (s *Scenario) Activate(id string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[id]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown profile %q", id)
	}
	now := time.Now()
	s.active.Store(&activeProfile{profile: p, started: now})
	return now, nil
}

// Clear stops the active profile (if any). Returns the previously-active
// profile ID, or "" if none.
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
func (s *Scenario) Status() (string, time.Time, time.Duration) {
	ap := s.active.Load()
	if ap == nil {
		return "", time.Time{}, 0
	}
	return ap.profile.ID, ap.started, time.Since(ap.started)
}

// ProfileIDs returns the registered profile IDs (stable order is not
// guaranteed; the HTTP handler sorts before display).
func (s *Scenario) ProfileIDs() []string {
	out := make([]string, 0, len(s.profiles))
	for k := range s.profiles {
		out = append(out, k)
	}
	return out
}

// trapezoidFactor returns the ramp factor at time t for a profile that
// started at start, lasts duration, with ramp-up rampUp and ramp-down
// rampDown. After `offset` is subtracted, returns 0 before t<0, linear
// 0..1 during ramp-up, 1 during plateau, linear 1..0 during ramp-down, and
// 0 after the end.
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

// RampedValue is the central API metrics.go calls per observation. It
// returns either a baseline sample or an interpolated value between baseline
// and the impacted incident range, depending on whether the active profile
// targets (selector, metricKey).
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
	factor := trapezoidFactor(t, ap.started, time.Duration(spec.OnsetOffsetMin*float64(time.Minute)), ap.profile.Duration)
	if factor <= 0 {
		return baseline.Sample(seed, t)
	}
	baseSample := baseline.Sample(seed, t)
	incidentSample := spec.IncidentRange.Sample(seed, t)
	return baseSample + (incidentSample-baseSample)*factor
}

// IsActiveOn reports whether the active profile targets (selector, metricKey)
// AND the ramp factor is non-zero. Used by logs.go to decide which event
// frequencies to use.
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
	return trapezoidFactor(t, ap.started, time.Duration(spec.OnsetOffsetMin*float64(time.Minute)), ap.profile.Duration) > 0
}

// ActiveProfileID returns the active profile's ID, or "" if none.
func (s *Scenario) ActiveProfileID() string {
	ap := s.active.Load()
	if ap == nil {
		return ""
	}
	return ap.profile.ID
}

// Lookup returns the requested profile if it exists.
func (s *Scenario) Lookup(id string) (*Profile, bool) {
	p, ok := s.profiles[id]
	return p, ok
}

// buildProfileRegistry constructs the four §29 failure profiles with their
// per-entity, per-metric incident ranges and onset offsets per spec §28.2.
func buildProfileRegistry() map[string]*Profile {
	const (
		bajajPGVM    = "vm-bajaj-pg-01"
		indigoPGVM   = "vm-indigo-pg-01"
		acmeBatchVM  = "vm-acme-batch-01"
		bajajPG      = "pg-bajaj-01"
		indigoPG     = "pg-indigo-01"
		bajajTenant  = "tenant_bajaj_finance"
		indigoTenant = "tenant_indigo_ops"
		degradedHost = "host-1017"
		degradedDS   = "datastore-202"
	)

	diskSat := &Profile{
		ID:       ProfileDiskSaturation,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorVM(bajajPGVM): {
				MetricNodeIOWaitPct:        {IncidentRange: Range{35, 65}, OnsetOffsetMin: 0},
				MetricNodeCPUStealPct:      {IncidentRange: Range{3, 8}, OnsetOffsetMin: 0},
				MetricNodeDiskIONow:        {IncidentRange: Range{12, 48}, OnsetOffsetMin: 0},
				MetricVMwareVMDiskWriteLat: {IncidentRange: Range{120, 450}, OnsetOffsetMin: 0},
				MetricVMwareVMDiskReadLat:  {IncidentRange: Range{40, 160}, OnsetOffsetMin: 0},
				MetricVMwareVMCPUUsagePct:  {IncidentRange: Range{50, 78}, OnsetOffsetMin: 0},
			},
			selectorPG(bajajPG): {
				MetricPGQueryLatencyP95Ms:   {IncidentRange: Range{350, 1200}, OnsetOffsetMin: 5},
				MetricPGCheckpointWriteRate: {IncidentRange: Range{10, 80}, OnsetOffsetMin: 5},
				MetricPGCheckpointSyncRate:  {IncidentRange: Range{5, 30}, OnsetOffsetMin: 5},
				MetricPGNumBackends:         {IncidentRange: Range{120, 260}, OnsetOffsetMin: 5},
				MetricPGCacheHitRatio:       {IncidentRange: Range{0.90, 0.97}, OnsetOffsetMin: 5},
				MetricPGLocksCount:          {IncidentRange: Range{300, 1200}, OnsetOffsetMin: 5},
			},
			selectorTenant(bajajTenant): {
				MetricTenantSLOBurnRate:      {IncidentRange: Range{8, 18}, OnsetOffsetMin: 10},
				MetricTenantSLOCompliance:    {IncidentRange: Range{0.960, 0.985}, OnsetOffsetMin: 10},
				MetricTenantSLOErrorBudget:   {IncidentRange: Range{0.05, 0.20}, OnsetOffsetMin: 10},
				MetricAirtelProbeSuccessRate: {IncidentRange: Range{0.940, 0.985}, OnsetOffsetMin: 10},
				MetricAirtelProbeLatencyMs:   {IncidentRange: Range{300, 1200}, OnsetOffsetMin: 10},
			},
		},
	}

	cpuReady := &Profile{
		ID:       ProfileCPUReady,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorVM(bajajPGVM): {
				MetricVMwareVMCPUReadyMs:  {IncidentRange: Range{4000, 18000}, OnsetOffsetMin: 0},
				MetricVMwareVMCPUUsagePct: {IncidentRange: Range{40, 70}, OnsetOffsetMin: 0},
				MetricNodeLoad1:           {IncidentRange: Range{10, 30}, OnsetOffsetMin: 0},
				MetricNodeContextSwitches: {IncidentRange: Range{15000, 60000}, OnsetOffsetMin: 0},
				MetricNodeCPUStealPct:     {IncidentRange: Range{8, 22}, OnsetOffsetMin: 0},
			},
			selectorVM(acmeBatchVM): {
				MetricVMwareVMCPUUsagePct: {IncidentRange: Range{90, 99}, OnsetOffsetMin: 0},
			},
			selectorPG(bajajPG): {
				MetricPGQueryLatencyP95Ms: {IncidentRange: Range{220, 750}, OnsetOffsetMin: 4},
				MetricPGNumBackends:       {IncidentRange: Range{120, 220}, OnsetOffsetMin: 4},
			},
			selectorTenant(bajajTenant): {
				MetricTenantSLOBurnRate:      {IncidentRange: Range{6, 14}, OnsetOffsetMin: 9},
				MetricTenantSLOCompliance:    {IncidentRange: Range{0.970, 0.990}, OnsetOffsetMin: 9},
				MetricTenantSLOErrorBudget:   {IncidentRange: Range{0.15, 0.35}, OnsetOffsetMin: 9},
				MetricAirtelProbeSuccessRate: {IncidentRange: Range{0.970, 0.990}, OnsetOffsetMin: 9},
				MetricAirtelProbeLatencyMs:   {IncidentRange: Range{200, 700}, OnsetOffsetMin: 9},
			},
		},
	}

	memPressure := &Profile{
		ID:       ProfileMemoryPressure,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorVM(bajajPGVM): {
				MetricNodeMemAvailable:    {IncidentRange: Range{0.02, 0.07}, OnsetOffsetMin: 0},
				MetricNodePSwapInRate:     {IncidentRange: Range{200, 2500}, OnsetOffsetMin: 0},
				MetricNodePSwapOutRate:    {IncidentRange: Range{200, 2500}, OnsetOffsetMin: 0},
				MetricNodeMemPressureRate: {IncidentRange: Range{0.2, 2.0}, OnsetOffsetMin: 0},
			},
			selectorPG(bajajPG): {
				MetricPGCacheHitRatio:     {IncidentRange: Range{0.85, 0.93}, OnsetOffsetMin: 3},
				MetricPGQueryLatencyP95Ms: {IncidentRange: Range{180, 650}, OnsetOffsetMin: 3},
				MetricPGLocksCount:        {IncidentRange: Range{200, 800}, OnsetOffsetMin: 3},
			},
			selectorTenant(bajajTenant): {
				MetricTenantSLOBurnRate:      {IncidentRange: Range{5, 12}, OnsetOffsetMin: 8},
				MetricTenantSLOCompliance:    {IncidentRange: Range{0.970, 0.990}, OnsetOffsetMin: 8},
				MetricTenantSLOErrorBudget:   {IncidentRange: Range{0.10, 0.30}, OnsetOffsetMin: 8},
				MetricAirtelProbeSuccessRate: {IncidentRange: Range{0.970, 0.995}, OnsetOffsetMin: 8},
				MetricAirtelProbeLatencyMs:   {IncidentRange: Range{180, 550}, OnsetOffsetMin: 8},
			},
		},
	}

	// Datastore-latency shared-infra profile — full spec §22 chain. Temporal
	// ordering from spec §28.2: datastore→host→VM→Linux→Postgres→probe→SLO,
	// with the at-risk tenant joining the impact set ~12 min in at the
	// at-risk amplitude.
	dsInfra := &Profile{
		ID:       ProfileDatastoreInfra,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorDatastore(degradedDS): {
				MetricDSWriteLat:   {IncidentRange: Range{80, 260}, OnsetOffsetMin: 0},
				MetricDSReadLat:    {IncidentRange: Range{35, 140}, OnsetOffsetMin: 0},
				MetricDSQueueDepth: {IncidentRange: Range{80, 300}, OnsetOffsetMin: 0},
				MetricDSIOPSWrite:  {IncidentRange: Range{6000, 9000}, OnsetOffsetMin: 0},
			},
			selectorHost(degradedHost): {
				MetricVMwareHostDiskWriteLat: {IncidentRange: Range{80, 240}, OnsetOffsetMin: 2},
				MetricVMwareHostDiskReadLat:  {IncidentRange: Range{40, 160}, OnsetOffsetMin: 2},
				MetricVMwareHostCPUUsagePct:  {IncidentRange: Range{75, 95}, OnsetOffsetMin: 2},
			},
			selectorVM(bajajPGVM): {
				MetricVMwareVMDiskWriteLat: {IncidentRange: Range{80, 250}, OnsetOffsetMin: 3},
				MetricVMwareVMDiskReadLat:  {IncidentRange: Range{45, 180}, OnsetOffsetMin: 3},
				MetricVMwareVMCPUReadyMs:   {IncidentRange: Range{3000, 12000}, OnsetOffsetMin: 3},
				MetricNodeIOWaitPct:        {IncidentRange: Range{25, 55}, OnsetOffsetMin: 4},
				MetricNodeDiskIONow:        {IncidentRange: Range{8, 64}, OnsetOffsetMin: 4},
				MetricNodeCPUStealPct:      {IncidentRange: Range{8, 22}, OnsetOffsetMin: 4},
			},
			selectorPG(bajajPG): {
				MetricPGQueryLatencyP95Ms:   {IncidentRange: Range{250, 850}, OnsetOffsetMin: 7},
				MetricPGCheckpointWriteRate: {IncidentRange: Range{3, 12}, OnsetOffsetMin: 7},
				MetricPGCheckpointSyncRate:  {IncidentRange: Range{1, 6}, OnsetOffsetMin: 7},
				MetricPGNumBackends:         {IncidentRange: Range{120, 260}, OnsetOffsetMin: 7},
				MetricPGCacheHitRatio:       {IncidentRange: Range{0.90, 0.97}, OnsetOffsetMin: 7},
				MetricPGLocksCount:          {IncidentRange: Range{300, 1200}, OnsetOffsetMin: 7},
				MetricPGReplicationLag:      {IncidentRange: Range{5, 60}, OnsetOffsetMin: 7},
			},
			selectorTenant(bajajTenant): {
				MetricAirtelProbeSuccessRate: {IncidentRange: Range{0.940, 0.985}, OnsetOffsetMin: 10},
				MetricAirtelProbeLatencyMs:   {IncidentRange: Range{300, 1200}, OnsetOffsetMin: 10},
				MetricTenantSLOBurnRate:      {IncidentRange: Range{8, 18}, OnsetOffsetMin: 11},
				MetricTenantSLOCompliance:    {IncidentRange: Range{0.960, 0.985}, OnsetOffsetMin: 11},
				MetricTenantSLOErrorBudget:   {IncidentRange: Range{0.05, 0.20}, OnsetOffsetMin: 11},
			},
			// At-risk tenant on the same shared datastore + host. Lower amplitudes
			// per spec §22.4 incident_range_at_risk values.
			selectorVM(indigoPGVM): {
				MetricVMwareVMDiskWriteLat: {IncidentRange: Range{20, 90}, OnsetOffsetMin: 12},
				MetricVMwareVMDiskReadLat:  {IncidentRange: Range{15, 60}, OnsetOffsetMin: 12},
				MetricVMwareVMCPUReadyMs:   {IncidentRange: Range{1000, 3500}, OnsetOffsetMin: 12},
				MetricNodeIOWaitPct:        {IncidentRange: Range{8, 18}, OnsetOffsetMin: 13},
				MetricNodeCPUStealPct:      {IncidentRange: Range{3, 8}, OnsetOffsetMin: 13},
			},
			selectorPG(indigoPG): {
				MetricPGQueryLatencyP95Ms: {IncidentRange: Range{100, 190}, OnsetOffsetMin: 14},
				MetricPGNumBackends:       {IncidentRange: Range{80, 180}, OnsetOffsetMin: 14},
			},
			selectorTenant(indigoTenant): {
				MetricTenantSLOBurnRate:      {IncidentRange: Range{2, 5}, OnsetOffsetMin: 15},
				MetricTenantSLOCompliance:    {IncidentRange: Range{0.990, 0.997}, OnsetOffsetMin: 15},
				MetricTenantSLOErrorBudget:   {IncidentRange: Range{0.25, 0.45}, OnsetOffsetMin: 15},
				MetricAirtelProbeSuccessRate: {IncidentRange: Range{0.990, 0.997}, OnsetOffsetMin: 15},
				MetricAirtelProbeLatencyMs:   {IncidentRange: Range{120, 250}, OnsetOffsetMin: 15},
			},
		},
	}

	return map[string]*Profile{
		diskSat.ID:     diskSat,
		cpuReady.ID:    cpuReady,
		memPressure.ID: memPressure,
		dsInfra.ID:     dsInfra,
	}
}
