// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package solar

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
	ProfileMVTransformerWindingOverheat = "mv_transformer_winding_overheat"
	ProfileInverterCoolingFault         = "inverter_cooling_fault"
	ProfileTrackerStowMisalignment      = "tracker_stow_misalignment"
	ProfileStringPIDDegradation         = "string_pid_degradation"
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

func (r Range) Sample(seed uint64, t time.Time) float64 {
	rng := rand.New(rand.NewSource(int64(seed) ^ t.Unix()))
	return r.Lo + (r.Hi-r.Lo)*rng.Float64()
}

func (r Range) Mid() float64 { return (r.Lo + r.Hi) / 2 }

// Entity selectors. String form keeps Impact lookups simple.
func selectorBlock(id string) string       { return "block:" + id }
func selectorOfftaker(id string) string    { return "offtaker:" + id }
func selectorTransformer(id string) string { return "transformer:" + id }
func selectorStation(id string) string     { return "station:" + id }
func selectorInverter(id string) string    { return "inverter:" + id }
func selectorTracker(id string) string     { return "tracker:" + id }
func selectorMet(id string) string         { return "met:" + id }
func selectorCompound(id string) string    { return "compound:" + id }
func selectorSubstation(id string) string  { return "substation:" + id }

// ImpactSpec specifies how a single metric on a single entity behaves
// during a failure profile. OnsetOffsetMin pushes ramp-up back from
// profile activation.
type ImpactSpec struct {
	IncidentRange  Range
	OnsetOffsetMin float64
}

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

// Scenario owns the currently-active profile (at most one).
type Scenario struct {
	active   atomic.Pointer[activeProfile]
	mu       sync.Mutex
	profiles map[string]*Profile
}

func NewScenario() *Scenario {
	return &Scenario{profiles: buildProfileRegistry()}
}

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

func (s *Scenario) Clear() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.active.Swap(nil)
	if prev == nil {
		return ""
	}
	return prev.profile.ID
}

func (s *Scenario) Status() (string, time.Time, time.Duration) {
	ap := s.active.Load()
	if ap == nil {
		return "", time.Time{}, 0
	}
	return ap.profile.ID, ap.started, time.Since(ap.started)
}

func (s *Scenario) ProfileIDs() []string {
	out := make([]string, 0, len(s.profiles))
	for k := range s.profiles {
		out = append(out, k)
	}
	return out
}

// trapezoidFactor returns the ramp factor at time t.
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

// RampedValue is the central API metrics.go calls per observation.
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

// IsActiveOn reports whether the active profile targets (selector,
// metricKey) AND the ramp factor is non-zero.
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

func (s *Scenario) ActiveProfileID() string {
	ap := s.active.Load()
	if ap == nil {
		return ""
	}
	return ap.profile.ID
}

func (s *Scenario) Lookup(id string) (*Profile, bool) {
	p, ok := s.profiles[id]
	return p, ok
}

