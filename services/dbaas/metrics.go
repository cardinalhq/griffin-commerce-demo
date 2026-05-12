// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// resourceAttrs are constants for this DBaaS plane. They ride as point
// attributes on every observation so lakerunner sees them as labels. (Could
// promote to OTel Resource once common.SetupTelemetry plumbs addlAttrs.)
var resourceAttrs = []attribute.KeyValue{
	attribute.String("cloud.provider", "airtel"),
	attribute.String("cloud.region", "in-mumbai-1"),
	attribute.String("cluster", "dbaas-mumbai-prod-01"),
	attribute.String("service.type", "dbaas"),
}

const instrumentScope = "github.com/cardinalhq/griffin-commerce-demo/services/dbaas"

// queryOps are the operation labels we fan db.queries_total / latency over.
var queryOps = []string{"select", "insert", "update", "delete", "ddl"}

// storageOps are the labels we fan db.storage.latency_seconds over.
var storageOps = []string{"read", "write"}

// pgErrorCodes used in non-fault baseline error emission. We add 53100
// dynamically when the disk-full knob is active.
var pgErrorCodes = []string{"40001", "23505", "08006"} // serialization, unique violation, conn lost

// instruments holds every registered metric instrument. Keeping them in one
// struct keeps the callback signature manageable.
type instruments struct {
	// Availability + connections (6)
	dbUp                metric.Float64ObservableGauge
	connectionsActive   metric.Float64ObservableGauge
	connectionsIdle     metric.Float64ObservableGauge
	connectionsMax      metric.Float64ObservableGauge
	connectionsRejected metric.Float64ObservableCounter
	connectionsWaiting  metric.Float64ObservableGauge

	// Pooler (4)
	poolerClientActive  metric.Float64ObservableGauge
	poolerClientWaiting metric.Float64ObservableGauge
	poolerServerActive  metric.Float64ObservableGauge
	poolerExhausted     metric.Float64ObservableCounter

	// Workload aggregates (7 logical / 9 instruments — query.duration is a quantile family)
	txCommitted        metric.Float64ObservableCounter
	txRolledBack       metric.Float64ObservableCounter
	queriesTotal       metric.Float64ObservableCounter
	queryErrors        metric.Float64ObservableCounter
	queryDuration      metric.Float64ObservableGauge
	queryDurationCount metric.Float64ObservableCounter
	queryDurationSum   metric.Float64ObservableCounter
	queriesSlow        metric.Float64ObservableCounter
	queriesLongRunning metric.Float64ObservableGauge

	// Locks (3)
	deadlocks   metric.Float64ObservableCounter
	lockWaitSec metric.Float64ObservableCounter
	lockWaiters metric.Float64ObservableGauge

	// Cache + temp (3)
	bufferHitRatio metric.Float64ObservableGauge
	indexHitRatio  metric.Float64ObservableGauge
	tempBytes      metric.Float64ObservableCounter

	// WAL + replication (4)
	walBytes           metric.Float64ObservableCounter
	walLagBytes        metric.Float64ObservableGauge
	walLagSeconds      metric.Float64ObservableGauge
	replicationSlotLag metric.Float64ObservableGauge

	// Maintenance (4 logical / 6 instruments)
	checkpointTotal         metric.Float64ObservableCounter
	checkpointDuration      metric.Float64ObservableGauge
	checkpointDurationCount metric.Float64ObservableCounter
	checkpointDurationSum   metric.Float64ObservableCounter
	bgwriterBuffersWritten  metric.Float64ObservableCounter
	autovacuumDuration      metric.Float64ObservableGauge
	autovacuumDurationCount metric.Float64ObservableCounter
	autovacuumDurationSum   metric.Float64ObservableCounter

	// Storage (7 logical / 9 instruments)
	storageUsed            metric.Float64ObservableGauge
	storageCapacity        metric.Float64ObservableGauge
	storageIOPSRead        metric.Float64ObservableGauge
	storageIOPSWrite       metric.Float64ObservableGauge
	storageIOPSProvisioned metric.Float64ObservableGauge
	storageLatency         metric.Float64ObservableGauge
	storageLatencyCount    metric.Float64ObservableCounter
	storageLatencySum      metric.Float64ObservableCounter
	volumeHealth           metric.Float64ObservableGauge

	// Host (2)
	hostCPU    metric.Float64ObservableGauge
	hostMemory metric.Float64ObservableGauge

	// Control plane (2)
	failoverEvents       metric.Float64ObservableCounter
	backupLastSuccessAge metric.Float64ObservableGauge
}

