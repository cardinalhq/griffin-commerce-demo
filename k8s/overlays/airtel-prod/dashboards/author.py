#!/usr/bin/env python3
"""
Authors the four Airtel demo dashboards against maestro.

Targets the Airtel org (3aa7b421-0ecb-48a8-bf3a-b7397814862a) in
prod-us-east-2-global. Maestro is reached via a port-forward to
:4200 with X-CardinalHQ-API-Key from the mcp-gateway-creds Secret.

Dashboards follow the spec §24 catalog:
  1. Tenant Health Overview         (§24.1)
  2. Tenant PostgreSQL Detail       (§24.2)
  3. VMware Infrastructure          (§24.3 + §24.4)
  4. Correlation & Blast Radius     (§24.5)

Schema reference: /tmp/dbaas-tenant-view.json (existing 'DBaaS — Tenant
View' dashboard in the Cardinal HQ org). 24-column grid; cells carry
{x,y,w,h,i}. Panels are 'label' (single-stat tile) or 'timeseries'
(variant: line | stacked-bar | stacked-area).
"""
import json
import os
import sys
import urllib.request
import urllib.error

# Cardinal HQ — the org the prod aws-prod-us-east-2-global node-local
# collector routes to. The Airtel org (3aa7b421-…) was the intuitive
# target but the collector isn't wired to it; until that changes, the
# telemetry surfaces in Cardinal HQ.
ORG_ID = "c4375e34-dfcf-498a-8ba3-a02d119baf82"
MAESTRO_URL = "http://localhost:4200"

# ---------- helpers ----------

def label_panel(pid, title, query, color="#3b82f6", unit="", reducer="last"):
    p = {
        "id": pid, "kind": "label", "title": title, "color": color,
        "queries": [{"query": query, "queryKind": "prometheus"}],
        "reducer": reducer,
    }
    if unit:
        p["format"] = {"unit": unit}
    return p

def ts_panel(pid, title, queries, variant="line"):
    if isinstance(queries, str):
        queries = [{"query": queries, "queryKind": "prometheus"}]
    elif queries and isinstance(queries[0], str):
        queries = [{"query": q, "queryKind": "prometheus"} for q in queries]
    return {
        "id": pid, "kind": "timeseries", "title": title,
        "queries": queries, "variant": variant,
    }

def row(*cells, y=0):
    """Lay cells side by side at row y. cells = [(pid, w, h), ...]"""
    out = []
    x = 0
    maxh = 0
    for pid, w, h in cells:
        out.append({"i": pid, "x": x, "y": y, "w": w, "h": h})
        x += w
        maxh = max(maxh, h)
    return out, y + maxh

def grid(*rows_):
    """Stack rows of cells, return (cells, total_height)."""
    cells = []
    y = 0
    for r in rows_:
        for pid, w, h in r:
            cells.append({"i": pid, "x": sum(rr[1] for rr in r[:r.index((pid, w, h))]),
                          "y": y, "w": w, "h": h})
        y += max(h for _, _, h in r)
    return cells

# Simpler row layout: pass [(pid, w, h), ...] gets placed sequentially left to right at given y
def row_at(y, *cells):
    out = []
    x = 0
    for pid, w, h in cells:
        out.append({"i": pid, "x": x, "y": y, "w": w, "h": h})
        x += w
    return out

# ---------- variables ----------

VAR_TENANT = {
    "kind": "query", "name": "tenant_id", "label": "Tenant",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "tenant_id", "metric": "tenant_slo_burn_rate", "signal": "metrics"},
    "defaultValue": ["tenant_bajaj_finance"],
}

VAR_PG = {
    "kind": "query", "name": "pg_instance", "label": "PostgreSQL Instance",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "pg_instance", "metric": "pg_query_latency_p95_ms", "signal": "metrics"},
    "defaultValue": ["pg-bajaj-01"],
}

VAR_VM = {
    "kind": "query", "name": "vm_name", "label": "VM",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "vm_name", "metric": "vmware_vm_disk_write_latency_ms", "signal": "metrics"},
    "defaultValue": ["vm-bajaj-pg-01"],
}

VAR_DATASTORE = {
    "kind": "query", "name": "datastore_id", "label": "Datastore",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "datastore_id", "metric": "vmware_datastore_write_latency_ms", "signal": "metrics"},
    "defaultValue": ["datastore-202"],
}

# ============================================================
# 1. Tenant Health Overview — spec §24.1
# ============================================================

