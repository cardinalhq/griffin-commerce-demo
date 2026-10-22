// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package solar

import (
	"hash/fnv"
	"sync"
	"time"
)

// state.go owns per-entity mutable counters, deterministic seeds, and
// dt-tracking that every observation in metrics.go reads.

func seedFor(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// dtAdvance updates lastTick and returns dt in seconds, clamped to ≥ 1ms.
func dtAdvance(prev *time.Time, now time.Time) float64 {
	dt := now.Sub(*prev).Seconds()
	*prev = now
	if dt < 0.001 {
		return 0.001
	}
	return dt
}

type offtakerState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
	cumActualMWh float64
}

func newOfftakerState(o *Offtaker) *offtakerState {
	return &offtakerState{seed: seedFor(o.OfftakerID), lastTick: time.Now()}
}

type transformerState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
}

func newTransformerState(t *MVTransformer) *transformerState {
	return &transformerState{seed: seedFor(t.ID), lastTick: time.Now()}
}

type blockState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
	cumEnergyMWh float64
}

func newBlockState(b *Block) *blockState {
	return &blockState{seed: seedFor(b.ID), lastTick: time.Now()}
}

type inverterStationState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
}

func newInverterStationState(s *InverterStation) *inverterStationState {
	return &inverterStationState{seed: seedFor(s.ID), lastTick: time.Now()}
}

type inverterState struct {
	mu               sync.Mutex
	seed             uint64
	lastTick         time.Time
	cumEnergyKWh     float64
	cumGridFaultCnt  float64
}

func newInverterState(i *Inverter) *inverterState {
	return &inverterState{seed: seedFor(i.ID), lastTick: time.Now()}
}

type trackerState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
	cumFault float64
}

func newTrackerState(t *Tracker) *trackerState {
	return &trackerState{seed: seedFor(t.ID), lastTick: time.Now()}
}

type metState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
}

func newMetStationState(m *MetStation) *metState {
	return &metState{seed: seedFor(m.ID), lastTick: time.Now()}
}

type substationState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
	cumExportMWh float64
}

func newSubstationState(s *Substation) *substationState {
	return &substationState{seed: seedFor(s.ID), lastTick: time.Now()}
}
