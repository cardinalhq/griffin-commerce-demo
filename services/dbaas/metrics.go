// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const instrumentScope = "github.com/cardinalhq/griffin-commerce-demo/services/dbaas"

// Metric keys — used by scenario.go to address impact specs. Strings match
// the spec metric names so logs/dashboards can grep them.
const (
	// Tenant SLO (spec §7)
	MetricTenantSLOBurnRate    = "tenant_slo_burn_rate"
	MetricTenantSLOCompliance  = "tenant_slo_compliance_ratio"
	MetricTenantSLOErrorBudget = "tenant_slo_error_budget_remaining_ratio"

	// Postgres (spec §8)
	MetricPGUp                  = "pg_up"
	MetricPGNumBackends         = "pg_stat_database_numbackends"
	MetricPGXactCommitTotal     = "pg_stat_database_xact_commit_total"
	MetricPGXactRollbackTotal   = "pg_stat_database_xact_rollback_total"
	MetricPGTupFetchedTotal     = "pg_stat_database_tup_fetched_total"
	MetricPGBlksReadTotal       = "pg_stat_database_blks_read_total"
	MetricPGBlksHitTotal        = "pg_stat_database_blks_hit_total"
	MetricPGCacheHitRatio       = "pg_database_cache_hit_ratio"
	MetricPGQueryLatencyBucket  = "pg_query_latency_seconds_bucket"
	MetricPGQueryLatencyCount   = "pg_query_latency_seconds_count"
	MetricPGQueryLatencySum     = "pg_query_latency_seconds_sum"
	MetricPGQueryLatencyP95Ms   = "pg_query_latency_p95_ms"
	MetricPGLocksCount          = "pg_locks_count"
	MetricPGActivityCount       = "pg_stat_activity_count"
	MetricPGCheckpointWriteRate = "pg_checkpoint_write_time_seconds_total"
	MetricPGCheckpointSyncRate  = "pg_checkpoint_sync_time_seconds_total"
	MetricPGWalBytesTotal       = "pg_wal_bytes_total"
	MetricPGReplicationLag      = "pg_replication_lag_seconds"

	// Linux VM (spec §9)
	MetricNodeCPUSecondsTotal = "node_cpu_seconds_total"
	MetricNodeIOWaitPct       = "node_cpu_iowait_percent"
	MetricNodeCPUStealPct     = "node_cpu_steal_percent"
	MetricNodeMemAvailable    = "node_memory_MemAvailable_bytes"
	MetricNodeDiskReadTime    = "node_disk_read_time_seconds_total"
	MetricNodeDiskWriteTime   = "node_disk_write_time_seconds_total"
	MetricNodeDiskIOTime      = "node_disk_io_time_seconds_total"
	MetricNodeDiskIONow       = "node_disk_io_now"
	MetricNodeFilesystemAvail = "node_filesystem_avail_bytes"
	MetricNodeNetRxDrop       = "node_network_receive_drop_total"
	MetricNodeNetTxDrop       = "node_network_transmit_drop_total"
	MetricNodeLoad1           = "node_load1"
	MetricNodeContextSwitches = "node_context_switches_total"
	MetricNodePSwapInRate     = "node_vmstat_pswpin"
	MetricNodePSwapOutRate    = "node_vmstat_pswpout"
	MetricNodeMemPressureRate = "node_pressure_memory_waiting_seconds_total"

	// VMware VM (spec §10)
	MetricVMwareVMPowerState   = "vmware_vm_power_state"
	MetricVMwareVMCPUUsagePct  = "vmware_vm_cpu_usage_percent"
	MetricVMwareVMCPUReadyMs   = "vmware_vm_cpu_ready_summation_ms"
	MetricVMwareVMMemUsagePct  = "vmware_vm_memory_usage_percent"
	MetricVMwareVMMemBallooned = "vmware_vm_memory_ballooned_bytes"
	MetricVMwareVMDiskReadLat  = "vmware_vm_disk_read_latency_ms"
	MetricVMwareVMDiskWriteLat = "vmware_vm_disk_write_latency_ms"
	MetricVMwareVMDiskUsage    = "vmware_vm_disk_usage_bytes"
	MetricVMwareVMNetRxBytes   = "vmware_vm_net_received_bytes_total"
	MetricVMwareVMNetTxBytes   = "vmware_vm_net_transmitted_bytes_total"

	// ESXi host (spec §11)
	MetricVMwareHostCPUUsagePct  = "vmware_host_cpu_usage_percent"
	MetricVMwareHostMemUsagePct  = "vmware_host_memory_usage_percent"
	MetricVMwareHostDiskReadLat  = "vmware_host_disk_read_latency_ms"
	MetricVMwareHostDiskWriteLat = "vmware_host_disk_write_latency_ms"
	MetricVMwareHostNetRxDropped = "vmware_host_net_dropped_rx_total"
	MetricVMwareHostNetTxDropped = "vmware_host_net_dropped_tx_total"
	MetricVMwareHostVMCount      = "vmware_host_vm_count"

	// Datastore (spec §12)
	MetricDSCapacity   = "vmware_datastore_capacity_bytes"
	MetricDSFreeBytes  = "vmware_datastore_free_bytes"
	MetricDSReadLat    = "vmware_datastore_read_latency_ms"
	MetricDSWriteLat   = "vmware_datastore_write_latency_ms"
	MetricDSIOPSRead   = "vmware_datastore_iops_read"
	MetricDSIOPSWrite  = "vmware_datastore_iops_write"
	MetricDSQueueDepth = "vmware_datastore_queue_depth"

	// Probe (spec §13)
	MetricAirtelProbeSuccessRate = "airtel_probe_success_ratio"
	MetricAirtelProbeLatencyMs   = "airtel_probe_latency_ms"

	// Alert (spec §14)
	MetricCardinalAlertActive = "cardinal_alert_active"
)

// pgLatencyBuckets are the `le` upper-bounds for pg_query_latency_seconds_bucket
// per spec §8.9.
var pgLatencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

var pgQueryClasses = []string{"oltp_read", "oltp_write", "reporting", "background"}

var pgLockModes = []string{"AccessShareLock", "RowExclusiveLock", "ShareLock", "ExclusiveLock"}

