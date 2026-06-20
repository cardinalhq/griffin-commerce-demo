// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

package dbaas

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// logs.go emits the spec §16–21 JSON-shaped events as OTLP log records via
// the global logger provider (already wired by common.SetupTelemetry).
// Each tick (default 5s) walks the catalog and samples a Poisson count of
// events per (entity, event_type) based on baseline/incident frequencies.

const (
	defaultLogIntervalSeconds = 5
)

func logIntervalSeconds() int {
	if v := os.Getenv("DBAAS_LOG_INTERVAL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultLogIntervalSeconds
}

// StartLogEmitter launches a goroutine that emits log events on the
// configured cadence until ctx is cancelled.
func StartLogEmitter(ctx context.Context, catalog *Catalog, scenario *Scenario) {
	logger := global.GetLoggerProvider().Logger(instrumentScope)
	interval := time.Duration(logIntervalSeconds()) * time.Second
	rng := rand.New(rand.NewSource(0xa17e1 ^ time.Now().UnixNano()))
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				emitTick(ctx, logger, catalog, scenario, now, interval, rng)
			}
		}
	}()
}

func emitTick(ctx context.Context, logger log.Logger, c *Catalog, sc *Scenario, now time.Time, interval time.Duration, rng *rand.Rand) {
	intervalMin := interval.Minutes()
	for _, pg := range c.PGInstances {
		sel := selectorPG(pg.Name)
		incident := sc.IsActiveOn(sel, MetricPGQueryLatencyP95Ms, now) ||
			sc.IsActiveOn(sel, MetricPGCacheHitRatio, now)
		freqSlow := 1.0
		freqCP := 0.2
		freqIO := 0.0
		if incident {
			freqSlow = 25
			freqCP = 2
			freqIO = 5
		}
		emitN(rng, freqSlow*intervalMin, func() { logSlowQuery(ctx, logger, pg, now, incident) })
		emitN(rng, freqCP*intervalMin, func() { logCheckpointSlow(ctx, logger, pg, now, incident) })
		emitN(rng, freqIO*intervalMin, func() { logPGIOWaitHigh(ctx, logger, pg, now) })
	}
	for _, vm := range c.VMs {
		sel := selectorVM(vm.Name)
		ioHigh := sc.IsActiveOn(sel, MetricNodeIOWaitPct, now)
		cpuReady := sc.IsActiveOn(sel, MetricVMwareVMCPUReadyMs, now)
		memHigh := sc.IsActiveOn(sel, MetricNodeMemAvailable, now)
		if ioHigh {
			emitN(rng, 4*intervalMin, func() { logLinuxIOWaitHigh(ctx, logger, vm, now) })
			emitN(rng, 1*intervalMin, func() { logKernelBlockedTask(ctx, logger, vm, now) })
			emitN(rng, 0.6*intervalMin, func() { logZabbixDiskTrigger(ctx, logger, vm, now) })
		}
		if cpuReady {
			emitN(rng, 1.5*intervalMin, func() { logVMCPUReadyHigh(ctx, logger, vm, now) })
		}
		if memHigh {
			emitN(rng, 2*intervalMin, func() { logMemoryPressure(ctx, logger, vm, now) })
		}
	}
	for _, h := range c.Hosts {
		sel := selectorHost(h.ID)
		if sc.IsActiveOn(sel, MetricVMwareHostDiskWriteLat, now) {
			emitN(rng, 1*intervalMin, func() { logHostContention(ctx, logger, h, c, now) })
		}
	}
	for _, d := range c.Datastores {
		sel := selectorDatastore(d.ID)
		if sc.IsActiveOn(sel, MetricDSWriteLat, now) {
			emitN(rng, 1.5*intervalMin, func() { logDatastoreLatencyHigh(ctx, logger, d, now) })
			emitN(rng, 0.4*intervalMin, func() { logZabbixDatastoreTrigger(ctx, logger, d, now) })
		}
	}
	for _, t := range c.Tenants {
		sel := selectorTenant(t.TenantID)
		if sc.IsActiveOn(sel, MetricAirtelProbeLatencyMs, now) {
			emitN(rng, 1*intervalMin, func() { logProbeDegraded(ctx, logger, t, c, now) })
			emitN(rng, 0.3*intervalMin, func() { logZabbixPGTrigger(ctx, logger, t, c, now) })
		}
	}
	// One correlation log per profile per minute helps the demo UI's
	// timeline panel converge to the same join story.
	if sc.ActiveProfileID() != "" {
		emitN(rng, 0.5*intervalMin, func() { logCorrelationDiscovered(ctx, logger, c, now) })
	}
}

