// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"hash/fnv"
	"sync"
	"time"
)

// state.go owns the per-entity mutable counters, deterministic seeds, and
// dt-tracking that every observation in metrics.go reads. Each state struct
// is locked under its own mutex; the metric callback locks only the
// entities it observes during the scrape so writers don't serialize across
// the whole catalog.

func seedFor(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// dtAdvance updates lastTick and returns dt in seconds, clamped to at least
// 1ms to avoid counter-rate divide-by-zero on the first tick.
func dtAdvance(prev *time.Time, now time.Time) float64 {
	dt := now.Sub(*prev).Seconds()
	*prev = now
	if dt < 0.001 {
		return 0.001
	}
	return dt
}

type tenantState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
}

func newTenantState(t *Tenant) *tenantState {
	return &tenantState{seed: seedFor(t.TenantID), lastTick: time.Now()}
}

type hostState struct {
	mu                               sync.Mutex
	seed                             uint64
	lastTick                         time.Time
	cumNetDroppedRx, cumNetDroppedTx float64
}

func newHostState(h *Host) *hostState {
	return &hostState{seed: seedFor(h.ID), lastTick: time.Now()}
}

type datastoreState struct {
	mu       sync.Mutex
	seed     uint64
	lastTick time.Time
}

func newDatastoreState(d *Datastore) *datastoreState {
	return &datastoreState{seed: seedFor(d.ID), lastTick: time.Now()}
}

type vmState struct {
	mu                                   sync.Mutex
	seed                                 uint64
	lastTick                             time.Time
	cumCPUSeconds                        map[string]float64
	cumDiskRead, cumDiskWrite, cumDiskIO float64
	cumNetRxDrop, cumNetTxDrop           float64
	cumVMwareNetRx, cumVMwareNetTx       float64
	cumPSwapIn, cumPSwapOut              float64
	cumMemPressure                       float64
	cumContextSwitches                   float64
}

func newVMState(v *VM) *vmState {
	return &vmState{
		seed:          seedFor(v.UUID),
		lastTick:      time.Now(),
		cumCPUSeconds: map[string]float64{},
	}
}

type pgState struct {
	mu                                     sync.Mutex
	seed                                   uint64
	lastTick                               time.Time
	cumXactCommit, cumXactRollback         float64
	cumTupFetched, cumBlksRead, cumBlksHit float64
	cumCheckpointWrite, cumCheckpointSync  float64
	cumWalBytes                            float64
	cumLatencyBuckets                      map[string]map[string]float64
	cumLatencyCount, cumLatencySum         map[string]float64
}

func newPGState(p *PGInstance) *pgState {
	return &pgState{
		seed:              seedFor(p.Name),
		lastTick:          time.Now(),
		cumLatencyBuckets: map[string]map[string]float64{},
		cumLatencyCount:   map[string]float64{},
		cumLatencySum:     map[string]float64{},
	}
}