// RegisterMetrics builds all 42 metrics and a single callback that walks
// the fleet once per collection cycle.
func RegisterMetrics(ctx context.Context, states []*InstanceState) error {
	meter := otel.Meter(instrumentScope)
	ins := &instruments{}

	if err := registerAll(meter, ins); err != nil {
		return err
	}

	deps := allObservables(ins)
	_, err := meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			now := time.Now()
			for _, st := range states {
				observeInstance(observer, ins, st, now)
			}
			return nil
		},
		deps...,
	)
	if err != nil {
		return fmt.Errorf("register dbaas callback: %w", err)
	}

	slog.InfoContext(ctx, "DBaaS metrics registered",
		"instruments_logical", 42,
		"instances", len(states),
	)
	return nil
}

// allObservables returns every instrument as a metric.Observable so they
// can be passed to RegisterCallback as dependencies.
func allObservables(ins *instruments) []metric.Observable {
	return []metric.Observable{
		ins.dbUp, ins.connectionsActive, ins.connectionsIdle, ins.connectionsMax,
		ins.connectionsRejected, ins.connectionsWaiting,
		ins.poolerClientActive, ins.poolerClientWaiting, ins.poolerServerActive, ins.poolerExhausted,
		ins.txCommitted, ins.txRolledBack, ins.queriesTotal, ins.queryErrors,
		ins.queryDuration, ins.queryDurationCount, ins.queryDurationSum,
		ins.queriesSlow, ins.queriesLongRunning,
		ins.deadlocks, ins.lockWaitSec, ins.lockWaiters,
		ins.bufferHitRatio, ins.indexHitRatio, ins.tempBytes,
		ins.walBytes, ins.walLagBytes, ins.walLagSeconds, ins.replicationSlotLag,
		ins.checkpointTotal, ins.checkpointDuration, ins.checkpointDurationCount, ins.checkpointDurationSum,
		ins.bgwriterBuffersWritten,
		ins.autovacuumDuration, ins.autovacuumDurationCount, ins.autovacuumDurationSum,
		ins.storageUsed, ins.storageCapacity, ins.storageIOPSRead, ins.storageIOPSWrite,
		ins.storageIOPSProvisioned, ins.storageLatency, ins.storageLatencyCount, ins.storageLatencySum,
		ins.volumeHealth,
		ins.hostCPU, ins.hostMemory,
		ins.failoverEvents, ins.backupLastSuccessAge,
	}
}

// instanceAttrs is the base label set for one instance (resource + per-instance).
func instanceAttrs(st *InstanceState) []attribute.KeyValue {
	inst := st.Inst
	attrs := make([]attribute.KeyValue, 0, len(resourceAttrs)+7)
	attrs = append(attrs, resourceAttrs...)
	attrs = append(attrs,
		attribute.String("customer.id", inst.CustomerID),
		attribute.String("db_id", inst.DBID),
		attribute.String("customer.tier", inst.Tier),
		attribute.String("role", inst.Role),
		attribute.String("pg_version", inst.PgVersion),
		attribute.String("storage_class", st.Storage.Name),
		attribute.String("volume_id", inst.DBID+"-vol-a"),
	)
	return attrs
}