type instruments struct {
	// Tenant SLO
	tenantSLOBurn        metric.Float64ObservableGauge
	tenantSLOCompliance  metric.Float64ObservableGauge
	tenantSLOErrorBudget metric.Float64ObservableGauge

	// Postgres
	pgUp              metric.Float64ObservableGauge
	pgNumBackends     metric.Float64ObservableGauge
	pgXactCommit      metric.Float64ObservableCounter
	pgXactRollback    metric.Float64ObservableCounter
	pgTupFetched      metric.Float64ObservableCounter
	pgBlksRead        metric.Float64ObservableCounter
	pgBlksHit         metric.Float64ObservableCounter
	pgCacheHitRatio   metric.Float64ObservableGauge
	pgLatencyBucket   metric.Float64ObservableCounter
	pgLatencyCount    metric.Float64ObservableCounter
	pgLatencySum      metric.Float64ObservableCounter
	pgLatencyP95      metric.Float64ObservableGauge
	pgLocksCount      metric.Float64ObservableGauge
	pgActivityCount   metric.Float64ObservableGauge
	pgCheckpointWrite metric.Float64ObservableCounter
	pgCheckpointSync  metric.Float64ObservableCounter
	pgWalBytes        metric.Float64ObservableCounter
	pgReplicationLag  metric.Float64ObservableGauge

	// Linux VM
	nodeCPUSeconds      metric.Float64ObservableCounter
	nodeIOWaitPct       metric.Float64ObservableGauge
	nodeCPUStealPct     metric.Float64ObservableGauge
	nodeMemAvailable    metric.Float64ObservableGauge
	nodeDiskReadTime    metric.Float64ObservableCounter
	nodeDiskWriteTime   metric.Float64ObservableCounter
	nodeDiskIOTime      metric.Float64ObservableCounter
	nodeDiskIONow       metric.Float64ObservableGauge
	nodeFsAvail         metric.Float64ObservableGauge
	nodeNetRxDrop       metric.Float64ObservableCounter
	nodeNetTxDrop       metric.Float64ObservableCounter
	nodeLoad1           metric.Float64ObservableGauge
	nodeContextSwitches metric.Float64ObservableCounter
	nodePSwapIn         metric.Float64ObservableCounter
	nodePSwapOut        metric.Float64ObservableCounter
	nodeMemPressure     metric.Float64ObservableCounter

	// VMware VM
	vmwareVMPowerState   metric.Float64ObservableGauge
	vmwareVMCPUUsagePct  metric.Float64ObservableGauge
	vmwareVMCPUReadyMs   metric.Float64ObservableGauge
	vmwareVMMemUsagePct  metric.Float64ObservableGauge
	vmwareVMMemBallooned metric.Float64ObservableGauge
	vmwareVMDiskReadLat  metric.Float64ObservableGauge
	vmwareVMDiskWriteLat metric.Float64ObservableGauge
	vmwareVMDiskUsage    metric.Float64ObservableGauge
	vmwareVMNetRx        metric.Float64ObservableCounter
	vmwareVMNetTx        metric.Float64ObservableCounter

	// Host
	vmwareHostCPUUsagePct  metric.Float64ObservableGauge
	vmwareHostMemUsagePct  metric.Float64ObservableGauge
	vmwareHostDiskReadLat  metric.Float64ObservableGauge
	vmwareHostDiskWriteLat metric.Float64ObservableGauge
	vmwareHostNetRxDropped metric.Float64ObservableCounter
	vmwareHostNetTxDropped metric.Float64ObservableCounter
	vmwareHostVMCount      metric.Float64ObservableGauge

	// Datastore
	dsCapacity   metric.Float64ObservableGauge
	dsFreeBytes  metric.Float64ObservableGauge
	dsReadLat    metric.Float64ObservableGauge
	dsWriteLat   metric.Float64ObservableGauge
	dsIOPSRead   metric.Float64ObservableGauge
	dsIOPSWrite  metric.Float64ObservableGauge
	dsQueueDepth metric.Float64ObservableGauge

	// Probe
	probeSuccessRate metric.Float64ObservableGauge
	probeLatencyMs   metric.Float64ObservableGauge

	// Alert
	alertActive metric.Float64ObservableGauge
}

// emitPGHistogram is the env-controlled cardinality switch for the
// pg_query_latency_seconds_bucket family. Default on; flip
// DBAAS_EMIT_PG_HIST=false at deploy time if the demo cap is tight.
func emitPGHistogram() bool {
	v := os.Getenv("DBAAS_EMIT_PG_HIST")
	return v == "" || v == "true" || v == "1"
}

// RegisterMetrics builds every instrument and registers a single async
// callback that walks the catalog on the SDK's collection cadence.
func RegisterMetrics(ctx context.Context, catalog *Catalog, scenario *Scenario) error {
	meter := otel.Meter(instrumentScope)
	ins := &instruments{}
	if err := registerAll(meter, ins); err != nil {
		return err
	}
	obs := allObservables(ins)
	_, err := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			now := time.Now()
			for _, t := range catalog.Tenants {
				observeTenant(o, ins, t, scenario, now)
				observeProbe(o, ins, t, scenario, now)
			}
			for _, pg := range catalog.PGInstances {
				observePG(o, ins, pg, scenario, now)
			}
			for _, vm := range catalog.VMs {
				observeLinuxVM(o, ins, vm, scenario, now)
				observeVMwareVM(o, ins, vm, scenario, now)
			}
			for _, h := range catalog.Hosts {
				observeHost(o, ins, h, scenario, catalog, now)
			}
			for _, d := range catalog.Datastores {
				observeDatastore(o, ins, d, scenario, now)
			}
			observeAlerts(o, ins, scenario, catalog, now)
			return nil
		},
		obs...,
	)
	if err != nil {
		return fmt.Errorf("register dbaas callback: %w", err)
	}
	slog.InfoContext(ctx, "Airtel telemetry simulator metrics registered",
		"tenants", len(catalog.Tenants),
		"pg_instances", len(catalog.PGInstances),
		"vms", len(catalog.VMs),
		"hosts", len(catalog.Hosts),
		"datastores", len(catalog.Datastores),
		"emit_pg_histogram", emitPGHistogram(),
	)
	return nil
}

