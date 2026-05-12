// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"fmt"
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

	// Profile is the parent customer's workload shape. Used inside the
	// metric callback to set values that need to feel customer-specific
	// rather than per-instance-randomized (e.g. cache hit ratio).
	Profile CustomerProfile

	// Baselines parameterize the load model — they fan the fleet so two
	// instances don't draw identical curves. Seeded deterministically off
	// db_id so reruns of the simulator look the same. Customer profile
	// scales the medians so HDFC and Infosys don't aggregate to the same
	// total.
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
// deterministic per-instance baselines. Each customer's profile scales the
// per-instance baselines so the dashboards visibly differ across tenants
// (HDFC Bank's 32 instances aggregate to a different total than Infosys's
// 12 instances even before tenant-specific multipliers).
func buildFleetState(now time.Time) []*InstanceState {
	out := make([]*InstanceState, 0)
	for _, c := range Fleet {
		prefix := customerPrefix(c.ID)
		for i := 1; i <= c.DBs; i++ {
			dbid := fmt.Sprintf("%s-prod-%02d", prefix, i)
			inst := &DBInstance{
				CustomerID: c.ID,
				DBID:       dbid,
				Tier:       c.Tier,
				Role:       "primary",
				PgVersion:  "15.6",
				Up:         true,
			}
			seed := hashSeed(dbid)
			st := &InstanceState{
				Inst:      inst,
				Profile:   c.Profile,
				Storage:   pickStorageClass(c.Profile.StorageTier, seed),
				startedAt: now,
				lastTick:  now,

				// Customer profile scales the per-instance medians. Per-instance
				// jitter on top keeps individual db_ids distinguishable.
				BaselineQPS:              jitterAround(seed, "qps", 120*c.Profile.QPSScale, 0.4),
				BaselineWriteFraction:    clamp(jitterAround(seed, "wfrac", c.Profile.WriteFraction, 0.2), 0.05, 0.6),
				BaselineConnections:      jitterAround(seed, "conns", 28*c.Profile.QPSScale, 0.35),
				BaselineWalBytesPerSec:   jitterAround(seed, "wal", 512*1024*c.Profile.QPSScale*c.Profile.WriteFraction*3, 0.5),
				BaselineStorageGrowthBps: jitterAround(seed, "grow", 8*1024*c.Profile.QPSScale, 0.5),

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
	}
	return out
}

// pickStorageClass picks a tier for one instance. Customer's preferred tier
// gets 80% of the customer's volumes; 20% drift to an adjacent tier so the
// per-customer storage-class breakdown is varied (not a single bar).
func pickStorageClass(preferred string, seed uint64) StorageClass {
	deviation := seed%5 == 0 // ~20% deviate
	switch preferred {
	case "io2-50k":
		if deviation {
			return io2_25k
		}
		return io2_50k
	case "io2-25k":
		if deviation {
			return io2_50k
		}
		return io2_25k
	case "gp3":
		if deviation {
			return io2_25k
		}
		return gp3
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