// emitN samples a Poisson count from lambda and invokes fn that many times.
// For demo realism this is bounded to 50 emissions per tick per event.
func emitN(rng *rand.Rand, lambda float64, fn func()) {
	if lambda <= 0 {
		return
	}
	n := poisson(rng, lambda)
	if n > 50 {
		n = 50
	}
	for i := 0; i < n; i++ {
		fn()
	}
}

func poisson(rng *rand.Rand, lambda float64) int {
	if lambda < 30 {
		L := math.Exp(-lambda)
		p := 1.0
		k := 0
		for {
			k++
			p *= rng.Float64()
			if p <= L {
				return k - 1
			}
		}
	}
	// Normal approximation for larger lambda.
	return int(math.Round(lambda + rng.NormFloat64()*math.Sqrt(lambda)))
}

// severity helpers — OTel SDK Severity enum doesn't carry DISASTER/CRITICAL;
// stash the original token as severity_text for downstream filtering.
func sevWarn() log.Severity     { return log.SeverityWarn }
func sevError() log.Severity    { return log.SeverityError }
func sevInfo() log.Severity     { return log.SeverityInfo }
func sevCritical() log.Severity { return log.SeverityFatal }
func sevHigh() log.Severity     { return log.SeverityError2 }

func emitRecord(ctx context.Context, logger log.Logger, sev log.Severity, sevText, body string, attrs ...log.KeyValue) {
	rec := log.Record{}
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(sev)
	rec.SetSeverityText(sevText)
	rec.SetBody(log.StringValue(body))
	rec.AddAttributes(attrs...)
	logger.Emit(ctx, rec)
}

// -- event constructors --