func allObservables(ins *instruments) []metric.Observable {
	return []metric.Observable{
		ins.tenantSLOBurn, ins.tenantSLOCompliance, ins.tenantSLOErrorBudget,
		ins.pgUp, ins.pgNumBackends, ins.pgXactCommit, ins.pgXactRollback,
		ins.pgTupFetched, ins.pgBlksRead, ins.pgBlksHit, ins.pgCacheHitRatio,
		ins.pgLatencyBucket, ins.pgLatencyCount, ins.pgLatencySum, ins.pgLatencyP95,
		ins.pgLocksCount, ins.pgActivityCount,
		ins.pgCheckpointWrite, ins.pgCheckpointSync, ins.pgWalBytes, ins.pgReplicationLag,
		ins.nodeCPUSeconds, ins.nodeIOWaitPct, ins.nodeCPUStealPct, ins.nodeMemAvailable,
		ins.nodeDiskReadTime, ins.nodeDiskWriteTime, ins.nodeDiskIOTime, ins.nodeDiskIONow,
		ins.nodeFsAvail, ins.nodeNetRxDrop, ins.nodeNetTxDrop,
		ins.nodeLoad1, ins.nodeContextSwitches, ins.nodePSwapIn, ins.nodePSwapOut,
		ins.nodeMemPressure,
		ins.vmwareVMPowerState, ins.vmwareVMCPUUsagePct, ins.vmwareVMCPUReadyMs,
		ins.vmwareVMMemUsagePct, ins.vmwareVMMemBallooned,
		ins.vmwareVMDiskReadLat, ins.vmwareVMDiskWriteLat, ins.vmwareVMDiskUsage,
		ins.vmwareVMNetRx, ins.vmwareVMNetTx,
		ins.vmwareHostCPUUsagePct, ins.vmwareHostMemUsagePct,
		ins.vmwareHostDiskReadLat, ins.vmwareHostDiskWriteLat,
		ins.vmwareHostNetRxDropped, ins.vmwareHostNetTxDropped, ins.vmwareHostVMCount,
		ins.dsCapacity, ins.dsFreeBytes, ins.dsReadLat, ins.dsWriteLat,
		ins.dsIOPSRead, ins.dsIOPSWrite, ins.dsQueueDepth,
		ins.probeSuccessRate, ins.probeLatencyMs,
		ins.alertActive,
	}
}

// -- attribute builders --

func tenantAttrs(t *Tenant) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("tenant_id", t.TenantID),
		attribute.String("customer_id", t.CustomerID),
		attribute.String("account_id", t.AccountID),
		attribute.String("service_tier", t.Tier),
		attribute.String("environment", t.Environment),
		attribute.String("region", t.Region),
		attribute.String("az", t.AZ),
	}
}

func tenantSLOAttrs(t *Tenant) []attribute.KeyValue {
	return append(tenantAttrs(t),
		attribute.String("slo_name", t.SLOName),
		attribute.String("managed_service", "postgres"),
	)
}

func pgAttrs(pg *PGInstance) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("pg_instance", pg.Name),
		attribute.String("pg_cluster", pg.Cluster),
		attribute.String("pg_role", pg.Role),
		attribute.String("pg_version", pg.Version),
		attribute.String("database_name", pg.Database),
		attribute.String("port", strconv.Itoa(pg.Port)),
		attribute.String("tenant_id", pg.Tenant.TenantID),
		attribute.String("customer_id", pg.Tenant.CustomerID),
	}
	if pg.VM != nil {
		attrs = append(attrs,
			attribute.String("vm_name", pg.VM.Name),
			attribute.String("vm_uuid", pg.VM.UUID),
		)
		if pg.VM.Host != nil {
			attrs = append(attrs,
				attribute.String("esxi_host_name", pg.VM.Host.Name),
				attribute.String("esxi_host_id", pg.VM.Host.ID),
			)
		}
		if pg.VM.Datastore != nil {
			attrs = append(attrs,
				attribute.String("datastore_name", pg.VM.Datastore.Name),
				attribute.String("datastore_id", pg.VM.Datastore.ID),
			)
		}
	}
	return attrs
}

func vmAttrs(vm *VM) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("vm_name", vm.Name),
		attribute.String("vm_uuid", vm.UUID),
		attribute.String("vm_moid", vm.MOID),
		attribute.String("workload_role", vm.WorkloadRole),
		attribute.String("os", vm.OS),
		attribute.String("vcenter_name", vcenterName),
		attribute.String("cluster", vm.Cluster.Name),
		attribute.String("region", vm.Region),
		attribute.String("az", vm.AZ),
	}
	if vm.Tenant != nil {
		attrs = append(attrs,
			attribute.String("tenant_id", vm.Tenant.TenantID),
			attribute.String("customer_id", vm.Tenant.CustomerID),
		)
	}
	if vm.Host != nil {
		attrs = append(attrs,
			attribute.String("esxi_host_name", vm.Host.Name),
			attribute.String("esxi_host_id", vm.Host.ID),
		)
	}
	if vm.Datastore != nil {
		attrs = append(attrs,
			attribute.String("datastore_name", vm.Datastore.Name),
			attribute.String("datastore_id", vm.Datastore.ID),
		)
	}
	return attrs
}

func hostAttrs(h *Host) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("vcenter_name", vcenterName),
		attribute.String("datacenter", datacenterDelhi),
		attribute.String("cluster", h.Cluster.Name),
		attribute.String("esxi_host_name", h.Name),
		attribute.String("esxi_host_id", h.ID),
		attribute.String("rack_id", h.RackID),
	}
}

func dsAttrs(d *Datastore) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("vcenter_name", vcenterName),
		attribute.String("datacenter", datacenterDelhi),
		attribute.String("datastore_name", d.Name),
		attribute.String("datastore_id", d.ID),
		attribute.String("datastore_type", d.Type),
		attribute.String("storage_array", d.StorageArray),
		attribute.String("service_tier", d.ServiceTier),
	}
}

// -- observers --

func observeTenant(o metric.Observer, ins *instruments, t *Tenant, sc *Scenario, now time.Time) {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	sel := selectorTenant(t.TenantID)
	burn := sc.RampedValue(sel, MetricTenantSLOBurnRate, Range{0.0, 1.0}, t.state.seed, now)
	comp := sc.RampedValue(sel, MetricTenantSLOCompliance, Range{0.9990, 1.0}, t.state.seed^1, now)
	budget := sc.RampedValue(sel, MetricTenantSLOErrorBudget, Range{0.65, 1.0}, t.state.seed^2, now)
	base := tenantSLOAttrs(t)
	o.ObserveFloat64(ins.tenantSLOBurn, burn, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.tenantSLOCompliance, clamp(comp, 0, 1), metric.WithAttributes(base...))
	o.ObserveFloat64(ins.tenantSLOErrorBudget, clamp(budget, 0, 1), metric.WithAttributes(base...))
}