// buildProfileRegistry constructs the four §29 failure profiles.
func buildProfileRegistry() map[string]*Profile {
	primaryTrafo := mvTrafoT04A
	siblingTrafo := mvTrafoT04B
	primaryCompound := mvCompound04

	// ----- mv_transformer_winding_overheat (canonical) -----
	mvOverheat := &Profile{
		ID:       ProfileMVTransformerWindingOverheat,
		Duration: defaultDuration,
		Impacts:  map[string]map[string]ImpactSpec{},
	}
	mvOverheat.Impacts[selectorTransformer(primaryTrafo)] = map[string]ImpactSpec{
		MetricMVTrafoOilFlowLPM:    {IncidentRange: Range{12, 35}, OnsetOffsetMin: 0},
		MetricMVTrafoWindingTempC:  {IncidentRange: Range{105, 138}, OnsetOffsetMin: 0},
		MetricMVTrafoOilTempC:      {IncidentRange: Range{82, 110}, OnsetOffsetMin: 1},
		MetricMVTrafoRadiatorTempC: {IncidentRange: Range{72, 96}, OnsetOffsetMin: 2},
		MetricMVTrafoLoadKVA:       {IncidentRange: Range{30000, 52000}, OnsetOffsetMin: 2},
		MetricMVTrafoOilLevelPct:   {IncidentRange: Range{82, 94}, OnsetOffsetMin: 4},
	}
	mvOverheat.Impacts[selectorCompound(primaryCompound)] = map[string]ImpactSpec{
		MetricMVCompoundAmbientTempC: {IncidentRange: Range{46, 58}, OnsetOffsetMin: 5},
	}
	// Sibling transformer in same compound — at-risk.
	mvOverheat.Impacts[selectorTransformer(siblingTrafo)] = map[string]ImpactSpec{
		MetricMVTrafoWindingTempC:  {IncidentRange: Range{86, 102}, OnsetOffsetMin: 13},
		MetricMVTrafoOilTempC:      {IncidentRange: Range{68, 82}, OnsetOffsetMin: 13},
		MetricMVTrafoRadiatorTempC: {IncidentRange: Range{58, 74}, OnsetOffsetMin: 13},
	}
	// Blocks on primary transformer: block-04 + block-12.
	for _, b := range []string{"block-04", "block-12"} {
		mvOverheat.Impacts[selectorBlock(b)] = map[string]ImpactSpec{
			MetricBlockPerformanceRatio: {IncidentRange: Range{0.62, 0.78}, OnsetOffsetMin: 7},
			MetricBlockACPowerMW:        {IncidentRange: Range{18, 28}, OnsetOffsetMin: 7},
			MetricBlockAvailability:     {IncidentRange: Range{0.72, 0.92}, OnsetOffsetMin: 7},
		}
	}
	// Blocks on sibling transformer: block-06 + block-14 — at-risk amplitude.
	for _, b := range []string{"block-06", "block-14"} {
		mvOverheat.Impacts[selectorBlock(b)] = map[string]ImpactSpec{
			MetricBlockPerformanceRatio: {IncidentRange: Range{0.84, 0.92}, OnsetOffsetMin: 15},
			MetricBlockACPowerMW:        {IncidentRange: Range{36, 44}, OnsetOffsetMin: 15},
		}
	}
	// Every station + inverter on primary transformer — derate.
	for _, b := range []string{"block-04", "block-12"} {
		for s := 1; s <= 4; s++ {
			sid := stationID(b, s)
			mvOverheat.Impacts[selectorStation(sid)] = map[string]ImpactSpec{
				MetricInverterStationACPowerKW:  {IncidentRange: Range{4500, 7500}, OnsetOffsetMin: 4},
				MetricInverterStationACVoltageV: {IncidentRange: Range{558, 588}, OnsetOffsetMin: 4},
			}
			for i := 1; i <= 4; i++ {
				iid := inverterID(b, s, i)
				mvOverheat.Impacts[selectorInverter(iid)] = map[string]ImpactSpec{
					MetricInverterACPowerKW:     {IncidentRange: Range{1100, 1900}, OnsetOffsetMin: 5},
					MetricInverterDCPowerKW:     {IncidentRange: Range{1180, 2000}, OnsetOffsetMin: 5},
					MetricInverterIGBTTempC:     {IncidentRange: Range{82, 104}, OnsetOffsetMin: 5},
					MetricInverterDerateState:   {IncidentRange: Range{1, 1}, OnsetOffsetMin: 5},
					MetricInverterEfficiencyPct: {IncidentRange: Range{93.5, 96.0}, OnsetOffsetMin: 5},
				}
			}
		}
	}
	// Primary offtaker — SECI Phase III PPA breach.
	mvOverheat.Impacts[selectorOfftaker(offtakerSECI)] = map[string]ImpactSpec{
		MetricPPAActualDispatchMW:    {IncidentRange: Range{55, 70}, OnsetOffsetMin: 9},
		MetricPPADispatchDeviationPct: {IncidentRange: Range{12, 30}, OnsetOffsetMin: 11},
		MetricPPAComplianceRatio:     {IncidentRange: Range{0.78, 0.88}, OnsetOffsetMin: 11},
		MetricPPABurnRate:            {IncidentRange: Range{6, 18}, OnsetOffsetMin: 11},
		MetricPPARevenueAtRiskINRMin: {IncidentRange: Range{18000, 65000}, OnsetOffsetMin: 11},
	}
	// At-risk offtaker — GUVNL nudges up but stays inside compliance.
	mvOverheat.Impacts[selectorOfftaker(offtakerGUVNL)] = map[string]ImpactSpec{
		MetricPPADispatchDeviationPct: {IncidentRange: Range{3, 7}, OnsetOffsetMin: 16},
		MetricPPAComplianceRatio:      {IncidentRange: Range{0.93, 0.97}, OnsetOffsetMin: 16},
		MetricPPABurnRate:             {IncidentRange: Range{1.5, 3.5}, OnsetOffsetMin: 16},
	}
	mvOverheat.Impacts[selectorSubstation(substationID)] = map[string]ImpactSpec{
		MetricSubstationExportPowerMW: {IncidentRange: Range{210, 240}, OnsetOffsetMin: 9},
	}

	// ----- inverter_cooling_fault -----
	coolingTarget := "INV-08-02-03"
	coolingStation := "IS-08-02"
	invCooling := &Profile{
		ID:       ProfileInverterCoolingFault,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorInverter(coolingTarget): {
				MetricInverterCoolingFanRPM: {IncidentRange: Range{0, 800}, OnsetOffsetMin: 0},
				MetricInverterHeatsinkTempC: {IncidentRange: Range{82, 104}, OnsetOffsetMin: 0},
				MetricInverterIGBTTempC:     {IncidentRange: Range{88, 112}, OnsetOffsetMin: 1},
				MetricInverterInternalTempC: {IncidentRange: Range{74, 92}, OnsetOffsetMin: 1},
				MetricInverterACPowerKW:     {IncidentRange: Range{900, 1800}, OnsetOffsetMin: 4},
				MetricInverterDerateState:   {IncidentRange: Range{1, 2}, OnsetOffsetMin: 4},
				MetricInverterEfficiencyPct: {IncidentRange: Range{91, 95}, OnsetOffsetMin: 4},
			},
			selectorStation(coolingStation): {
				MetricInverterStationACPowerKW: {IncidentRange: Range{8500, 11000}, OnsetOffsetMin: 5},
			},
		},
	}

	// ----- tracker_stow_misalignment -----
	trkID := "TRK-12"
	trkBlock := "block-12"
	trkStow := &Profile{
		ID:       ProfileTrackerStowMisalignment,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorTracker(trkID): {
				MetricTrackerAngleDeg:       {IncidentRange: Range{12, 18}, OnsetOffsetMin: 0},
				MetricTrackerTargetAngleDeg: {IncidentRange: Range{42, 50}, OnsetOffsetMin: 0},
				MetricTrackerMotorCurrentA:  {IncidentRange: Range{8.5, 14}, OnsetOffsetMin: 0},
				MetricTrackerFaultCount:     {IncidentRange: Range{12, 30}, OnsetOffsetMin: 0},
			},
			selectorMet("MET-12-1"): {
				MetricMetIrradiancePOAWm2: {IncidentRange: Range{420, 620}, OnsetOffsetMin: 1},
				MetricMetSoilingLossPct:   {IncidentRange: Range{4.5, 9.0}, OnsetOffsetMin: 1},
			},
			selectorBlock(trkBlock): {
				MetricBlockPerformanceRatio: {IncidentRange: Range{0.66, 0.80}, OnsetOffsetMin: 3},
				MetricBlockACPowerMW:        {IncidentRange: Range{22, 32}, OnsetOffsetMin: 3},
			},
			selectorOfftaker(offtakerSECI): {
				MetricPPADispatchDeviationPct: {IncidentRange: Range{4, 10}, OnsetOffsetMin: 6},
				MetricPPABurnRate:             {IncidentRange: Range{2, 6}, OnsetOffsetMin: 6},
			},
		},
	}

	// ----- string_pid_degradation -----
	pidInverter := "INV-10-03-01"
	pidStation := "IS-10-03"
	pidBlock := "block-10"
	pid := &Profile{
		ID:       ProfileStringPIDDegradation,
		Duration: defaultDuration,
		Impacts: map[string]map[string]ImpactSpec{
			selectorInverter(pidInverter): {
				MetricInverterDCPowerKW:     {IncidentRange: Range{2100, 2500}, OnsetOffsetMin: 0},
				MetricInverterMPPTVoltageV:  {IncidentRange: Range{780, 830}, OnsetOffsetMin: 0},
				MetricInverterMPPTCurrentA:  {IncidentRange: Range{2.4, 3.1}, OnsetOffsetMin: 0},
				MetricInverterStringImbalancePct: {IncidentRange: Range{4.8, 12.0}, OnsetOffsetMin: 0},
				MetricInverterStringMinCurrentA:  {IncidentRange: Range{1.2, 2.4}, OnsetOffsetMin: 0},
				MetricInverterACPowerKW:     {IncidentRange: Range{2000, 2400}, OnsetOffsetMin: 3},
				MetricInverterDerateState:   {IncidentRange: Range{1, 1}, OnsetOffsetMin: 10},
			},
			selectorStation(pidStation): {
				MetricInverterStationACPowerKW: {IncidentRange: Range{10500, 11800}, OnsetOffsetMin: 4},
			},
			selectorBlock(pidBlock): {
				MetricBlockPerformanceRatio: {IncidentRange: Range{0.88, 0.94}, OnsetOffsetMin: 6},
			},
		},
	}

	return map[string]*Profile{
		mvOverheat.ID:  mvOverheat,
		invCooling.ID:  invCooling,
		trkStow.ID:     trkStow,
		pid.ID:         pid,
	}
}