def dashboard_tenant_health():
    """Tenant fleet ops view — what an Airtel managed-DB ops engineer
    pulls up every morning. Should read as steady-state fleet health
    most of the time, with the SLO breach narrative only one of many
    signals on the page."""
    panels = {}

    # Fleet KPI strip (4 tiles) — total fleet posture.
    # Use last_over_time(...[5m]) so the count is robust to a single
    # missed scrape. Use `or on() vector(0)` on the filtered counts so
    # zero-matches render "0" instead of "No data".
    panels["th_kpi_tenants"]  = label_panel("th_kpi_tenants", "Tenants",
        'count(group by (tenant_id)(last_over_time(tenant_slo_burn_rate[5m])))',
        color="#3b82f6")
    panels["th_kpi_dbs"]      = label_panel("th_kpi_dbs", "DB Instances",
        'count(group by (pg_instance)(last_over_time(pg_up[5m])))',
        color="#3b82f6")
    panels["th_kpi_breach"]   = label_panel("th_kpi_breach", "Tenants Breaching SLO",
        '(count(last_over_time(tenant_slo_burn_rate[5m]) > 10) or on() vector(0))',
        color="#ef4444")
    panels["th_kpi_atrisk"]   = label_panel("th_kpi_atrisk", "Tenants At Risk",
        '(count(last_over_time(tenant_slo_burn_rate[5m]) > 2 and last_over_time(tenant_slo_burn_rate[5m]) <= 10) or on() vector(0))',
        color="#f59e0b")

    # Selected tenant deep tile row (4) — drives the rest of the page
    panels["th_burn"]   = label_panel("th_burn",   "SLO Burn Rate",
        'max(tenant_slo_burn_rate{tenant_id="$tenant_id"})', color="#ef4444", unit="x")
    panels["th_comp"]   = label_panel("th_comp",   "SLO Compliance",
        'min(tenant_slo_compliance_ratio{tenant_id="$tenant_id"}) * 100', color="#10b981", unit="%")
    panels["th_budget"] = label_panel("th_budget", "Error Budget Remaining",
        'min(tenant_slo_error_budget_remaining_ratio{tenant_id="$tenant_id"}) * 100', color="#f59e0b", unit="%")
    panels["th_probe"]  = label_panel("th_probe",  "Probe Success",
        'min(airtel_probe_success_ratio{tenant_id="$tenant_id"}) * 100', color="#10b981", unit="%")

    # Fleet SLO trend (2) — the wall every morning
    panels["th_burn_all"] = ts_panel("th_burn_all", "Burn Rate by Tenant",
        'max by (tenant_id)(tenant_slo_burn_rate)')
    panels["th_budget_all"] = ts_panel("th_budget_all", "Error Budget Remaining by Tenant",
        'min by (tenant_id)(tenant_slo_error_budget_remaining_ratio)')

    # Tier rollup (2) — gold vs silver
    panels["th_burn_tier"] = ts_panel("th_burn_tier", "Avg Burn Rate by Service Tier",
        'avg by (service_tier)(tenant_slo_burn_rate)')
    panels["th_comp_tier"] = ts_panel("th_comp_tier", "Avg Compliance Ratio by Service Tier",
        'avg by (service_tier)(tenant_slo_compliance_ratio)')

    # Region / AZ rollup (2) — Airtel's perspective
    panels["th_burn_az"]  = ts_panel("th_burn_az", "Max Burn Rate by AZ",
        'max by (az)(tenant_slo_burn_rate)')
    panels["th_probe_az"] = ts_panel("th_probe_az", "Min Probe Success by AZ",
        'min by (az)(airtel_probe_success_ratio)')

    # Application latency (3)
    panels["th_pg_p95"]    = ts_panel("th_pg_p95", "PostgreSQL p95 Latency by Tenant (ms)",
        'max by (tenant_id)(pg_query_latency_p95_ms)')
    panels["th_probe_lat"] = ts_panel("th_probe_lat", "Probe Latency by Tenant (ms)",
        'max by (tenant_id)(airtel_probe_latency_ms)')
    panels["th_probe_succ_all"] = ts_panel("th_probe_succ_all", "Probe Success Ratio by Tenant",
        'min by (tenant_id)(airtel_probe_success_ratio)')

    # Fleet workload (3) — proves the fleet is doing real work
    panels["th_commit_rate"] = ts_panel("th_commit_rate", "Commit Rate by DB (tx/s)",
        'rate(pg_stat_database_xact_commit_total[5m])')
    panels["th_backends"]    = ts_panel("th_backends", "Active Backends by DB",
        'max by (pg_instance)(pg_stat_database_numbackends)')
    panels["th_cache_all"]   = ts_panel("th_cache_all", "Buffer Cache Hit Ratio by DB",
        'min by (pg_instance)(pg_database_cache_hit_ratio)')

    # Shared infrastructure rollup (3)
    panels["th_ds_lat"]    = ts_panel("th_ds_lat", "Datastore Write Latency (ms)",
        'max by (datastore_name)(vmware_datastore_write_latency_ms)')
    panels["th_host_lat"]  = ts_panel("th_host_lat", "ESXi Host Disk Write Latency (ms)",
        'max by (esxi_host_name)(vmware_host_disk_write_latency_ms)')
    panels["th_host_cpu"]  = ts_panel("th_host_cpu", "ESXi Host CPU (%)",
        'max by (esxi_host_name)(vmware_host_cpu_usage_percent)')

    # Active alerts (1)
    panels["th_alerts"] = ts_panel("th_alerts", "Active Alerts (cardinal_alert_active)",
        'max by (alert_name, alert_severity, affected_entity_id, suspected_layer)(cardinal_alert_active)',
        variant="stacked-bar")

    sections = [
        {"title": "Fleet Posture", "cells": row_at(0,
            ("th_kpi_tenants", 6, 3), ("th_kpi_dbs", 6, 3), ("th_kpi_breach", 6, 3), ("th_kpi_atrisk", 6, 3))},
        {"title": "Selected Tenant — SLO Snapshot", "cells": row_at(0,
            ("th_burn", 6, 3), ("th_comp", 6, 3), ("th_budget", 6, 3), ("th_probe", 6, 3))},
        {"title": "Fleet SLO Trends", "cells": row_at(0,
            ("th_burn_all", 12, 7), ("th_budget_all", 12, 7))},
        {"title": "Service Tier Rollup", "cells": row_at(0,
            ("th_burn_tier", 12, 6), ("th_comp_tier", 12, 6))},
        {"title": "Availability Zone Rollup", "cells": row_at(0,
            ("th_burn_az", 12, 6), ("th_probe_az", 12, 6))},
        {"title": "Application-Visible Latency", "cells": row_at(0,
            ("th_pg_p95", 8, 7), ("th_probe_lat", 8, 7), ("th_probe_succ_all", 8, 7))},
        {"title": "Fleet Workload", "cells": row_at(0,
            ("th_commit_rate", 8, 6), ("th_backends", 8, 6), ("th_cache_all", 8, 6))},
        {"title": "Shared Infrastructure", "cells": row_at(0,
            ("th_ds_lat", 8, 6), ("th_host_lat", 8, 6), ("th_host_cpu", 8, 6))},
        {"title": "Active Alerts", "cells": row_at(0,
            ("th_alerts", 24, 6))},
    ]
    return {
        "name": "Airtel — Tenant Health Overview",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_TENANT],
            "panels": panels,
            "sections": sections,
        },
    }