// observeInstance computes and emits every metric value for one instance at
// the given collection time. Counter state is integrated forward under the
// per-instance mutex.
func observeInstance(o metric.Observer, ins *instruments, st *InstanceState, now time.Time) {
	st.mu.Lock()
	dt := now.Sub(st.lastTick).Seconds()
	if dt < 0.001 {
		dt = 0.001 // first tick or clock skew; avoid zero
	}
	st.lastTick = now
	defer st.mu.Unlock()

	base := instanceAttrs(st)
	seed := hashSeed(st.Inst.DBID)
	loadMult := load(now, seed)
	diskFull := st.diskFullActive.Load()

	// --- Availability ---
	up := 1.0
	if !st.Inst.Up {
		up = 0
	}
	o.ObserveFloat64(ins.dbUp, up, metric.WithAttributes(base...))

	// --- Connections ---
	maxConn := 200.0
	active := st.BaselineConnections * loadMult
	if active > maxConn*0.95 {
		active = maxConn * 0.95
	}
	idle := math.Max(0, st.BaselineConnections*0.4-active*0.05)
	waiting := 0.0
	if diskFull {
		waiting = 12 + 10*loadMult // backed up at the pooler while writes error
	}
	o.ObserveFloat64(ins.connectionsActive, active, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.connectionsIdle, idle, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.connectionsMax, maxConn, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.connectionsWaiting, waiting, metric.WithAttributes(base...))

	// connections.rejected_total — small baseline, spike when disk-full
	for _, reason := range []string{"max_connections", "auth", "timeout"} {
		rate := 0.005 * loadMult // ~0.5% / sec baseline per reason
		if diskFull && reason == "timeout" {
			rate += 1.5 // timeouts from blocked writes
		}
		st.cumConnRejected[reason] += int64(rate * dt)
		o.ObserveFloat64(ins.connectionsRejected, float64(st.cumConnRejected[reason]),
			metric.WithAttributes(append(base, attribute.String("reason", reason))...))
	}

	// --- Pooler ---
	o.ObserveFloat64(ins.poolerClientActive, active*1.05, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.poolerClientWaiting, waiting, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.poolerServerActive, math.Min(active*0.9, 50), metric.WithAttributes(base...))
	if diskFull {
		st.cumPoolExhaust += int64(0.3 * dt)
	}
	o.ObserveFloat64(ins.poolerExhausted, float64(st.cumPoolExhaust), metric.WithAttributes(base...))

	// --- Workload aggregates ---
	totalQPS := st.BaselineQPS * loadMult
	writeQPS := totalQPS * st.BaselineWriteFraction
	readQPS := totalQPS - writeQPS

	// Per-op increments. Split writes across insert/update/delete proportionally.
	opSplits := map[string]float64{
		"select": readQPS,
		"insert": writeQPS * 0.55,
		"update": writeQPS * 0.35,
		"delete": writeQPS * 0.08,
		"ddl":    writeQPS * 0.02,
	}
	for _, op := range queryOps {
		st.cumQueries[op] += int64(opSplits[op] * dt)
		o.ObserveFloat64(ins.queriesTotal, float64(st.cumQueries[op]),
			metric.WithAttributes(append(base, attribute.String("op", op))...))
	}

	// Tx counts roughly follow QPS (one tx per ~5 queries).
	st.cumTxCommitted += int64(totalQPS / 5 * dt)
	st.cumTxRolledBack += int64(totalQPS / 5 * 0.005 * dt)
	if diskFull {
		st.cumTxRolledBack += int64(writeQPS * dt) // every write tx rolls back
	}
	o.ObserveFloat64(ins.txCommitted, float64(st.cumTxCommitted), metric.WithAttributes(base...))
	o.ObserveFloat64(ins.txRolledBack, float64(st.cumTxRolledBack), metric.WithAttributes(base...))

	// Query errors — baseline noise across the three normal codes.
	for _, code := range pgErrorCodes {
		st.cumQueryErrors[code] += int64(0.0005 * totalQPS * dt)
		o.ObserveFloat64(ins.queryErrors, float64(st.cumQueryErrors[code]),
			metric.WithAttributes(append(base, attribute.String("error_code", code))...))
	}
	// 53100 disk-full errors — only when the knob is active on this instance.
	if diskFull {
		st.cumQueryErrors["53100"] += int64(writeQPS * dt)
	}
	o.ObserveFloat64(ins.queryErrors, float64(st.cumQueryErrors["53100"]),
		metric.WithAttributes(append(base, attribute.String("error_code", "53100"))...))

	// Slow queries (>1s threshold) — small fraction of total.
	st.cumSlowQueries += int64(0.002 * totalQPS * dt)
	o.ObserveFloat64(ins.queriesSlow, float64(st.cumSlowQueries), metric.WithAttributes(base...))

	// Long-running queries (>60s, gauge) — usually 0; goes up when disk-full backs things up.
	longRunning := 0.0
	if diskFull {
		longRunning = 3
	}
	o.ObserveFloat64(ins.queriesLongRunning, longRunning, metric.WithAttributes(base...))

	// Query latency quantile family — by op.
	for _, op := range queryOps {
		// Normal: select ~5ms p50, ~25ms p95, ~80ms p99. Write ops higher.
		p50, p95, p99 := baselineQueryLatency(op)
		if diskFull && (op == "insert" || op == "update" || op == "delete") {
			p99 *= 12 // writes pile up before failing
			p95 *= 6
		}
		o.ObserveFloat64(ins.queryDuration, p50,
			metric.WithAttributes(append(base, attribute.String("op", op), attribute.String("quantile", "0.5"))...))
		o.ObserveFloat64(ins.queryDuration, p95,
			metric.WithAttributes(append(base, attribute.String("op", op), attribute.String("quantile", "0.95"))...))
		o.ObserveFloat64(ins.queryDuration, p99,
			metric.WithAttributes(append(base, attribute.String("op", op), attribute.String("quantile", "0.99"))...))
		st.cumQueryLatencyCount[op] += int64(opSplits[op] * dt)
		st.cumQueryLatencySum[op] += opSplits[op] * dt * p50
		o.ObserveFloat64(ins.queryDurationCount, float64(st.cumQueryLatencyCount[op]),
			metric.WithAttributes(append(base, attribute.String("op", op))...))
		o.ObserveFloat64(ins.queryDurationSum, st.cumQueryLatencySum[op],
			metric.WithAttributes(append(base, attribute.String("op", op))...))
	}

	// --- Locks ---
	st.cumDeadlocks += int64(0.001 * loadMult * dt)
	o.ObserveFloat64(ins.deadlocks, float64(st.cumDeadlocks), metric.WithAttributes(base...))
	for _, lockType := range []string{"row", "table", "advisory"} {
		st.cumLockWaitSec += 0.05 * loadMult * dt
		o.ObserveFloat64(ins.lockWaitSec, st.cumLockWaitSec,
			metric.WithAttributes(append(base, attribute.String("lock_type", lockType))...))
	}
	waiters := math.Floor(loadMult * 0.5)
	if diskFull {
		waiters += 40
	}
	o.ObserveFloat64(ins.lockWaiters, waiters, metric.WithAttributes(base...))

	// --- Cache + temp ---
	bufferHit := 0.97 - 0.005*math.Max(0, loadMult-1)
	if diskFull {
		bufferHit = 0.86
	}
	o.ObserveFloat64(ins.bufferHitRatio, bufferHit, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.indexHitRatio, math.Min(0.995, bufferHit+0.02), metric.WithAttributes(base...))
	st.cumTempBytes += int64(loadMult * 4096 * dt) // small temp file pressure
	o.ObserveFloat64(ins.tempBytes, float64(st.cumTempBytes), metric.WithAttributes(base...))

	// --- WAL + replication ---
	walRate := st.BaselineWalBytesPerSec * loadMult
	st.cumWalBytes += int64(walRate * dt)
	o.ObserveFloat64(ins.walBytes, float64(st.cumWalBytes), metric.WithAttributes(base...))
	// Replica lag — small under normal load, bigger when WAL bursts.
	for _, replica := range []string{"replica-01", "replica-02"} {
		lagBytes := walRate * 0.3
		lagSec := 0.5 + 2*math.Max(0, loadMult-1.2)
		o.ObserveFloat64(ins.walLagBytes, lagBytes,
			metric.WithAttributes(append(base, attribute.String("replica_id", replica))...))
		o.ObserveFloat64(ins.walLagSeconds, lagSec,
			metric.WithAttributes(append(base, attribute.String("replica_id", replica))...))
	}
	for _, slot := range []string{"slot-01"} {
		o.ObserveFloat64(ins.replicationSlotLag, walRate*0.4,
			metric.WithAttributes(append(base, attribute.String("slot", slot))...))
	}

	// --- Maintenance ---
	for _, kind := range []string{"timed", "requested"} {
		st.cumCheckpoint[kind] += int64(0.005 * dt) // ~1 per ~3min
		o.ObserveFloat64(ins.checkpointTotal, float64(st.cumCheckpoint[kind]),
			metric.WithAttributes(append(base, attribute.String("kind", kind))...))
	}
	cpP50, cpP95, cpP99 := 0.4, 1.2, 2.1
	o.ObserveFloat64(ins.checkpointDuration, cpP50, metric.WithAttributes(append(base, attribute.String("quantile", "0.5"))...))
	o.ObserveFloat64(ins.checkpointDuration, cpP95, metric.WithAttributes(append(base, attribute.String("quantile", "0.95"))...))
	o.ObserveFloat64(ins.checkpointDuration, cpP99, metric.WithAttributes(append(base, attribute.String("quantile", "0.99"))...))
	st.cumCheckpointCount += int64(0.01 * dt)
	st.cumCheckpointSum += 0.01 * dt * cpP50
	o.ObserveFloat64(ins.checkpointDurationCount, float64(st.cumCheckpointCount), metric.WithAttributes(base...))
	o.ObserveFloat64(ins.checkpointDurationSum, st.cumCheckpointSum, metric.WithAttributes(base...))

	st.cumBgWriter += int64(walRate / 8192 * dt) // bufferRate ≈ wal/8KB pages
	o.ObserveFloat64(ins.bgwriterBuffersWritten, float64(st.cumBgWriter), metric.WithAttributes(base...))

	avP50, avP95, avP99 := 12.0, 45.0, 120.0
	o.ObserveFloat64(ins.autovacuumDuration, avP50, metric.WithAttributes(append(base, attribute.String("quantile", "0.5"))...))
	o.ObserveFloat64(ins.autovacuumDuration, avP95, metric.WithAttributes(append(base, attribute.String("quantile", "0.95"))...))
	o.ObserveFloat64(ins.autovacuumDuration, avP99, metric.WithAttributes(append(base, attribute.String("quantile", "0.99"))...))
	st.cumAutovacuumCount += int64(0.002 * dt) // ~1 per ~8min
	st.cumAutovacuumSum += 0.002 * dt * avP50
	o.ObserveFloat64(ins.autovacuumDurationCount, float64(st.cumAutovacuumCount), metric.WithAttributes(base...))
	o.ObserveFloat64(ins.autovacuumDurationSum, st.cumAutovacuumSum, metric.WithAttributes(base...))

	// --- Storage ---
	frac := storageRamp(st, now)
	used := frac * st.Storage.CapacityBytes
	o.ObserveFloat64(ins.storageUsed, used, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.storageCapacity, st.Storage.CapacityBytes, metric.WithAttributes(base...))

	readIOPS := readQPS * 2 // each read ~2 IOPS at baseline
	writeIOPS := writeQPS * 4
	o.ObserveFloat64(ins.storageIOPSRead, readIOPS, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.storageIOPSWrite, writeIOPS, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.storageIOPSProvisioned, st.Storage.ProvisionedIOPS, metric.WithAttributes(base...))

	// Storage latency quantile family by op
	for _, op := range storageOps {
		p50, p95, p99 := 0.001, 0.004, 0.008 // 1ms / 4ms / 8ms baselines (seconds)
		if frac > 0.93 {                      // near-full volumes start getting slower
			p99 += 0.004
		}
		o.ObserveFloat64(ins.storageLatency, p50, metric.WithAttributes(append(base, attribute.String("op", op), attribute.String("quantile", "0.5"))...))
		o.ObserveFloat64(ins.storageLatency, p95, metric.WithAttributes(append(base, attribute.String("op", op), attribute.String("quantile", "0.95"))...))
		o.ObserveFloat64(ins.storageLatency, p99, metric.WithAttributes(append(base, attribute.String("op", op), attribute.String("quantile", "0.99"))...))
		count := readIOPS
		if op == "write" {
			count = writeIOPS
		}
		st.cumStorageLatCount[op] += int64(count * dt)
		st.cumStorageLatSum[op] += count * dt * p50
		o.ObserveFloat64(ins.storageLatencyCount, float64(st.cumStorageLatCount[op]),
			metric.WithAttributes(append(base, attribute.String("op", op))...))
		o.ObserveFloat64(ins.storageLatencySum, st.cumStorageLatSum[op],
			metric.WithAttributes(append(base, attribute.String("op", op))...))
	}
	o.ObserveFloat64(ins.volumeHealth, 1.0, metric.WithAttributes(base...))

	// --- Host ---
	cpu := 0.18 + 0.10*loadMult
	if diskFull {
		cpu += 0.05 // some thrashing
	}
	o.ObserveFloat64(ins.hostCPU, math.Min(0.95, cpu), metric.WithAttributes(base...))
	mem := 4 * 1024 * 1024 * 1024 * (0.55 + 0.05*loadMult) // 4 GB baseline
	o.ObserveFloat64(ins.hostMemory, mem, metric.WithAttributes(base...))

	// --- Control plane ---
	for _, result := range []string{"ok", "timeout", "abort"} {
		// Failovers are rare — emit zero count baseline.
		o.ObserveFloat64(ins.failoverEvents, float64(st.cumFailover[result]),
			metric.WithAttributes(append(base, attribute.String("result", result))...))
	}
	// Last successful backup age — 6h baseline, climbs when disk-full.
	age := 6 * 3600.0
	if diskFull {
		age = 26 * 3600.0 // backup takes longer when volume is near-full
	}
	o.ObserveFloat64(ins.backupLastSuccessAge, age, metric.WithAttributes(base...))
}

