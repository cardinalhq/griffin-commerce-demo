// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// StorageClass captures the contracted IOPS tier for a volume. Two enterprise
// tiers + one commodity, matches the plan.
type StorageClass struct {
	Name             string
	ProvisionedIOPS  float64
	CapacityBytes    float64
}

var (
	io2_25k = StorageClass{Name: "io2-25k", ProvisionedIOPS: 25_000, CapacityBytes: 1_000 * 1024 * 1024 * 1024} // 1 TB
	io2_50k = StorageClass{Name: "io2-50k", ProvisionedIOPS: 50_000, CapacityBytes: 2_000 * 1024 * 1024 * 1024} // 2 TB
	gp3     = StorageClass{Name: "gp3", ProvisionedIOPS: 3_000, CapacityBytes: 500 * 1024 * 1024 * 1024}        // 500 GB
)

// InstanceState carries the simulator state for one DB instance. Pure
// functions in load.go derive metric values from the instance plus the
// current wall clock; cumulative-counter state lives here under mu.
type InstanceState struct {
	Inst *DBInstance

	// Storage class + per-instance startup parameters.
	Storage StorageClass

	// Baselines parameterize the load model — they fan the fleet so two
	// instances don't draw identical curves. Seeded deterministically off
	// db_id so reruns of the simulator look the same.
	BaselineQPS              float64 // queries per second at midday
	BaselineWriteFraction    float64 // 0–1, share of QPS that's writes
	BaselineConnections      float64 // active conns at midday
	BaselineWalBytesPerSec   float64 // WAL emission baseline
	BaselineStorageGrowthBps float64 // bytes/sec of normal data growth

	// mu serializes cumulative counter updates from the OTel observable
	// callback. The callback is single-threaded today but the SDK reserves
	// the right to parallelize across instruments — cheaper to lock.
	mu sync.Mutex

	// Wall-clock anchor + last-tick timestamp for counter integration.
	startedAt time.Time
	lastTick  time.Time

	// Cumulative counter state. Maps are op / error_code / etc. labelled.
	cumQueries      map[string]int64 // by op
	cumQueryErrors  map[string]int64 // by error_code
	cumSlowQueries  int64
	cumTxCommitted  int64
	cumTxRolledBack int64
	cumDeadlocks    int64
	cumLockWaitSec  float64
	cumTempBytes    int64
	cumWalBytes     int64
	cumCheckpoint   map[string]int64 // by kind
	cumBgWriter     int64
	cumConnRejected map[string]int64 // by reason
	cumPoolExhaust  int64
	cumFailover     map[string]int64 // by result

	// Quantile-gauge sliding-window state — count + sum.
	cumQueryLatencyCount map[string]int64
	cumQueryLatencySum   map[string]float64
	cumStorageLatCount   map[string]int64
	cumStorageLatSum     map[string]float64
	cumCheckpointCount   int64
	cumCheckpointSum     float64
	cumAutovacuumCount   int64
	cumAutovacuumSum     float64

	// Scenario / fault wiring. Atomic so faults.Client callback can flip
	// without contending with the metric callback.
	diskFullActive atomic.Bool
}

// fleetState is the package-level slice of per-instance state, built once at
// startup. Metrics callbacks read from it; the faults client mutates the
// per-instance diskFullActive flag.
var fleetState []*InstanceState

// buildFleetState expands the fleet config into InstanceState slots with
// deterministic per-instance baselines. Deterministic so test/demo reruns
// match what was seen before.
func buildFleetState(now time.Time) []*InstanceState {
	insts := BuildInstances()
	out := make([]*InstanceState, 0, len(insts))
	for _, inst := range insts {
		seed := hashSeed(inst.DBID)
		st := &InstanceState{
			Inst:      inst,
			Storage:   pickStorageClass(inst, seed),
			startedAt: now,
			lastTick:  now,

			BaselineQPS:              jitterAround(seed, "qps", 120, 0.4),    // ~120 qps median
			BaselineWriteFraction:    clamp(jitterAround(seed, "wfrac", 0.30, 0.20), 0.05, 0.55),
			BaselineConnections:      jitterAround(seed, "conns", 28, 0.35),
			BaselineWalBytesPerSec:   jitterAround(seed, "wal", 512*1024, 0.5),
			BaselineStorageGrowthBps: jitterAround(seed, "grow", 8*1024, 0.5), // ~8 KB/s normal growth

			cumQueries:           map[string]int64{},
			cumQueryErrors:       map[string]int64{},
			cumCheckpoint:        map[string]int64{},
			cumConnRejected:      map[string]int64{},
			cumFailover:          map[string]int64{},
			cumQueryLatencyCount: map[string]int64{},
			cumQueryLatencySum:   map[string]float64{},
			cumStorageLatCount:   map[string]int64{},
			cumStorageLatSum:     map[string]float64{},
		}
		out = append(out, st)
	}
	return out
}

// pickStorageClass deterministically assigns a tier based on customer tier
// and a per-instance hash. Enterprise customers get io2-25k mostly with
// some io2-50k; business customers get gp3.
func pickStorageClass(inst *DBInstance, seed uint64) StorageClass {
	if inst.Tier == "business" {
		return gp3
	}
	// Roughly 80% io2-25k, 20% io2-50k.
	if seed%5 == 0 {
		return io2_50k
	}
	return io2_25k
}

// hashSeed turns a stable string into a deterministic seed for jitter.
func hashSeed(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// jitterAround returns base * (1 + delta) where delta is deterministic in
// [-spread, +spread]. Spreads the fleet around a baseline without RNG.
func jitterAround(seed uint64, salt string, base, spread float64) float64 {
	h := fnv.New64a()
	h.Write([]byte(salt))
	var b [8]byte
	for i := range b {
		b[i] = byte(seed >> (i * 8))
	}
	h.Write(b[:])
	x := float64(h.Sum64()%10000) / 10000.0 // [0,1)
	delta := (x*2 - 1) * spread             // [-spread, +spread]
	return base * (1 + delta)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