# ============================================================
# 2. Tenant PostgreSQL Detail — spec §24.2
# ============================================================

def dashboard_pg_detail():
    """PG instance-level operations view — what a DBA running a single
    managed Postgres looks at to triage. Mirrors what a real DBA expects
    out of `postgres_exporter` + a node_exporter sidecar."""
    panels = {}

    # Tile strip (6) — instant posture of the selected DB
    panels["pg_p95"]     = label_panel("pg_p95",     "p95 Latency",
        'max(pg_query_latency_p95_ms{pg_instance="$pg_instance"})', color="#ef4444", unit="ms")
    panels["pg_back"]    = label_panel("pg_back",    "Active Backends",
        'max(pg_stat_database_numbackends{pg_instance="$pg_instance"})', color="#3b82f6")
    panels["pg_cache"]   = label_panel("pg_cache",   "Buffer Cache Hit",
        'min(pg_database_cache_hit_ratio{pg_instance="$pg_instance"}) * 100', color="#10b981", unit="%")
    panels["pg_qps"]     = label_panel("pg_qps",     "Commit Rate",
        'sum(rate(pg_stat_database_xact_commit_total{pg_instance="$pg_instance"}[5m]))',
        color="#3b82f6", unit="tx/s")
    panels["pg_wal_rate"] = label_panel("pg_wal_rate", "WAL Throughput",
        'sum(rate(pg_wal_bytes_total{pg_instance="$pg_instance"}[5m])) / 1024 / 1024',
        color="#3b82f6", unit="MB/s")
    panels["pg_repl"]    = label_panel("pg_repl",    "Replication Lag",
        'max(pg_replication_lag_seconds{pg_instance="$pg_instance"})', color="#f59e0b", unit="s")

    # Latency analysis (3) — distribution + per-class p95 + heatmap-ish
    panels["pg_p95_class"] = ts_panel("pg_p95_class", "p95 Latency by Query Class (ms)",
        'max by (query_class)(pg_query_latency_p95_ms{pg_instance="$pg_instance"})')
    panels["pg_p99_hist"]  = ts_panel("pg_p99_hist", "p99 Query Latency from Histogram (s)",
        'histogram_quantile(0.99, sum by (le)(rate(pg_query_latency_seconds_bucket{pg_instance="$pg_instance"}[5m])))')
    panels["pg_lat_buckets"] = ts_panel("pg_lat_buckets", "Latency Bucket Rate by le (req/s)",
        'sum by (le)(rate(pg_query_latency_seconds_bucket{pg_instance="$pg_instance"}[5m]))',
        variant="stacked-area")

    # Throughput (3)
    panels["pg_tx_rate"]   = ts_panel("pg_tx_rate", "Transaction Rate (tx/s)",
        ['sum(rate(pg_stat_database_xact_commit_total{pg_instance="$pg_instance"}[5m]))',
         'sum(rate(pg_stat_database_xact_rollback_total{pg_instance="$pg_instance"}[5m]))'])
    panels["pg_tup_rate"]  = ts_panel("pg_tup_rate", "Tuples Fetched (rows/s)",
        'sum(rate(pg_stat_database_tup_fetched_total{pg_instance="$pg_instance"}[5m]))')
    panels["pg_blk_rate"]  = ts_panel("pg_blk_rate", "Block Reads vs Cache Hits (blocks/s)",
        ['sum(rate(pg_stat_database_blks_read_total{pg_instance="$pg_instance"}[5m]))',
         'sum(rate(pg_stat_database_blks_hit_total{pg_instance="$pg_instance"}[5m]))'])

    # Sessions & wait events (2)
    panels["pg_acts_wet"]  = ts_panel("pg_acts_wet", "Sessions by wait_event_type",
        'sum by (wait_event_type)(pg_stat_activity_count{pg_instance="$pg_instance"})',
        variant="stacked-area")
    panels["pg_acts_state"] = ts_panel("pg_acts_state", "Sessions by state",
        'sum by (state)(pg_stat_activity_count{pg_instance="$pg_instance"})',
        variant="stacked-bar")

    # Checkpoint & WAL (3)
    panels["pg_cp_write"]  = ts_panel("pg_cp_write", "Checkpoint Write Time Rate (s/s)",
        'rate(pg_checkpoint_write_time_seconds_total{pg_instance="$pg_instance"}[5m])')
    panels["pg_cp_sync"]   = ts_panel("pg_cp_sync", "Checkpoint Sync Time Rate (s/s)",
        'rate(pg_checkpoint_sync_time_seconds_total{pg_instance="$pg_instance"}[5m])')
    panels["pg_wal"]       = ts_panel("pg_wal", "WAL Generation Rate (bytes/s)",
        'sum(rate(pg_wal_bytes_total{pg_instance="$pg_instance"}[5m]))')

    # Locks, cache & replication (3)
    panels["pg_locks"]     = ts_panel("pg_locks", "Locks Held by Mode",
        'sum by (mode)(pg_locks_count{pg_instance="$pg_instance"})', variant="stacked-bar")
    panels["pg_cache_ts"]  = ts_panel("pg_cache_ts", "Buffer Cache Hit Ratio",
        'min(pg_database_cache_hit_ratio{pg_instance="$pg_instance"})')
    panels["pg_repl_ts"]   = ts_panel("pg_repl_ts", "Replication Lag by Replica (s)",
        'max by (replica_name)(pg_replication_lag_seconds{pg_instance="$pg_instance"})')

    # VM correlation row — picked via the separate $vm_name dropdown
    panels["pg_iowait"]    = ts_panel("pg_iowait", "VM Linux iowait (%)",
        'max(node_cpu_iowait_percent{vm_name="$vm_name"})')
    panels["pg_vmware_dw"] = ts_panel("pg_vmware_dw", "VMware VM Disk Write Latency (ms)",
        'max(vmware_vm_disk_write_latency_ms{vm_name="$vm_name"})')
    panels["pg_vm_ready"]  = ts_panel("pg_vm_ready", "VMware VM CPU Ready (ms / 20s)",
        'max(vmware_vm_cpu_ready_summation_ms{vm_name="$vm_name"})')
    panels["pg_node_disk"] = ts_panel("pg_node_disk", "Linux node_disk_io_now (outstanding)",
        'max(node_disk_io_now{vm_name="$vm_name"})')
    panels["pg_load1"]     = ts_panel("pg_load1", "Linux node_load1",
        'max(node_load1{vm_name="$vm_name"})')
    panels["pg_mem_avail"] = ts_panel("pg_mem_avail", "Linux MemAvailable (bytes)",
        'min(node_memory_memavailable_bytes{vm_name="$vm_name"})')

    sections = [
        {"title": "DB Posture", "cells": row_at(0,
            ("pg_p95", 4, 3), ("pg_back", 4, 3), ("pg_cache", 4, 3),
            ("pg_qps", 4, 3), ("pg_wal_rate", 4, 3), ("pg_repl", 4, 3))},
        {"title": "Query Latency Analysis", "cells": row_at(0,
            ("pg_p95_class", 8, 7), ("pg_p99_hist", 8, 7), ("pg_lat_buckets", 8, 7))},
        {"title": "Throughput", "cells": row_at(0,
            ("pg_tx_rate", 8, 6), ("pg_tup_rate", 8, 6), ("pg_blk_rate", 8, 6))},
        {"title": "Sessions & Wait Events", "cells": row_at(0,
            ("pg_acts_wet", 12, 7), ("pg_acts_state", 12, 7))},
        {"title": "Checkpoint & WAL", "cells": row_at(0,
            ("pg_cp_write", 8, 6), ("pg_cp_sync", 8, 6), ("pg_wal", 8, 6))},
        {"title": "Locks · Cache · Replication", "cells": row_at(0,
            ("pg_locks", 8, 6), ("pg_cache_ts", 8, 6), ("pg_repl_ts", 8, 6))},
        {"title": "VM Correlation — pick the VM hosting this DB (vm-bajaj-pg-01 for pg-bajaj-01, etc.)",
         "cells": row_at(0,
            ("pg_iowait", 12, 6), ("pg_vmware_dw", 12, 6))},
        {"title": "VM Correlation — guest signals", "cells": row_at(0,
            ("pg_vm_ready", 6, 6), ("pg_node_disk", 6, 6),
            ("pg_load1", 6, 6), ("pg_mem_avail", 6, 6))},
    ]
    return {
        "name": "Airtel — Tenant PostgreSQL Detail",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_PG, VAR_VM],
            "panels": panels,
            "sections": sections,
        },
    }