// baselineQueryLatency returns p50/p95/p99 (seconds) for a given op type
// under nominal load. SELECT is fast, writes a bit slower, DDL slowest.
func baselineQueryLatency(op string) (float64, float64, float64) {
	switch op {
	case "select":
		return 0.005, 0.025, 0.080
	case "insert", "update":
		return 0.008, 0.040, 0.120
	case "delete":
		return 0.010, 0.045, 0.130
	case "ddl":
		return 0.050, 0.200, 0.450
	}
	return 0.010, 0.050, 0.100
}

// registerAll creates every instrument. Returned errors are wrapped so the
// caller knows which metric failed.
func registerAll(meter metric.Meter, ins *instruments) error {
	var err error
	g := func(name, desc string) metric.Float64ObservableGauge {
		if err != nil {
			return nil
		}
		var gg metric.Float64ObservableGauge
		gg, err = meter.Float64ObservableGauge(name, metric.WithDescription(desc))
		return gg
	}
	c := func(name, desc string) metric.Float64ObservableCounter {
		if err != nil {
			return nil
		}
		var cc metric.Float64ObservableCounter
		cc, err = meter.Float64ObservableCounter(name, metric.WithDescription(desc))
		return cc
	}

	ins.dbUp = g("db.up", "1 when the DB instance is up and accepting connections")
	ins.connectionsActive = g("db.connections.active", "Active client connections")
	ins.connectionsIdle = g("db.connections.idle", "Idle client connections")
	ins.connectionsMax = g("db.connections.max", "Max client connections (parameter-group setting)")
	ins.connectionsRejected = c("db.connections.rejected_total", "Connections rejected, partitioned by reason")
	ins.connectionsWaiting = g("db.connections.waiting", "Clients queued at the pooler")

	ins.poolerClientActive = g("db.pooler.client_conn_active", "Pooler client connections active")
	ins.poolerClientWaiting = g("db.pooler.client_conn_waiting", "Pooler client connections waiting")
	ins.poolerServerActive = g("db.pooler.server_conn_active", "Pooler server-side connections active")
	ins.poolerExhausted = c("db.pooler.pool_exhausted_total", "Pooler pool exhaustion events")

	ins.txCommitted = c("db.tx.committed_total", "Transactions committed")
	ins.txRolledBack = c("db.tx.rolled_back_total", "Transactions rolled back")
	ins.queriesTotal = c("db.queries_total", "Queries by op")
	ins.queryErrors = c("db.queries.errors_total", "Query errors by SQLSTATE")
	ins.queryDuration = g("db.query.duration_seconds", "Query duration by op (quantile gauge)")
	ins.queryDurationCount = c("db.query.duration_seconds.count", "Query duration observation count")
	ins.queryDurationSum = c("db.query.duration_seconds.sum", "Query duration observation sum")
	ins.queriesSlow = c("db.queries.slow_total", "Queries exceeding 1s threshold")
	ins.queriesLongRunning = g("db.queries.long_running", "Currently active queries > 60s")

	ins.deadlocks = c("db.deadlocks_total", "Deadlocks detected")
	ins.lockWaitSec = c("db.lock.wait_seconds_total", "Total seconds spent waiting on locks, by lock_type")
	ins.lockWaiters = g("db.lock.waiters", "Sessions currently waiting on a lock")

	ins.bufferHitRatio = g("db.cache.buffer_hit_ratio", "Buffer cache hit ratio")
	ins.indexHitRatio = g("db.cache.index_hit_ratio", "Index cache hit ratio")
	ins.tempBytes = c("db.temp.bytes_total", "Temp file bytes written (aggregate)")

	ins.walBytes = c("db.wal.bytes_total", "WAL bytes generated")
	ins.walLagBytes = g("db.wal.lag_bytes", "WAL lag in bytes, by replica_id")
	ins.walLagSeconds = g("db.wal.lag_seconds", "WAL lag in seconds, by replica_id")
	ins.replicationSlotLag = g("db.replication.slot_lag_bytes", "Replication slot lag in bytes")

	ins.checkpointTotal = c("db.checkpoint_total", "Checkpoint events by kind")
	ins.checkpointDuration = g("db.checkpoint.duration_seconds", "Checkpoint duration (quantile gauge)")
	ins.checkpointDurationCount = c("db.checkpoint.duration_seconds.count", "Checkpoint duration observation count")
	ins.checkpointDurationSum = c("db.checkpoint.duration_seconds.sum", "Checkpoint duration observation sum")
	ins.bgwriterBuffersWritten = c("db.bgwriter.buffers_written_total", "Background writer buffers written")
	ins.autovacuumDuration = g("db.autovacuum.duration_seconds", "Autovacuum duration (quantile gauge)")
	ins.autovacuumDurationCount = c("db.autovacuum.duration_seconds.count", "Autovacuum duration observation count")
	ins.autovacuumDurationSum = c("db.autovacuum.duration_seconds.sum", "Autovacuum duration observation sum")

	ins.storageUsed = g("db.storage.used_bytes", "Bytes used on the volume")
	ins.storageCapacity = g("db.storage.capacity_bytes", "Volume's allocated capacity in bytes")
	ins.storageIOPSRead = g("db.storage.iops_read", "Current read IOPS draw on the volume")
	ins.storageIOPSWrite = g("db.storage.iops_write", "Current write IOPS draw on the volume")
	ins.storageIOPSProvisioned = g("db.storage.iops_provisioned", "Volume's contracted IOPS ceiling")
	ins.storageLatency = g("db.storage.latency_seconds", "Storage latency by op (quantile gauge)")
	ins.storageLatencyCount = c("db.storage.latency_seconds.count", "Storage latency observation count")
	ins.storageLatencySum = c("db.storage.latency_seconds.sum", "Storage latency observation sum")
	ins.volumeHealth = g("db.storage.volume_health", "1 when volume hardware health is OK")

	ins.hostCPU = g("db.host.cpu_usage_ratio", "Underlying host CPU usage ratio")
	ins.hostMemory = g("db.host.memory_used_bytes", "Underlying host memory used bytes")

	ins.failoverEvents = c("db.failover.events_total", "Failover events by result")
	ins.backupLastSuccessAge = g("db.backup.last_success_age_seconds", "Age of the last successful backup in seconds")

	if err != nil {
		return fmt.Errorf("create dbaas instruments: %w", err)
	}
	return nil
}
