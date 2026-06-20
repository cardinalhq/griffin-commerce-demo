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
    panels = {}

    # Row 1 — KPI tiles for the selected tenant
    panels["th_burn"]   = label_panel("th_burn",   "SLO Burn Rate",
        'max(tenant_slo_burn_rate{tenant_id="$tenant_id"})', color="#ef4444", unit="x")
    panels["th_comp"]   = label_panel("th_comp",   "SLO Compliance",
        'min(tenant_slo_compliance_ratio{tenant_id="$tenant_id"}) * 100', color="#10b981", unit="%")
    panels["th_budget"] = label_panel("th_budget", "Error Budget Remaining",
        'min(tenant_slo_error_budget_remaining_ratio{tenant_id="$tenant_id"}) * 100', color="#f59e0b", unit="%")
    panels["th_probe"]  = label_panel("th_probe",  "Probe Success",
        'min(airtel_probe_success_ratio{tenant_id="$tenant_id"}) * 100', color="#10b981", unit="%")

    # Row 2 — Fleet burn rate + compliance trend
    panels["th_burn_all"] = ts_panel("th_burn_all", "Burn Rate by Tenant",
        'max by (tenant_id)(tenant_slo_burn_rate)')
    panels["th_comp_all"] = ts_panel("th_comp_all", "Compliance Ratio by Tenant",
        'min by (tenant_id)(tenant_slo_compliance_ratio)')

    # Row 3 — Application latency narrative
    panels["th_pg_p95"]  = ts_panel("th_pg_p95", "PostgreSQL p95 Latency by Tenant (ms)",
        'max by (tenant_id)(pg_query_latency_p95_ms)')
    panels["th_probe_lat"] = ts_panel("th_probe_lat", "Probe Latency by Tenant (ms)",
        'max by (tenant_id)(airtel_probe_latency_ms)')

    # Row 4 — Shared infrastructure rollup
    panels["th_ds_lat"]  = ts_panel("th_ds_lat", "Datastore Write Latency (ms)",
        'max by (datastore_name)(vmware_datastore_write_latency_ms)')
    panels["th_host_lat"] = ts_panel("th_host_lat", "ESXi Host Disk Write Latency (ms)",
        'max by (esxi_host_name)(vmware_host_disk_write_latency_ms)')

    # Row 5 — Probe success heat
    panels["th_probe_succ_all"] = ts_panel("th_probe_succ_all", "Probe Success Ratio by Tenant",
        'min by (tenant_id)(airtel_probe_success_ratio)')

    # Row 6 — Active alerts table-via-ts
    panels["th_alerts"] = ts_panel("th_alerts", "Active Alerts (cardinal_alert_active)",
        'max by (alert_name, alert_severity, affected_entity_id, suspected_layer)(cardinal_alert_active)',
        variant="stacked-bar")

    sections = [
        {"title": "Selected Tenant SLO Snapshot", "cells": row_at(0,
            ("th_burn", 6, 3), ("th_comp", 6, 3), ("th_budget", 6, 3), ("th_probe", 6, 3))},
        {"title": "SLO Burn vs Compliance — Fleet", "cells": row_at(0,
            ("th_burn_all", 12, 7), ("th_comp_all", 12, 7))},
        {"title": "Application-Visible Latency", "cells": row_at(0,
            ("th_pg_p95", 12, 7), ("th_probe_lat", 12, 7))},
        {"title": "Shared Infrastructure", "cells": row_at(0,
            ("th_ds_lat", 12, 7), ("th_host_lat", 12, 7))},
        {"title": "Probe Success — Full Fleet", "cells": row_at(0,
            ("th_probe_succ_all", 24, 6))},
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
    panels = {}
    panels["pg_p95"]      = label_panel("pg_p95",      "p95 Latency",
        'max(pg_query_latency_p95_ms{pg_instance="$pg_instance"})', color="#ef4444", unit="ms")
    panels["pg_back"]     = label_panel("pg_back",     "Backends",
        'max(pg_stat_database_numbackends{pg_instance="$pg_instance"})', color="#3b82f6")
    panels["pg_cache"]    = label_panel("pg_cache",    "Cache Hit",
        'min(pg_database_cache_hit_ratio{pg_instance="$pg_instance"}) * 100', color="#10b981", unit="%")
    panels["pg_repl"]     = label_panel("pg_repl",     "Replication Lag",
        'max(pg_replication_lag_seconds{pg_instance="$pg_instance"})', color="#f59e0b", unit="s")

    panels["pg_p95_class"] = ts_panel("pg_p95_class", "p95 Latency by Query Class (ms)",
        'max by (query_class)(pg_query_latency_p95_ms{pg_instance="$pg_instance"})')
    panels["pg_acts"]      = ts_panel("pg_acts", "Active Sessions by wait_event_type",
        'sum by (wait_event_type)(pg_stat_activity_count{pg_instance="$pg_instance"})',
        variant="stacked-area")

    panels["pg_cp_write"]  = ts_panel("pg_cp_write", "Checkpoint Write Rate (s/s)",
        'rate(pg_checkpoint_write_time_seconds_total{pg_instance="$pg_instance"}[1m])')
    panels["pg_cp_sync"]   = ts_panel("pg_cp_sync", "Checkpoint Sync Rate (s/s)",
        'rate(pg_checkpoint_sync_time_seconds_total{pg_instance="$pg_instance"}[1m])')

    panels["pg_locks"]     = ts_panel("pg_locks", "Locks by mode",
        'sum by (mode)(pg_locks_count{pg_instance="$pg_instance"})', variant="stacked-bar")
    panels["pg_cache_ts"]  = ts_panel("pg_cache_ts", "Cache Hit Ratio",
        'min(pg_database_cache_hit_ratio{pg_instance="$pg_instance"})')

    # VM correlation row — picked via the separate $vm_name dropdown so we
    # avoid a group_left join (query engine doesn't support it).
    panels["pg_iowait"]    = ts_panel("pg_iowait", "VM Linux iowait (%)",
        'max(node_cpu_iowait_percent{vm_name="$vm_name"})')
    panels["pg_vmware_dw"] = ts_panel("pg_vmware_dw", "VMware VM Disk Write Latency (ms)",
        'max(vmware_vm_disk_write_latency_ms{vm_name="$vm_name"})')
    panels["pg_vm_ready"]  = ts_panel("pg_vm_ready", "VMware VM CPU Ready (ms / 20s)",
        'max(vmware_vm_cpu_ready_summation_ms{vm_name="$vm_name"})')
    panels["pg_node_disk"] = ts_panel("pg_node_disk", "Linux node_disk_io_now (outstanding ops)",
        'max(node_disk_io_now{vm_name="$vm_name"})')

    sections = [
        {"title": "Tile Row", "cells": row_at(0,
            ("pg_p95", 6, 3), ("pg_back", 6, 3), ("pg_cache", 6, 3), ("pg_repl", 6, 3))},
        {"title": "Query Latency & Sessions", "cells": row_at(0,
            ("pg_p95_class", 12, 7), ("pg_acts", 12, 7))},
        {"title": "Checkpoint Behavior", "cells": row_at(0,
            ("pg_cp_write", 12, 6), ("pg_cp_sync", 12, 6))},
        {"title": "Locks & Cache", "cells": row_at(0,
            ("pg_locks", 12, 6), ("pg_cache_ts", 12, 6))},
        {"title": "VM Correlation — pick the VM hosting the PG (vm-bajaj-pg-01 for pg-bajaj-01, etc.)", "cells": row_at(0,
            ("pg_iowait", 12, 7), ("pg_vmware_dw", 12, 7))},
        {"title": "VM Correlation — more signals", "cells": row_at(0,
            ("pg_vm_ready", 12, 6), ("pg_node_disk", 12, 6))},
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
    panels = {}

    # Datastore row
    panels["ds_write"] = ts_panel("ds_write", "Datastore Write Latency (ms)",
        'max by (datastore_name)(vmware_datastore_write_latency_ms)')
    panels["ds_read"]  = ts_panel("ds_read", "Datastore Read Latency (ms)",
        'max by (datastore_name)(vmware_datastore_read_latency_ms)')
    panels["ds_qd"]    = ts_panel("ds_qd", "Datastore Queue Depth",
        'max by (datastore_name)(vmware_datastore_queue_depth)')
    panels["ds_iops"]  = ts_panel("ds_iops", "Datastore Write IOPS",
        'max by (datastore_name)(vmware_datastore_iops_write)', variant="stacked-area")

    # Host row
    panels["host_cpu"] = ts_panel("host_cpu", "ESXi Host CPU (%)",
        'max by (esxi_host_name)(vmware_host_cpu_usage_percent)')
    panels["host_dw"]  = ts_panel("host_dw", "ESXi Host Disk Write Latency (ms)",
        'max by (esxi_host_name)(vmware_host_disk_write_latency_ms)')
    panels["host_vmc"] = ts_panel("host_vmc", "VMs per Host",
        'max by (esxi_host_name)(vmware_host_vm_count)', variant="stacked-bar")

    # VM-level row
    panels["vm_dw_all"] = ts_panel("vm_dw_all", "VM Disk Write Latency by VM (ms)",
        'max by (vm_name)(vmware_vm_disk_write_latency_ms)')
    panels["vm_ready"]  = ts_panel("vm_ready", "VMware CPU Ready by VM (ms / 20s)",
        'max by (vm_name)(vmware_vm_cpu_ready_summation_ms)')
    panels["vm_iow"]    = ts_panel("vm_iow", "Linux iowait by VM (%)",
        'max by (vm_name)(node_cpu_iowait_percent)')
    panels["vm_steal"]  = ts_panel("vm_steal", "Linux CPU steal by VM (%)",
        'max by (vm_name)(node_cpu_steal_percent)')

    # Blast-radius helper
    panels["bl_tenants_ds"] = ts_panel("bl_tenants_ds",
        "Tenants attached to selected datastore — VM disk write latency",
        'max by (tenant_id, vm_name)(vmware_vm_disk_write_latency_ms{datastore_id="$datastore_id"})',
        variant="stacked-bar")

    sections = [
        {"title": "Datastores", "cells": row_at(0,
            ("ds_write", 12, 7), ("ds_read", 12, 7))},
        {"title": "Datastore Workload", "cells": row_at(0,
            ("ds_qd", 12, 6), ("ds_iops", 12, 6))},
        {"title": "ESXi Hosts", "cells": row_at(0,
            ("host_cpu", 8, 6), ("host_dw", 8, 6), ("host_vmc", 8, 6))},
        {"title": "VM Layer", "cells": row_at(0,
            ("vm_dw_all", 12, 7), ("vm_ready", 12, 7))},
        {"title": "Linux Guest", "cells": row_at(0,
            ("vm_iow", 12, 6), ("vm_steal", 12, 6))},
        {"title": "Blast Radius — Tenants on Selected Datastore", "cells": row_at(0,
            ("bl_tenants_ds", 24, 7))},
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

    # Blast radius: every tenant whose VM lives on the degraded shared datastore
    panels["cor_blast"] = ts_panel("cor_blast",
        "Blast Radius — VM disk write latency by tenant on datastore-202 / host-1017",
        'max by (tenant_id, vm_name)(vmware_vm_disk_write_latency_ms{datastore_id="datastore-202"})',
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

    # Active alerts table
    panels["cor_alerts"] = ts_panel("cor_alerts",
        "Active Alerts",
        'max by (alert_name, alert_severity, suspected_layer, affected_entity_id)(cardinal_alert_active)',
        variant="stacked-bar")

    sections = [
        {"title": "The Breach", "cells": row_at(0,
            ("cor_burn", 6, 3), ("cor_p95", 6, 3), ("cor_ds", 6, 3), ("cor_at_risk", 6, 3))},
        {"title": "Bajaj — Cause to Symptom (top to bottom)", "cells": row_at(0,
            ("cor_chain_lat", 24, 8))},
        {"title": "Host & Guest Evidence", "cells": row_at(0,
            ("cor_chain_host", 12, 7), ("cor_chain_guest", 12, 7))},
        {"title": "Application Symptoms — Bajaj", "cells": row_at(0,
            ("cor_app", 24, 7))},
        {"title": "Blast Radius — Shared Datastore (datastore-202)", "cells": row_at(0,
            ("cor_blast", 24, 7))},
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