# ============================================================
# 3. VMware Infrastructure — spec §24.3 + §24.4
# ============================================================

def dashboard_vmware_infra():
    """VMware/vCenter operations view — what a virtualisation engineer
    pulls up. Mirrors the standard Veeam ONE / vRealize ops layout:
    inventory rollups, datastore health, host health, VM contention,
    then guest-side pressure."""
    panels = {}

    # Inventory KPI tiles (4)
    panels["vi_hosts"]    = label_panel("vi_hosts", "ESXi Hosts",
        'count(group by (esxi_host_id)(last_over_time(vmware_host_cpu_usage_percent[5m])))',
        color="#3b82f6")
    panels["vi_vms"]      = label_panel("vi_vms", "VMs (powered on)",
        'sum(last_over_time(vmware_vm_power_state[5m]))', color="#3b82f6")
    panels["vi_ds"]       = label_panel("vi_ds", "Datastores",
        'count(group by (datastore_id)(last_over_time(vmware_datastore_capacity_bytes[5m])))',
        color="#3b82f6")
    panels["vi_cap_used"] = label_panel("vi_cap_used", "Storage Used",
        '(sum(vmware_datastore_capacity_bytes) - sum(vmware_datastore_free_bytes)) / sum(vmware_datastore_capacity_bytes) * 100',
        color="#f59e0b", unit="%")

    # Datastore performance (4)
    panels["ds_write"]   = ts_panel("ds_write", "Datastore Write Latency (ms)",
        'max by (datastore_name)(vmware_datastore_write_latency_ms)')
    panels["ds_read"]    = ts_panel("ds_read", "Datastore Read Latency (ms)",
        'max by (datastore_name)(vmware_datastore_read_latency_ms)')
    panels["ds_qd"]      = ts_panel("ds_qd", "Datastore Queue Depth",
        'max by (datastore_name)(vmware_datastore_queue_depth)')
    panels["ds_iops_w"]  = ts_panel("ds_iops_w", "Datastore Write IOPS",
        'max by (datastore_name)(vmware_datastore_iops_write)', variant="stacked-area")

    # Storage capacity (3)
    panels["ds_cap_used"] = ts_panel("ds_cap_used", "Capacity Used by Datastore (bytes)",
        'max by (datastore_name)(vmware_datastore_capacity_bytes) - max by (datastore_name)(vmware_datastore_free_bytes)',
        variant="stacked-bar")
    panels["ds_free"]     = ts_panel("ds_free", "Free Bytes by Datastore",
        'max by (datastore_name)(vmware_datastore_free_bytes)')
    panels["ds_tier"]     = ts_panel("ds_tier", "Capacity by Service Tier (bytes)",
        'sum by (service_tier)(vmware_datastore_capacity_bytes)', variant="stacked-bar")

    # Host CPU/Memory (3)
    panels["host_cpu"]    = ts_panel("host_cpu", "ESXi Host CPU (%)",
        'max by (esxi_host_name)(vmware_host_cpu_usage_percent)')
    panels["host_mem"]    = ts_panel("host_mem", "ESXi Host Memory (%)",
        'max by (esxi_host_name)(vmware_host_memory_usage_percent)')
    panels["host_vmc"]    = ts_panel("host_vmc", "VMs per Host",
        'max by (esxi_host_name)(vmware_host_vm_count)', variant="stacked-bar")

    # Host disk & network (3)
    panels["host_dw"]     = ts_panel("host_dw", "Host Disk Write Latency (ms)",
        'max by (esxi_host_name)(vmware_host_disk_write_latency_ms)')
    panels["host_drx"]    = ts_panel("host_drx", "Host Net Dropped Rx (pkts/s)",
        'sum by (esxi_host_name)(rate(vmware_host_net_dropped_rx_total[5m]))')
    panels["host_dtx"]    = ts_panel("host_dtx", "Host Net Dropped Tx (pkts/s)",
        'sum by (esxi_host_name)(rate(vmware_host_net_dropped_tx_total[5m]))')

    # VM CPU (3)
    panels["vm_cpu"]      = ts_panel("vm_cpu", "VM CPU Usage by VM (%)",
        'max by (vm_name)(vmware_vm_cpu_usage_percent)')
    panels["vm_ready"]    = ts_panel("vm_ready", "VM CPU Ready by VM (ms / 20s)",
        'max by (vm_name)(vmware_vm_cpu_ready_summation_ms)')
    panels["vm_load1"]    = ts_panel("vm_load1", "Linux load1 by VM",
        'max by (vm_name)(node_load1)')

    # VM Memory & Disk (3)
    panels["vm_mem"]      = ts_panel("vm_mem", "VM Memory Usage by VM (%)",
        'max by (vm_name)(vmware_vm_memory_usage_percent)')
    panels["vm_balloon"]  = ts_panel("vm_balloon", "VM Memory Ballooned (bytes)",
        'max by (vm_name)(vmware_vm_memory_ballooned_bytes)')
    panels["vm_dw"]       = ts_panel("vm_dw", "VM Disk Write Latency by VM (ms)",
        'max by (vm_name)(vmware_vm_disk_write_latency_ms)')

    # Linux guest pressure (3)
    panels["vm_iow"]      = ts_panel("vm_iow", "Linux iowait by VM (%)",
        'max by (vm_name)(node_cpu_iowait_percent)')
    panels["vm_steal"]    = ts_panel("vm_steal", "Linux CPU steal by VM (%)",
        'max by (vm_name)(node_cpu_steal_percent)')
    panels["vm_swap"]     = ts_panel("vm_swap", "Linux Swap In+Out by VM (pages/s)",
        'sum by (vm_name)(rate(node_vmstat_pswpin[5m])) + sum by (vm_name)(rate(node_vmstat_pswpout[5m]))')

    # Blast-radius helper (1)
    panels["bl_tenants_ds"] = ts_panel("bl_tenants_ds",
        "Tenants attached to selected datastore — VM disk write latency",
        'max by (tenant_id, vm_name)(vmware_vm_disk_write_latency_ms{datastore_id="$datastore_id"})',
        variant="stacked-bar")

    # Active alerts (1)
    panels["vi_alerts"]   = ts_panel("vi_alerts", "Active Infra Alerts",
        'max by (alert_name, alert_severity, suspected_layer)(cardinal_alert_active{suspected_layer!=""})',
        variant="stacked-bar")

    sections = [
        {"title": "Inventory", "cells": row_at(0,
            ("vi_hosts", 6, 3), ("vi_vms", 6, 3), ("vi_ds", 6, 3), ("vi_cap_used", 6, 3))},
        {"title": "Datastore Performance", "cells": row_at(0,
            ("ds_write", 6, 7), ("ds_read", 6, 7), ("ds_qd", 6, 7), ("ds_iops_w", 6, 7))},
        {"title": "Storage Capacity", "cells": row_at(0,
            ("ds_cap_used", 8, 6), ("ds_free", 8, 6), ("ds_tier", 8, 6))},
        {"title": "ESXi Host — CPU / Memory / VMs", "cells": row_at(0,
            ("host_cpu", 8, 6), ("host_mem", 8, 6), ("host_vmc", 8, 6))},
        {"title": "ESXi Host — Disk & Network", "cells": row_at(0,
            ("host_dw", 8, 6), ("host_drx", 8, 6), ("host_dtx", 8, 6))},
        {"title": "VM — CPU Layer", "cells": row_at(0,
            ("vm_cpu", 8, 6), ("vm_ready", 8, 6), ("vm_load1", 8, 6))},
        {"title": "VM — Memory & Disk", "cells": row_at(0,
            ("vm_mem", 8, 6), ("vm_balloon", 8, 6), ("vm_dw", 8, 6))},
        {"title": "Linux Guest Pressure", "cells": row_at(0,
            ("vm_iow", 8, 6), ("vm_steal", 8, 6), ("vm_swap", 8, 6))},
        {"title": "Blast Radius — Tenants on Selected Datastore", "cells": row_at(0,
            ("bl_tenants_ds", 24, 7))},
        {"title": "Active Alerts", "cells": row_at(0,
            ("vi_alerts", 24, 6))},
    ]
    return {
        "name": "Airtel — VMware Infrastructure",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_DATASTORE],
            "panels": panels,
            "sections": sections,
        },
    }