func observeProbe(o metric.Observer, ins *instruments, t *Tenant, sc *Scenario, now time.Time) {
	sel := selectorTenant(t.TenantID)
	succ := sc.RampedValue(sel, MetricAirtelProbeSuccessRate, Range{0.999, 1.0}, t.state.seed^7, now)
	lat := sc.RampedValue(sel, MetricAirtelProbeLatencyMs, Range{20, 80}, t.state.seed^8, now)
	attrs := append(tenantAttrs(t),
		attribute.String("probe_name", "postgres_tcp_connect_and_simple_query"),
		attribute.String("target_service", "postgres"),
	)
	o.ObserveFloat64(ins.probeSuccessRate, clamp(succ, 0, 1), metric.WithAttributes(attrs...))
	o.ObserveFloat64(ins.probeLatencyMs, lat, metric.WithAttributes(attrs...))
}

func observePG(o metric.Observer, ins *instruments, pg *PGInstance, sc *Scenario, now time.Time) {
	pg.state.mu.Lock()
	defer pg.state.mu.Unlock()
	dt := dtAdvance(&pg.state.lastTick, now)
	sel := selectorPG(pg.Name)
	base := pgAttrs(pg)

	o.ObserveFloat64(ins.pgUp, 1, metric.WithAttributes(base...))

	numBackends := sc.RampedValue(sel, MetricPGNumBackends, Range{30, 180}, pg.state.seed^1, now)
	o.ObserveFloat64(ins.pgNumBackends, numBackends, metric.WithAttributes(base...))

	// Transaction counters
	commitRate := 800.0 // mid baseline (300–1500 range)
	if active := sc.IsActiveOn(sel, MetricPGQueryLatencyP95Ms, now); active {
		commitRate = 200 // flat / reduced under incident
	}
	pg.state.cumXactCommit += commitRate * dt
	rollbackRate := sc.RampedValue(sel, MetricPGXactRollbackTotal, Range{1, 10}, pg.state.seed^2, now)
	pg.state.cumXactRollback += rollbackRate * dt
	o.ObserveFloat64(ins.pgXactCommit, pg.state.cumXactCommit, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.pgXactRollback, pg.state.cumXactRollback, metric.WithAttributes(base...))

	tupRate := 18000.0
	if sc.IsActiveOn(sel, MetricPGQueryLatencyP95Ms, now) {
		tupRate = 6000
	}
	pg.state.cumTupFetched += tupRate * dt
	o.ObserveFloat64(ins.pgTupFetched, pg.state.cumTupFetched, metric.WithAttributes(base...))

	blksReadRate := 1500.0
	blksHitRate := 48000.0
	if sc.IsActiveOn(sel, MetricPGCacheHitRatio, now) {
		blksReadRate = 8000
		blksHitRate = 32000
	}
	pg.state.cumBlksRead += blksReadRate * dt
	pg.state.cumBlksHit += blksHitRate * dt
	o.ObserveFloat64(ins.pgBlksRead, pg.state.cumBlksRead, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.pgBlksHit, pg.state.cumBlksHit, metric.WithAttributes(base...))

	cacheHit := sc.RampedValue(sel, MetricPGCacheHitRatio, Range{0.96, 0.995}, pg.state.seed^3, now)
	o.ObserveFloat64(ins.pgCacheHitRatio, clamp(cacheHit, 0, 1), metric.WithAttributes(base...))

	// p95 latency gauge + histogram (per query_class)
	for _, qc := range pgQueryClasses {
		p95Ms := sc.RampedValue(sel, MetricPGQueryLatencyP95Ms, Range{35, 90}, pg.state.seed^uint64(len(qc)), now)
		// SELECT-ish (oltp_read) is the lowest baseline; ddl-ish (reporting/background) is higher.
		switch qc {
		case "oltp_write":
			p95Ms *= 1.4
		case "reporting":
			p95Ms *= 2.0
		case "background":
			p95Ms *= 1.8
		}
		attrs := append(base, attribute.String("query_class", qc))
		o.ObserveFloat64(ins.pgLatencyP95, p95Ms, metric.WithAttributes(attrs...))

		if emitPGHistogram() {
			emitPGLatencyHistogram(o, ins, pg, qc, p95Ms/1000.0, base, dt)
		}
	}

	// pg_locks_count per mode
	lockTotal := sc.RampedValue(sel, MetricPGLocksCount, Range{20, 500}, pg.state.seed^4, now)
	for i, mode := range pgLockModes {
		share := []float64{0.6, 0.25, 0.10, 0.05}[i]
		o.ObserveFloat64(ins.pgLocksCount, lockTotal*share,
			metric.WithAttributes(append(base, attribute.String("mode", mode))...))
	}

	// pg_stat_activity_count: emit a curated set of (state, wait_event_type, wait_event) tuples
	emitPGActivity(o, ins, pg, sc, sel, base, now)

	// Checkpoint rates as counters
	cpWriteRate := sc.RampedValue(sel, MetricPGCheckpointWriteRate, Range{0.1, 1.0}, pg.state.seed^5, now)
	cpSyncRate := sc.RampedValue(sel, MetricPGCheckpointSyncRate, Range{0.01, 0.2}, pg.state.seed^6, now)
	pg.state.cumCheckpointWrite += cpWriteRate * dt
	pg.state.cumCheckpointSync += cpSyncRate * dt
	o.ObserveFloat64(ins.pgCheckpointWrite, pg.state.cumCheckpointWrite, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.pgCheckpointSync, pg.state.cumCheckpointSync, metric.WithAttributes(base...))

	walRate := 8.0 * 1024 * 1024 // 8 MB/s baseline
	pg.state.cumWalBytes += walRate * dt
	o.ObserveFloat64(ins.pgWalBytes, pg.state.cumWalBytes, metric.WithAttributes(base...))

	repLag := sc.RampedValue(sel, MetricPGReplicationLag, Range{0, 2}, pg.state.seed^7, now)
	o.ObserveFloat64(ins.pgReplicationLag, repLag,
		metric.WithAttributes(append(base, attribute.String("replica_name", "replica-01"))...))
}