func logSlowQuery(ctx context.Context, logger log.Logger, pg *PGInstance, now time.Time, incident bool) {
	dur := 320 + rand.Float64()*180 // 320–500ms baseline
	if incident {
		dur = 600 + rand.Float64()*900
	}
	msg := fmt.Sprintf("duration: %.3f ms execute stmt_%d: SELECT account_id, balance FROM accounts WHERE customer_ref = $1",
		dur, rand.Intn(99999))
	attrs := []log.KeyValue{
		log.String("source", "postgres"),
		log.String("event_type", "postgres_slow_query"),
		log.String("severity_text", "WARN"),
		log.Float64("duration_ms", dur),
		log.String("query_class", "oltp_read"),
	}
	attrs = append(attrs, pgLogAttrs(pg)...)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logCheckpointSlow(ctx context.Context, logger log.Logger, pg *PGInstance, now time.Time, incident bool) {
	write := 4.0 + rand.Float64()*8.0
	sync := 0.5 + rand.Float64()*2.0
	if incident {
		write = 18 + rand.Float64()*32
		sync = 4 + rand.Float64()*10
	}
	total := write + sync + 6
	msg := fmt.Sprintf("checkpoint complete: wrote %d buffers (%.1f%%); write=%.3f s, sync=%.3f s, total=%.3f s",
		1500+rand.Intn(20000), 5+rand.Float64()*20, write, sync, total)
	attrs := append(pgLogAttrs(pg),
		log.String("source", "postgres"),
		log.String("event_type", "postgres_checkpoint_slow"),
		log.String("severity_text", "WARN"),
		log.Float64("checkpoint_write_seconds", write),
		log.Float64("checkpoint_sync_seconds", sync),
		log.Float64("checkpoint_total_seconds", total),
	)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logPGIOWaitHigh(ctx context.Context, logger log.Logger, pg *PGInstance, now time.Time) {
	waiting := 40 + rand.Intn(120)
	msg := fmt.Sprintf("active sessions waiting on IO: wait_event_type=IO wait_event=DataFileWrite count=%d", waiting)
	attrs := append(pgLogAttrs(pg),
		log.String("source", "postgres"),
		log.String("event_type", "postgres_io_wait_high"),
		log.String("severity_text", "WARN"),
		log.String("wait_event_type", "IO"),
		log.String("wait_event", "DataFileWrite"),
		log.Int("waiting_sessions", waiting),
	)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logLinuxIOWaitHigh(ctx context.Context, logger log.Logger, vm *VM, now time.Time) {
	await := 120 + rand.Float64()*120
	queue := 30 + rand.Float64()*30
	util := 92 + rand.Float64()*7
	msg := fmt.Sprintf("high iowait detected: device=sda await=%.1fms avgqu-sz=%.1f util=%.1f%%", await, queue, util)
	attrs := append(vmLogAttrs(vm),
		log.String("source", "linux"),
		log.String("event_type", "linux_io_wait_high"),
		log.String("severity_text", "WARN"),
		log.String("device", "sda"),
		log.Float64("await_ms", await),
		log.Float64("avgqu_sz", queue),
		log.Float64("util_percent", util),
	)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logKernelBlockedTask(ctx context.Context, logger log.Logger, vm *VM, now time.Time) {
	pid := 18000 + rand.Intn(2000)
	msg := fmt.Sprintf("task postgres:%d blocked for more than 120 seconds waiting on IO", pid)
	attrs := append(vmLogAttrs(vm),
		log.String("source", "linux_kernel"),
		log.String("event_type", "linux_kernel_blocked_task"),
		log.String("severity_text", "WARN"),
		log.String("device", "sda"),
		log.Int("pid", pid),
	)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logMemoryPressure(ctx context.Context, logger log.Logger, vm *VM, now time.Time) {
	msg := "kswapd activity high; postgres process experiencing reclaim stalls"
	attrs := append(vmLogAttrs(vm),
		log.String("source", "linux_kernel"),
		log.String("event_type", "linux_memory_pressure"),
		log.String("severity_text", "WARN"),
	)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logVMCPUReadyHigh(ctx context.Context, logger log.Logger, vm *VM, now time.Time) {
	cpuReady := 5000 + rand.Float64()*8000
	msg := fmt.Sprintf("VM CPU ready time high for %s: cpu_ready_summation_ms=%.0f", vm.Name, cpuReady)
	attrs := append(vmLogAttrs(vm),
		log.String("source", "vcenter"),
		log.String("event_type", "vmware_vm_cpu_ready_high"),
		log.String("severity_text", "WARN"),
		log.Float64("cpu_ready_summation_ms", cpuReady),
	)
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logDatastoreLatencyHigh(ctx context.Context, logger log.Logger, d *Datastore, now time.Time) {
	write := 120 + rand.Float64()*140
	read := 50 + rand.Float64()*90
	qd := 100 + rand.Intn(200)
	msg := fmt.Sprintf("Datastore %s device latency exceeded threshold: write_latency_ms=%.0f read_latency_ms=%.0f queue_depth=%d",
		d.Name, write, read, qd)
	attrs := []log.KeyValue{
		log.String("source", "vmware"),
		log.String("event_type", "vmware_datastore_latency_high"),
		log.String("severity_text", "WARN"),
		log.String("vcenter_name", vcenterName),
		log.String("datacenter", datacenterDelhi),
		log.String("datastore_name", d.Name),
		log.String("datastore_id", d.ID),
		log.String("datastore_type", d.Type),
		log.String("storage_array", d.StorageArray),
		log.String("service_tier", d.ServiceTier),
		log.Float64("write_latency_ms", write),
		log.Float64("read_latency_ms", read),
		log.Int("queue_depth", qd),
	}
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logHostContention(ctx context.Context, logger log.Logger, h *Host, c *Catalog, now time.Time) {
	vms := c.VMsOnHost(h.ID)
	tenants := map[string]bool{}
	for _, v := range vms {
		if v.Tenant != nil {
			tenants[v.Tenant.TenantID] = true
		}
	}
	msg := fmt.Sprintf("Host %s showing high CPU ready and datastore latency across multiple VMs", h.Name)
	attrs := []log.KeyValue{
		log.String("source", "vmware"),
		log.String("event_type", "vmware_host_contention"),
		log.String("severity_text", "WARN"),
		log.String("vcenter_name", vcenterName),
		log.String("cluster", h.Cluster.Name),
		log.String("esxi_host_name", h.Name),
		log.String("esxi_host_id", h.ID),
		log.Int("affected_vm_count", len(vms)),
		log.Int("affected_tenant_count", len(tenants)),
		log.String("primary_datastore_name", "ds-gold-delhi-02"),
	}
	emitRecord(ctx, logger, sevWarn(), "WARN", msg, attrs...)
	_ = now
}

func logZabbixDiskTrigger(ctx context.Context, logger log.Logger, vm *VM, now time.Time) {
	msg := fmt.Sprintf("Trigger fired: High disk write latency on %s", vm.Name)
	attrs := append(vmLogAttrs(vm),
		log.String("source", "zabbix"),
		log.String("event_type", "zabbix_trigger_fired"),
		log.String("severity_text", "HIGH"),
		log.String("zabbix_event_id", fmt.Sprintf("zbx-%d", 900000+rand.Intn(999))),
		log.String("zabbix_host", vm.Name),
		log.String("zabbix_hostgroup", "Airtel Cloud/Managed DB/PostgreSQL"),
		log.String("zabbix_item_key", "vfs.dev.write.await[sda]"),
		log.String("trigger_name", "High disk write latency"),
		log.String("trigger_priority", "HIGH"),
	)
	emitRecord(ctx, logger, sevHigh(), "HIGH", msg, attrs...)
	_ = now
}

func logZabbixDatastoreTrigger(ctx context.Context, logger log.Logger, d *Datastore, now time.Time) {
	msg := fmt.Sprintf("Trigger fired: VMware datastore %s write latency is high", d.Name)
	attrs := []log.KeyValue{
		log.String("source", "zabbix"),
		log.String("event_type", "zabbix_trigger_fired"),
		log.String("severity_text", "HIGH"),
		log.String("zabbix_event_id", fmt.Sprintf("zbx-%d", 950000+rand.Intn(999))),
		log.String("zabbix_host", vcenterName),
		log.String("zabbix_hostgroup", "Airtel Cloud/VMware/vCenter"),
		log.String("zabbix_template", "Template VM VMware Hypervisor"),
		log.String("zabbix_item_key", fmt.Sprintf("vmware.datastore.write_latency[%s]", d.Name)),
		log.String("trigger_name", "VMware datastore write latency is high"),
		log.String("trigger_priority", "HIGH"),
		log.String("vcenter_name", vcenterName),
		log.String("datastore_name", d.Name),
		log.String("datastore_id", d.ID),
	}
	emitRecord(ctx, logger, sevHigh(), "HIGH", msg, attrs...)
	_ = now
}

func logZabbixPGTrigger(ctx context.Context, logger log.Logger, t *Tenant, c *Catalog, now time.Time) {
	// Find a PG instance for this tenant for context.
	var pg *PGInstance
	for _, p := range c.PGInstances {
		if p.Tenant != nil && p.Tenant.TenantID == t.TenantID {
			pg = p
			break
		}
	}
	pgName := "pg-unknown"
	vmName := "unknown-vm"
	if pg != nil {
		pgName = pg.Name
		if pg.VM != nil {
			vmName = pg.VM.Name
		}
	}
	msg := fmt.Sprintf("Trigger fired: PostgreSQL response time is too high on %s", pgName)
	attrs := []log.KeyValue{
		log.String("source", "zabbix"),
		log.String("event_type", "zabbix_trigger_fired"),
		log.String("severity_text", "DISASTER"),
		log.String("zabbix_event_id", fmt.Sprintf("zbx-%d", 980000+rand.Intn(999))),
		log.String("zabbix_host", vmName),
		log.String("zabbix_hostgroup", "Airtel Cloud/Managed DB/PostgreSQL"),
		log.String("zabbix_template", "Template DB PostgreSQL by ODBC"),
		log.String("zabbix_item_key", fmt.Sprintf("pgsql.ping.time[%s]", pgName)),
		log.String("trigger_name", "PostgreSQL response time is too high"),
		log.String("trigger_priority", "DISASTER"),
		log.String("tenant_id", t.TenantID),
		log.String("customer_id", t.CustomerID),
		log.String("pg_instance", pgName),
		log.String("vm_name", vmName),
	}
	emitRecord(ctx, logger, sevCritical(), "DISASTER", msg, attrs...)
	_ = now
}

func logProbeDegraded(ctx context.Context, logger log.Logger, t *Tenant, c *Catalog, now time.Time) {
	var pg *PGInstance
	for _, p := range c.PGInstances {
		if p.Tenant != nil && p.Tenant.TenantID == t.TenantID {
			pg = p
			break
		}
	}
	pgName, vmName := "pg-unknown", "unknown-vm"
	if pg != nil {
		pgName = pg.Name
		if pg.VM != nil {
			vmName = pg.VM.Name
		}
	}
	latency := 600 + rand.Float64()*500
	success := rand.Float64() > 0.3
	msg := fmt.Sprintf("PostgreSQL probe degraded for %s: simple query latency %.0fms, success=%v", t.TenantID, latency, success)
	attrs := []log.KeyValue{
		log.String("source", "airtel_probe"),
		log.String("event_type", "tenant_probe_degraded"),
		log.String("severity_text", "ERROR"),
		log.String("probe_name", "postgres_tcp_connect_and_simple_query"),
		log.String("target_service", "postgres"),
		log.Float64("probe_latency_ms", latency),
		log.Bool("success", success),
		log.String("tenant_id", t.TenantID),
		log.String("customer_id", t.CustomerID),
		log.String("pg_instance", pgName),
		log.String("vm_name", vmName),
		log.String("region", t.Region),
		log.String("az", t.AZ),
	}
	emitRecord(ctx, logger, sevError(), "ERROR", msg, attrs...)
	_ = now
}

func logCorrelationDiscovered(ctx context.Context, logger log.Logger, c *Catalog, now time.Time) {
	// Find the primary impacted PG instance for the correlation story.
	var primary *PGInstance
	for _, p := range c.PGInstances {
		if p.IncidentRole == rolePrimaryPG {
			primary = p
			break
		}
	}
	if primary == nil {
		return
	}
	msg := "Correlation path discovered: pg_query_latency_p95_ms -> vmware_vm_disk_write_latency_ms -> vmware_datastore_write_latency_ms"
	attrs := []log.KeyValue{
		log.String("source", "cardinal"),
		log.String("event_type", "correlation_path_discovered"),
		log.String("severity_text", "INFO"),
		log.String("selected_signal", "pg_query_latency_p95_ms"),
		log.String("selected_entity_type", "pg_instance"),
		log.String("selected_entity_id", primary.Name),
		log.String("tenant_id", primary.Tenant.TenantID),
		log.String("customer_id", primary.Tenant.CustomerID),
		log.String("pg_instance", primary.Name),
		log.Float64("confidence", 0.92+rand.Float64()*0.05),
	}
	if primary.VM != nil {
		attrs = append(attrs, log.String("vm_name", primary.VM.Name), log.String("vm_uuid", primary.VM.UUID))
		if primary.VM.Datastore != nil {
			attrs = append(attrs, log.String("datastore_name", primary.VM.Datastore.Name))
		}
	}
	emitRecord(ctx, logger, sevInfo(), "INFO", msg, attrs...)
	_ = now
}

func pgLogAttrs(pg *PGInstance) []log.KeyValue {
	out := []log.KeyValue{
		log.String("pg_instance", pg.Name),
		log.String("pg_cluster", pg.Cluster),
		log.String("database_name", pg.Database),
	}
	if pg.Tenant != nil {
		out = append(out,
			log.String("tenant_id", pg.Tenant.TenantID),
			log.String("customer_id", pg.Tenant.CustomerID),
		)
	}
	if pg.VM != nil {
		out = append(out,
			log.String("vm_name", pg.VM.Name),
			log.String("vm_uuid", pg.VM.UUID),
		)
		if pg.VM.Host != nil {
			out = append(out, log.String("esxi_host_name", pg.VM.Host.Name))
		}
		if pg.VM.Datastore != nil {
			out = append(out, log.String("datastore_name", pg.VM.Datastore.Name))
		}
	}
	return out
}

func vmLogAttrs(vm *VM) []log.KeyValue {
	out := []log.KeyValue{
		log.String("vm_name", vm.Name),
		log.String("vm_uuid", vm.UUID),
		log.String("workload_role", vm.WorkloadRole),
	}
	if vm.Tenant != nil {
		out = append(out,
			log.String("tenant_id", vm.Tenant.TenantID),
			log.String("customer_id", vm.Tenant.CustomerID),
		)
	}
	if vm.Host != nil {
		out = append(out,
			log.String("esxi_host_name", vm.Host.Name),
			log.String("esxi_host_id", vm.Host.ID),
		)
	}
	if vm.Datastore != nil {
		out = append(out,
			log.String("datastore_name", vm.Datastore.Name),
			log.String("datastore_id", vm.Datastore.ID),
		)
	}
	return out
}