# ============================================================
# 4. Correlation & Blast Radius — spec §24.5
# ============================================================

def dashboard_correlation():
    panels = {}

    # KPI: the application-visible breach
    panels["cor_burn"] = label_panel("cor_burn",
        "Bajaj Burn Rate",
        'max(tenant_slo_burn_rate{tenant_id="tenant_bajaj_finance"})',
        color="#ef4444", unit="x")
    panels["cor_p95"]  = label_panel("cor_p95",
        "Bajaj PG p95 (ms)",
        'max(pg_query_latency_p95_ms{pg_instance="pg-bajaj-01"})',
        color="#ef4444", unit="ms")
    panels["cor_ds"]   = label_panel("cor_ds",
        "ds-gold-delhi-02 Write Latency (ms)",
        'max(vmware_datastore_write_latency_ms{datastore_id="datastore-202"})',
        color="#f59e0b", unit="ms")
    panels["cor_at_risk"] = label_panel("cor_at_risk",
        "IndiGo Burn (at risk)",
        'max(tenant_slo_burn_rate{tenant_id="tenant_indigo_ops"})',
        color="#f59e0b", unit="x")

    # The chain: 4 series stacked together visually so the cause→symptom story is obvious
    panels["cor_chain_lat"] = ts_panel("cor_chain_lat",
        "Bajaj Latency Chain (ms) — datastore → VM → PG",
        [
            'max(vmware_datastore_write_latency_ms{datastore_id="datastore-202"})',
            'max(vmware_vm_disk_write_latency_ms{vm_name="vm-bajaj-pg-01"})',
            'max(pg_query_latency_p95_ms{pg_instance="pg-bajaj-01"})',
        ])

    panels["cor_chain_host"] = ts_panel("cor_chain_host",
        "Bajaj Host Contention — host_lat & vm_cpu_ready",
        [
            'max(vmware_host_disk_write_latency_ms{esxi_host_id="host-1017"})',
            'max(vmware_vm_cpu_ready_summation_ms{vm_name="vm-bajaj-pg-01"})',
        ])

    panels["cor_chain_guest"] = ts_panel("cor_chain_guest",
        "Bajaj Guest Pressure — iowait & disk_io_now",
        [
            'max(node_cpu_iowait_percent{vm_name="vm-bajaj-pg-01"})',
            'max(node_disk_io_now{vm_name="vm-bajaj-pg-01"})',
        ])

    panels["cor_app"] = ts_panel("cor_app",
        "Bajaj Application Symptoms — probe & SLO",
        [
            'max(airtel_probe_latency_ms{tenant_id="tenant_bajaj_finance"})',
            'max(tenant_slo_burn_rate{tenant_id="tenant_bajaj_finance"})',
        ])

    # Bajaj WAL + checkpoint pressure (the PG-internal evidence of the IO stall)
    panels["cor_pg_wal"] = ts_panel("cor_pg_wal",
        "Bajaj PG — WAL & Checkpoint Pressure",
        [
            'rate(pg_wal_bytes_total{pg_instance="pg-bajaj-01"}[5m])',
            'rate(pg_checkpoint_sync_time_seconds_total{pg_instance="pg-bajaj-01"}[5m]) * 1000000',
        ])

    # Bajaj wait events — IO waits should dominate during the breach
    panels["cor_pg_waits"] = ts_panel("cor_pg_waits",
        "Bajaj PG — Sessions by wait_event_type",
        'sum by (wait_event_type)(pg_stat_activity_count{pg_instance="pg-bajaj-01"})',
        variant="stacked-area")

    # Blast radius: every tenant whose VM lives on the degraded shared datastore
    panels["cor_blast"] = ts_panel("cor_blast",
        "Blast Radius — VM disk write latency by tenant on datastore-202",
        'max by (tenant_id, vm_name)(vmware_vm_disk_write_latency_ms{datastore_id="datastore-202"})',
        variant="stacked-bar")

    # Same host neighbours — show every VM that shares host-1017
    panels["cor_host_blast"] = ts_panel("cor_host_blast",
        "Blast Radius — VM CPU Ready on host-1017",
        'max by (tenant_id, vm_name)(vmware_vm_cpu_ready_summation_ms{esxi_host_id="host-1017"})',
        variant="stacked-bar")

    # Indigo as the at-risk tenant — show its PG p95 trailing Bajaj's
    panels["cor_indigo_chain"] = ts_panel("cor_indigo_chain",
        "IndiGo Chain — same datastore, lower amplitude",
        [
            'max(vmware_vm_disk_write_latency_ms{vm_name="vm-indigo-pg-01"})',
            'max(node_cpu_iowait_percent{vm_name="vm-indigo-pg-01"})',
            'max(pg_query_latency_p95_ms{pg_instance="pg-indigo-01"})',
            'max(tenant_slo_burn_rate{tenant_id="tenant_indigo_ops"})',
        ])

    # Shared storage array — broader blast radius
    panels["cor_array_blast"] = ts_panel("cor_array_blast",
        "Storage Array Blast — gold-01 datastores write latency",
        'max by (datastore_name)(vmware_datastore_write_latency_ms{storage_array="san-delhi-gold-01"})')

    # Active alerts table
    panels["cor_alerts"] = ts_panel("cor_alerts",
        "Active Alerts",
        'max by (alert_name, alert_severity, suspected_layer, affected_entity_id)(cardinal_alert_active)',
        variant="stacked-bar")

    sections = [
        {"title": "The Breach", "cells": row_at(0,
            ("cor_burn", 6, 3), ("cor_p95", 6, 3), ("cor_ds", 6, 3), ("cor_at_risk", 6, 3))},
        {"title": "Bajaj — Cause to Symptom (datastore → VM → PG)", "cells": row_at(0,
            ("cor_chain_lat", 24, 8))},
        {"title": "Host & Guest Evidence", "cells": row_at(0,
            ("cor_chain_host", 12, 7), ("cor_chain_guest", 12, 7))},
        {"title": "Bajaj — PG-internal Evidence", "cells": row_at(0,
            ("cor_pg_wal", 12, 6), ("cor_pg_waits", 12, 6))},
        {"title": "Bajaj — Application Symptoms", "cells": row_at(0,
            ("cor_app", 24, 6))},
        {"title": "Blast Radius — Shared Datastore (datastore-202)", "cells": row_at(0,
            ("cor_blast", 24, 6))},
        {"title": "Blast Radius — Shared Host (host-1017)", "cells": row_at(0,
            ("cor_host_blast", 24, 6))},
        {"title": "Blast Radius — Shared Storage Array (san-delhi-gold-01)", "cells": row_at(0,
            ("cor_array_blast", 24, 6))},
        {"title": "At-Risk Tenant — IndiGo (same datastore, lower amplitude)", "cells": row_at(0,
            ("cor_indigo_chain", 24, 8))},
        {"title": "Active Alerts", "cells": row_at(0,
            ("cor_alerts", 24, 6))},
    ]
    return {
        "name": "Airtel — Correlation & Blast Radius",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [],
            "panels": panels,
            "sections": sections,
        },
    }

# ---------- POST helper ----------

def main():
    """Emit SQL on stdout — pipe to psql."""
    builders = [
        dashboard_tenant_health,
        dashboard_pg_detail,
        dashboard_vmware_infra,
        dashboard_correlation,
    ]
    print("BEGIN;")
    for b in builders:
        d = b()
        spec_json = json.dumps(d["spec"]).replace("'", "''")
        name = d["name"].replace("'", "''")
        # Upsert by (org_id, name) — delete-then-insert is safest given the
        # name isn't a unique index.
        print(f"DELETE FROM maestro_dashboards WHERE org_id = '{ORG_ID}' AND name = '{name}' AND deleted_at IS NULL;")
        print(f"INSERT INTO maestro_dashboards (org_id, name, spec) VALUES ('{ORG_ID}', '{name}', '{spec_json}'::jsonb) RETURNING id, name;")
    print("COMMIT;")


if __name__ == "__main__":
    main()