func emitPGLatencyHistogram(o metric.Observer, ins *instruments, pg *PGInstance, qc string, p95Sec float64, base []attribute.KeyValue, dt float64) {
	// Approximate the query rate per query_class as 50 q/s baseline; spec
	// permits a single nominal value here since the histogram is only for
	// shape demos.
	qps := 50.0
	deltaCount := qps * dt
	if _, ok := pg.state.cumLatencyBuckets[qc]; !ok {
		pg.state.cumLatencyBuckets[qc] = map[string]float64{}
	}
	p50 := p95Sec / 3
	p99 := p95Sec * 2
	for _, le := range pgLatencyBuckets {
		frac := cumulativeLatencyFraction(le, p50, p95Sec, p99)
		key := strconv.FormatFloat(le, 'f', -1, 64)
		pg.state.cumLatencyBuckets[qc][key] += deltaCount * frac
		attrs := append(base,
			attribute.String("query_class", qc),
			attribute.String("le", key),
		)
		o.ObserveFloat64(ins.pgLatencyBucket, pg.state.cumLatencyBuckets[qc][key], metric.WithAttributes(attrs...))
	}
	// +Inf bucket
	pg.state.cumLatencyBuckets[qc]["+Inf"] = pg.state.cumLatencyCount[qc] + deltaCount
	attrs := append(base,
		attribute.String("query_class", qc),
		attribute.String("le", "+Inf"),
	)
	o.ObserveFloat64(ins.pgLatencyBucket, pg.state.cumLatencyBuckets[qc]["+Inf"], metric.WithAttributes(attrs...))

	pg.state.cumLatencyCount[qc] += deltaCount
	pg.state.cumLatencySum[qc] += deltaCount * p50
	o.ObserveFloat64(ins.pgLatencyCount, pg.state.cumLatencyCount[qc],
		metric.WithAttributes(append(base, attribute.String("query_class", qc))...))
	o.ObserveFloat64(ins.pgLatencySum, pg.state.cumLatencySum[qc],
		metric.WithAttributes(append(base, attribute.String("query_class", qc))...))
}

// cumulativeLatencyFraction returns the share of queries with latency ≤ le
// given a piecewise-linear CDF defined by p50/p95/p99.
func cumulativeLatencyFraction(le, p50, p95, p99 float64) float64 {
	switch {
	case le <= 0:
		return 0
	case le < p50:
		return 0.5 * (le / p50)
	case le < p95:
		return 0.5 + 0.45*(le-p50)/(p95-p50)
	case le < p99:
		return 0.95 + 0.04*(le-p95)/(p99-p95)
	default:
		return 0.99
	}
}

func emitPGActivity(o metric.Observer, ins *instruments, pg *PGInstance, sc *Scenario, sel string, base []attribute.KeyValue, now time.Time) {
	type act struct {
		state, weType, wEvent string
		baseRange             Range
	}
	acts := []act{
		{"active", "", "", Range{20, 60}},
		{"idle", "Client", "ClientRead", Range{30, 90}},
		{"idle in transaction", "Client", "ClientRead", Range{0, 10}},
		{"active", "IO", "DataFileRead", Range{0, 8}},
		{"active", "IO", "DataFileWrite", Range{0, 8}},
		{"active", "Lock", "tuple", Range{0, 4}},
	}
	ioWaitActive := sc.IsActiveOn(sel, MetricPGQueryLatencyP95Ms, now) ||
		sc.IsActiveOn(sel, MetricPGCacheHitRatio, now)
	for i, a := range acts {
		rng := a.baseRange
		if ioWaitActive && a.weType == "IO" {
			rng = Range{30, 90}
		}
		val := rng.Sample(pg.state.seed^uint64(i+10), now)
		attrs := append(base,
			attribute.String("state", a.state),
		)
		if a.weType != "" {
			attrs = append(attrs,
				attribute.String("wait_event_type", a.weType),
				attribute.String("wait_event", a.wEvent),
			)
		}
		o.ObserveFloat64(ins.pgActivityCount, val, metric.WithAttributes(attrs...))
	}
}

func observeLinuxVM(o metric.Observer, ins *instruments, vm *VM, sc *Scenario, now time.Time) {
	vm.state.mu.Lock()
	defer vm.state.mu.Unlock()
	dt := dtAdvance(&vm.state.lastTick, now)
	sel := selectorVM(vm.Name)
	base := vmAttrs(vm)
	instAttr := attribute.String("instance", vm.Name+":9100")
	dev := attribute.String("device", "sda")

	// CPU mode counters — emit user/system/idle/iowait/steal cumulative seconds.
	cpuModes := []struct {
		mode    string
		baseRPS float64
	}{
		{"user", 0.6},
		{"system", 0.2},
		{"idle", 0.18},
		{"iowait", 0.02},
		{"steal", 0.005},
		{"irq", 0.001},
		{"softirq", 0.001},
	}
	for _, m := range cpuModes {
		rate := m.baseRPS
		if m.mode == "iowait" {
			rate = sc.RampedValue(sel, MetricNodeIOWaitPct, Range{0.5, 5.0}, vm.state.seed^uint64(m.mode[0]), now) / 100.0
		}
		if m.mode == "steal" {
			rate = sc.RampedValue(sel, MetricNodeCPUStealPct, Range{0.0, 2.0}, vm.state.seed^uint64(m.mode[0]), now) / 100.0
		}
		vm.state.cumCPUSeconds[m.mode] += rate * dt
		attrs := append(base, instAttr,
			attribute.String("cpu", "0"),
			attribute.String("mode", m.mode),
		)
		o.ObserveFloat64(ins.nodeCPUSeconds, vm.state.cumCPUSeconds[m.mode], metric.WithAttributes(attrs...))
	}

	o.ObserveFloat64(ins.nodeIOWaitPct,
		sc.RampedValue(sel, MetricNodeIOWaitPct, Range{0.5, 5.0}, vm.state.seed^1, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.nodeCPUStealPct,
		sc.RampedValue(sel, MetricNodeCPUStealPct, Range{0.0, 2.0}, vm.state.seed^2, now),
		metric.WithAttributes(base...))

	// Memory available — baseline ~30% of total. RampedValue gives a fraction
	// for the memory-pressure profile; multiply by total to get bytes.
	memTotal := float64(vm.MemoryGiB) * 1024 * 1024 * 1024
	memFrac := sc.RampedValue(sel, MetricNodeMemAvailable, Range{0.20, 0.45}, vm.state.seed^3, now)
	o.ObserveFloat64(ins.nodeMemAvailable, memFrac*memTotal,
		metric.WithAttributes(append(base, instAttr)...))

	diskReadRate := sc.RampedValue(sel, MetricNodeDiskReadTime, Range{0.01, 0.5}, vm.state.seed^4, now)
	diskWriteRate := sc.RampedValue(sel, MetricNodeDiskWriteTime, Range{0.02, 0.8}, vm.state.seed^5, now)
	vm.state.cumDiskRead += diskReadRate * dt
	vm.state.cumDiskWrite += diskWriteRate * dt
	vm.state.cumDiskIO += (diskReadRate + diskWriteRate) * dt
	attrsDisk := append(base, instAttr, dev)
	o.ObserveFloat64(ins.nodeDiskReadTime, vm.state.cumDiskRead, metric.WithAttributes(attrsDisk...))
	o.ObserveFloat64(ins.nodeDiskWriteTime, vm.state.cumDiskWrite, metric.WithAttributes(attrsDisk...))
	o.ObserveFloat64(ins.nodeDiskIOTime, vm.state.cumDiskIO, metric.WithAttributes(attrsDisk...))

	o.ObserveFloat64(ins.nodeDiskIONow,
		sc.RampedValue(sel, MetricNodeDiskIONow, Range{0, 4}, vm.state.seed^6, now),
		metric.WithAttributes(attrsDisk...))

	o.ObserveFloat64(ins.nodeFsAvail, 0.6*memTotal, // hand-wave: a few hundred GiB free
		metric.WithAttributes(append(base, instAttr, dev,
			attribute.String("mountpoint", "/"),
			attribute.String("fstype", "ext4"))...))

	vm.state.cumNetRxDrop += 0.5 * dt
	vm.state.cumNetTxDrop += 0.4 * dt
	o.ObserveFloat64(ins.nodeNetRxDrop, vm.state.cumNetRxDrop,
		metric.WithAttributes(append(base, instAttr, attribute.String("device", "eth0"))...))
	o.ObserveFloat64(ins.nodeNetTxDrop, vm.state.cumNetTxDrop,
		metric.WithAttributes(append(base, instAttr, attribute.String("device", "eth0"))...))

	o.ObserveFloat64(ins.nodeLoad1,
		sc.RampedValue(sel, MetricNodeLoad1, Range{1.0, 4.0}, vm.state.seed^7, now),
		metric.WithAttributes(append(base, instAttr)...))

	ctxRate := sc.RampedValue(sel, MetricNodeContextSwitches, Range{1000, 6000}, vm.state.seed^8, now)
	vm.state.cumContextSwitches += ctxRate * dt
	o.ObserveFloat64(ins.nodeContextSwitches, vm.state.cumContextSwitches,
		metric.WithAttributes(append(base, instAttr)...))

	pswpInRate := sc.RampedValue(sel, MetricNodePSwapInRate, Range{0, 1}, vm.state.seed^9, now)
	pswpOutRate := sc.RampedValue(sel, MetricNodePSwapOutRate, Range{0, 1}, vm.state.seed^10, now)
	vm.state.cumPSwapIn += pswpInRate * dt
	vm.state.cumPSwapOut += pswpOutRate * dt
	o.ObserveFloat64(ins.nodePSwapIn, vm.state.cumPSwapIn, metric.WithAttributes(append(base, instAttr)...))
	o.ObserveFloat64(ins.nodePSwapOut, vm.state.cumPSwapOut, metric.WithAttributes(append(base, instAttr)...))

	memPressureRate := sc.RampedValue(sel, MetricNodeMemPressureRate, Range{0, 0.02}, vm.state.seed^11, now)
	vm.state.cumMemPressure += memPressureRate * dt
	o.ObserveFloat64(ins.nodeMemPressure, vm.state.cumMemPressure,
		metric.WithAttributes(append(base, instAttr)...))
}

func observeVMwareVM(o metric.Observer, ins *instruments, vm *VM, sc *Scenario, now time.Time) {
	sel := selectorVM(vm.Name)
	base := vmAttrs(vm)
	o.ObserveFloat64(ins.vmwareVMPowerState, 1, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.vmwareVMCPUUsagePct,
		clamp(sc.RampedValue(sel, MetricVMwareVMCPUUsagePct, Range{20, 65}, vm.state.seed^20, now), 0, 100),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.vmwareVMCPUReadyMs,
		sc.RampedValue(sel, MetricVMwareVMCPUReadyMs, Range{50, 800}, vm.state.seed^21, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.vmwareVMMemUsagePct,
		clamp(sc.RampedValue(sel, MetricVMwareVMMemUsagePct, Range{45, 80}, vm.state.seed^22, now), 0, 100),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.vmwareVMMemBallooned,
		sc.RampedValue(sel, MetricVMwareVMMemBallooned, Range{0, 104857600}, vm.state.seed^23, now),
		metric.WithAttributes(base...))

	diskAttrs := append(base, attribute.String("virtual_disk", "Hard disk 1"))
	o.ObserveFloat64(ins.vmwareVMDiskReadLat,
		sc.RampedValue(sel, MetricVMwareVMDiskReadLat, Range{1, 12}, vm.state.seed^24, now),
		metric.WithAttributes(diskAttrs...))
	o.ObserveFloat64(ins.vmwareVMDiskWriteLat,
		sc.RampedValue(sel, MetricVMwareVMDiskWriteLat, Range{2, 15}, vm.state.seed^25, now),
		metric.WithAttributes(diskAttrs...))
	o.ObserveFloat64(ins.vmwareVMDiskUsage, float64(vm.MemoryGiB)*1024*1024*1024*4, // ~4× memory hand-wave
		metric.WithAttributes(base...))

	vm.state.cumVMwareNetRx += 200000 * 15 // ~200 KB/s baseline × 15s scrape
	vm.state.cumVMwareNetTx += 180000 * 15
	nicAttrs := append(base, attribute.String("nic", "vmnic0"))
	o.ObserveFloat64(ins.vmwareVMNetRx, vm.state.cumVMwareNetRx, metric.WithAttributes(nicAttrs...))
	o.ObserveFloat64(ins.vmwareVMNetTx, vm.state.cumVMwareNetTx, metric.WithAttributes(nicAttrs...))
}

func observeHost(o metric.Observer, ins *instruments, h *Host, sc *Scenario, c *Catalog, now time.Time) {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	dt := dtAdvance(&h.state.lastTick, now)
	sel := selectorHost(h.ID)
	base := hostAttrs(h)
	o.ObserveFloat64(ins.vmwareHostCPUUsagePct,
		clamp(sc.RampedValue(sel, MetricVMwareHostCPUUsagePct, Range{35, 70}, h.state.seed^1, now), 0, 100),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.vmwareHostMemUsagePct,
		clamp(sc.RampedValue(sel, MetricVMwareHostMemUsagePct, Range{45, 80}, h.state.seed^2, now), 0, 100),
		metric.WithAttributes(base...))
	// host disk latency is reported per datastore the host writes to. For the
	// degraded host we link the latency to datastore-202.
	dsAttr := attribute.String("datastore_id", "datastore-202")
	dsNameAttr := attribute.String("datastore_name", "ds-gold-delhi-02")
	o.ObserveFloat64(ins.vmwareHostDiskReadLat,
		sc.RampedValue(sel, MetricVMwareHostDiskReadLat, Range{1, 10}, h.state.seed^3, now),
		metric.WithAttributes(append(base, dsAttr, dsNameAttr)...))
	o.ObserveFloat64(ins.vmwareHostDiskWriteLat,
		sc.RampedValue(sel, MetricVMwareHostDiskWriteLat, Range{2, 15}, h.state.seed^4, now),
		metric.WithAttributes(append(base, dsAttr, dsNameAttr)...))
	for _, vmnic := range []string{"vmnic0", "vmnic1"} {
		h.state.cumNetDroppedRx += 0.2 * dt
		h.state.cumNetDroppedTx += 0.2 * dt
		nicAttrs := append(base, attribute.String("vmnic", vmnic))
		o.ObserveFloat64(ins.vmwareHostNetRxDropped, h.state.cumNetDroppedRx, metric.WithAttributes(nicAttrs...))
		o.ObserveFloat64(ins.vmwareHostNetTxDropped, h.state.cumNetDroppedTx, metric.WithAttributes(nicAttrs...))
	}
	o.ObserveFloat64(ins.vmwareHostVMCount, float64(len(c.VMsOnHost(h.ID))), metric.WithAttributes(base...))
}

func observeDatastore(o metric.Observer, ins *instruments, d *Datastore, sc *Scenario, now time.Time) {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()
	_ = dtAdvance(&d.state.lastTick, now)
	sel := selectorDatastore(d.ID)
	base := dsAttrs(d)
	capacity := float64(d.CapacityGiB) * 1024 * 1024 * 1024
	o.ObserveFloat64(ins.dsCapacity, capacity, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.dsFreeBytes, capacity*0.55, metric.WithAttributes(base...))
	o.ObserveFloat64(ins.dsReadLat,
		sc.RampedValue(sel, MetricDSReadLat, Range{1, 8}, d.state.seed^1, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.dsWriteLat,
		sc.RampedValue(sel, MetricDSWriteLat, Range{2, 12}, d.state.seed^2, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.dsIOPSRead,
		sc.RampedValue(sel, MetricDSIOPSRead, Range{500, 5000}, d.state.seed^3, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.dsIOPSWrite,
		sc.RampedValue(sel, MetricDSIOPSWrite, Range{500, 7000}, d.state.seed^4, now),
		metric.WithAttributes(base...))
	o.ObserveFloat64(ins.dsQueueDepth,
		sc.RampedValue(sel, MetricDSQueueDepth, Range{0, 20}, d.state.seed^5, now),
		metric.WithAttributes(base...))
}

// observeAlerts emits cardinal_alert_active gauges for synthetic alerts that
// fire during a profile. Without a profile this emits nothing.
func observeAlerts(o metric.Observer, ins *instruments, sc *Scenario, _ *Catalog, _ time.Time) {
	id := sc.ActiveProfileID()
	if id == "" {
		return
	}
	switch id {
	case ProfileDatastoreInfra:
		o.ObserveFloat64(ins.alertActive, 1,
			metric.WithAttributes(
				attribute.String("alert_name", "VMware datastore write latency high"),
				attribute.String("alert_severity", "high"),
				attribute.String("affected_entity_type", "datastore"),
				attribute.String("affected_entity_id", "datastore-202"),
				attribute.String("suspected_layer", "vmware_datastore"),
			))
		o.ObserveFloat64(ins.alertActive, 1,
			metric.WithAttributes(
				attribute.String("alert_name", "Tenant PostgreSQL latency SLO burn"),
				attribute.String("alert_severity", "critical"),
				attribute.String("tenant_id", "tenant_bajaj_finance"),
				attribute.String("customer_id", "bajaj_finance"),
				attribute.String("affected_entity_type", "pg_instance"),
				attribute.String("affected_entity_id", "pg-bajaj-01"),
				attribute.String("suspected_layer", "vmware_datastore"),
			))
	default:
		o.ObserveFloat64(ins.alertActive, 1,
			metric.WithAttributes(
				attribute.String("alert_name", "Tenant PostgreSQL latency SLO burn"),
				attribute.String("alert_severity", "critical"),
				attribute.String("tenant_id", "tenant_bajaj_finance"),
				attribute.String("customer_id", "bajaj_finance"),
				attribute.String("affected_entity_type", "pg_instance"),
				attribute.String("affected_entity_id", "pg-bajaj-01"),
				attribute.String("suspected_layer", "linux_vm"),
			))
	}
}

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// registerAll creates every async instrument. Returned errors are wrapped so
// the caller knows which metric failed to register.
func registerAll(m metric.Meter, ins *instruments) error {
	var err error
	g := func(name, desc string) metric.Float64ObservableGauge {
		if err != nil {
			return nil
		}
		var x metric.Float64ObservableGauge
		x, err = m.Float64ObservableGauge(name, metric.WithDescription(desc))
		return x
	}
	c := func(name, desc string) metric.Float64ObservableCounter {
		if err != nil {
			return nil
		}
		var x metric.Float64ObservableCounter
		x, err = m.Float64ObservableCounter(name, metric.WithDescription(desc))
		return x
	}

	ins.tenantSLOBurn = g(MetricTenantSLOBurnRate, "Tenant SLO error budget burn rate")
	ins.tenantSLOCompliance = g(MetricTenantSLOCompliance, "Tenant SLO compliance ratio")
	ins.tenantSLOErrorBudget = g(MetricTenantSLOErrorBudget, "Tenant SLO error budget remaining ratio")

	ins.pgUp = g(MetricPGUp, "1 when the PostgreSQL instance is accepting connections")
	ins.pgNumBackends = g(MetricPGNumBackends, "Active backend connections per database")
	ins.pgXactCommit = c(MetricPGXactCommitTotal, "Transactions committed")
	ins.pgXactRollback = c(MetricPGXactRollbackTotal, "Transactions rolled back")
	ins.pgTupFetched = c(MetricPGTupFetchedTotal, "Tuples fetched")
	ins.pgBlksRead = c(MetricPGBlksReadTotal, "Disk blocks read")
	ins.pgBlksHit = c(MetricPGBlksHitTotal, "Disk blocks served from cache")
	ins.pgCacheHitRatio = g(MetricPGCacheHitRatio, "Buffer cache hit ratio")
	ins.pgLatencyBucket = c(MetricPGQueryLatencyBucket, "Query latency histogram (cumulative)")
	ins.pgLatencyCount = c(MetricPGQueryLatencyCount, "Query latency histogram count")
	ins.pgLatencySum = c(MetricPGQueryLatencySum, "Query latency histogram sum")
	ins.pgLatencyP95 = g(MetricPGQueryLatencyP95Ms, "Query latency p95 in milliseconds")
	ins.pgLocksCount = g(MetricPGLocksCount, "Open locks per mode")
	ins.pgActivityCount = g(MetricPGActivityCount, "Session count by state and wait_event")
	ins.pgCheckpointWrite = c(MetricPGCheckpointWriteRate, "Cumulative checkpoint write time")
	ins.pgCheckpointSync = c(MetricPGCheckpointSyncRate, "Cumulative checkpoint sync time")
	ins.pgWalBytes = c(MetricPGWalBytesTotal, "WAL bytes generated")
	ins.pgReplicationLag = g(MetricPGReplicationLag, "Replication lag in seconds")

	ins.nodeCPUSeconds = c(MetricNodeCPUSecondsTotal, "node_exporter CPU seconds counter by mode")
	ins.nodeIOWaitPct = g(MetricNodeIOWaitPct, "CPU iowait percent")
	ins.nodeCPUStealPct = g(MetricNodeCPUStealPct, "CPU steal percent")
	ins.nodeMemAvailable = g(MetricNodeMemAvailable, "node_exporter MemAvailable bytes")
	ins.nodeDiskReadTime = c(MetricNodeDiskReadTime, "Cumulative disk read time")
	ins.nodeDiskWriteTime = c(MetricNodeDiskWriteTime, "Cumulative disk write time")
	ins.nodeDiskIOTime = c(MetricNodeDiskIOTime, "Cumulative disk IO time")
	ins.nodeDiskIONow = g(MetricNodeDiskIONow, "Outstanding IO operations")
	ins.nodeFsAvail = g(MetricNodeFilesystemAvail, "Filesystem available bytes")
	ins.nodeNetRxDrop = c(MetricNodeNetRxDrop, "Receive drops")
	ins.nodeNetTxDrop = c(MetricNodeNetTxDrop, "Transmit drops")
	ins.nodeLoad1 = g(MetricNodeLoad1, "1-minute load average")
	ins.nodeContextSwitches = c(MetricNodeContextSwitches, "Cumulative context switches")
	ins.nodePSwapIn = c(MetricNodePSwapInRate, "Cumulative pages swapped in")
	ins.nodePSwapOut = c(MetricNodePSwapOutRate, "Cumulative pages swapped out")
	ins.nodeMemPressure = c(MetricNodeMemPressureRate, "Memory pressure waiting seconds")

	ins.vmwareVMPowerState = g(MetricVMwareVMPowerState, "VM power state (0/1/2)")
	ins.vmwareVMCPUUsagePct = g(MetricVMwareVMCPUUsagePct, "VM CPU usage percent")
	ins.vmwareVMCPUReadyMs = g(MetricVMwareVMCPUReadyMs, "VM CPU ready summation milliseconds")
	ins.vmwareVMMemUsagePct = g(MetricVMwareVMMemUsagePct, "VM memory usage percent")
	ins.vmwareVMMemBallooned = g(MetricVMwareVMMemBallooned, "VM memory ballooned bytes")
	ins.vmwareVMDiskReadLat = g(MetricVMwareVMDiskReadLat, "VM virtual disk read latency ms")
	ins.vmwareVMDiskWriteLat = g(MetricVMwareVMDiskWriteLat, "VM virtual disk write latency ms")
	ins.vmwareVMDiskUsage = g(MetricVMwareVMDiskUsage, "VM disk usage bytes")
	ins.vmwareVMNetRx = c(MetricVMwareVMNetRxBytes, "VM NIC received bytes")
	ins.vmwareVMNetTx = c(MetricVMwareVMNetTxBytes, "VM NIC transmitted bytes")

	ins.vmwareHostCPUUsagePct = g(MetricVMwareHostCPUUsagePct, "ESXi host CPU usage percent")
	ins.vmwareHostMemUsagePct = g(MetricVMwareHostMemUsagePct, "ESXi host memory usage percent")
	ins.vmwareHostDiskReadLat = g(MetricVMwareHostDiskReadLat, "ESXi host disk read latency ms")
	ins.vmwareHostDiskWriteLat = g(MetricVMwareHostDiskWriteLat, "ESXi host disk write latency ms")
	ins.vmwareHostNetRxDropped = c(MetricVMwareHostNetRxDropped, "ESXi host RX drops")
	ins.vmwareHostNetTxDropped = c(MetricVMwareHostNetTxDropped, "ESXi host TX drops")
	ins.vmwareHostVMCount = g(MetricVMwareHostVMCount, "ESXi host VM count")

	ins.dsCapacity = g(MetricDSCapacity, "Datastore capacity bytes")
	ins.dsFreeBytes = g(MetricDSFreeBytes, "Datastore free bytes")
	ins.dsReadLat = g(MetricDSReadLat, "Datastore read latency ms")
	ins.dsWriteLat = g(MetricDSWriteLat, "Datastore write latency ms")
	ins.dsIOPSRead = g(MetricDSIOPSRead, "Datastore read IOPS")
	ins.dsIOPSWrite = g(MetricDSIOPSWrite, "Datastore write IOPS")
	ins.dsQueueDepth = g(MetricDSQueueDepth, "Datastore queue depth")

	ins.probeSuccessRate = g(MetricAirtelProbeSuccessRate, "Airtel synthetic probe success ratio")
	ins.probeLatencyMs = g(MetricAirtelProbeLatencyMs, "Airtel synthetic probe latency ms")

	ins.alertActive = g(MetricCardinalAlertActive, "Active synthetic alert state")

	if err != nil {
		return fmt.Errorf("create dbaas instruments: %w", err)
	}
	return nil
}
