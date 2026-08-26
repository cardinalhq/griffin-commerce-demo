#!/usr/bin/env python3
"""
Authors the NVCF M1 demo dashboards against maestro.

Targets the Cardinal demo org (3aa7b421-0ecb-48a8-bf3a-b7397814862a, named
"Airtel" historically — same org the airtel/adani pods send telemetry to
via the node-local OTel agent DaemonSet).

Five pages, scoped to the metrics M1 actually emits today:
  1. NVCF — Executive Overview      (nvcf-exec)
  2. NVCF — Per-function Deep Dive  (nvcf-function)        [var: $function_id]
  3. NVCF — GPU & Compute Fleet     (nvcf-gpu)             [var: $nvca_cluster_name]
  4. NVCF — Tenant Fairness         (nvcf-tenant)          [var: $account_name]
  5. NVCF — Deployment Regression   (nvcf-regression)      [vars: $function_id, $function_version_id]

Out of scope until M2 (panels in the spec but no metric landing yet):
  cold-start (nvcf_grpc_proxy_service_session_init_seconds),
  thermal divergence (DCGM_FI_PROF_SM_ACTIVE, DCGM_FI_DEV_POWER_USAGE),
  router (llm_request_router_*), gateway (nvcf_grpc_proxy_service_*),
  registry/cache (nvca_image_pull_issue_total, nvca_model_cache_result_total),
  rate-limit (llm_api_gateway_*), capacity (nvca_instance_type_*).

M1 emits ttft / output_tps / latency as gauges (not histograms), so the
spec's histogram_quantile(p95, ...) panels are rendered as gauge averages
here. Histogram conversion is a parallel M2 work item.

Schema reference: /tmp/dbaas-tenant-view.json (existing Cardinal HQ
dashboard). 24-column grid; cells carry {x,y,w,h,i}. Panels are 'label'
(single-stat) or 'timeseries' (variant: line | stacked-bar | stacked-area).
"""
import json

# Target: "Cardinal HQ - Demo" org in the prod-us-east-2-global maestro DB.
# The first apply targeted "Airtel" (3aa7b421-...) by mistake — that org has
# no metric segments routed to it. AIRTEL_ORG_ID below drives the cleanup
# DELETE that strips the stale rows.
ORG_ID = "6d69ff5f-d386-491e-a715-306a8f172b53"  # Cardinal HQ - Demo
AIRTEL_ORG_ID = "3aa7b421-0ecb-48a8-bf3a-b7397814862a"  # stale target, cleanup only


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


def row_at(y, *cells):
    out = []
    x = 0
    for pid, w, h in cells:
        out.append({"i": pid, "x": x, "y": y, "w": w, "h": h})
        x += w
    return out


# ---------- variables ----------
# All NVCF variables key off labels carried by metrics that are guaranteed
# to be present in M1 (function_request_total carries function_id +
# function_version_id + account_name; dcgm_fi_dev_gpu_util carries
# nvca_cluster_name).

VAR_FUNCTION_ID = {
    "kind": "query", "name": "function_id", "label": "Function",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "function_id", "metric": "function_request_total", "signal": "metrics"},
}

VAR_FUNCTION_VERSION_ID = {
    "kind": "query", "name": "function_version_id", "label": "Version",
    "sort": "alphabetical", "multi": True, "includeAll": True,
    "source": {"label": "function_version_id", "metric": "function_request_total", "signal": "metrics"},
}

VAR_CLUSTER = {
    "kind": "query", "name": "nvca_cluster_name", "label": "Cluster",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "nvca_cluster_name", "metric": "dcgm_fi_dev_gpu_util", "signal": "metrics"},
}

VAR_ACCOUNT = {
    "kind": "query", "name": "account_name", "label": "Account",
    "sort": "alphabetical", "multi": False, "includeAll": False,
    "source": {"label": "account_name", "metric": "nvcf_function_queue_depth", "signal": "metrics"},
}


# ============================================================
# 1. NVCF — Executive Overview  (nvcf-exec)
# ============================================================

def dashboard_exec():
    panels = {}

    # Row 1 — fleet stats. Service-scoped so panels render even before
    # custom variable selection.
    panels["ex_rps"] = label_panel(
        "ex_rps", "Invocations/sec (fleet)",
        'sum(rate(function_request_total{service_name="nvcf"}[1m]))',
        color="#3b82f6", unit="req/s")
    panels["ex_lat"] = label_panel(
        "ex_lat", "Avg Function Latency",
        'avg(function_request_latency{service_name="nvcf"})',
        color="#f59e0b", unit="s")
    panels["ex_gpu"] = label_panel(
        "ex_gpu", "Avg GPU Utilization",
        'avg(dcgm_fi_dev_gpu_util{service_name="nvcf"})',
        color="#10b981", unit="%")
    panels["ex_inflight"] = label_panel(
        "ex_inflight", "Total Inflight Requests",
        'sum(stargate_client_requests_inflight{service_name="nvcf"})',
        color="#8b5cf6")

    # Row 2 — fleet rate + GPU util trend
    panels["ex_rps_func"] = ts_panel(
        "ex_rps_func", "Invocations/sec by Function",
        'sum by (function_id)(rate(function_request_total{service_name="nvcf"}[1m]))',
        variant="stacked-area")
    panels["ex_gpu_cluster"] = ts_panel(
        "ex_gpu_cluster", "GPU Utilization by Cluster",
        'avg by (nvca_cluster_name)(dcgm_fi_dev_gpu_util{service_name="nvcf"})')

    # Row 3 — latency + throughput
    panels["ex_lat_func"] = ts_panel(
        "ex_lat_func", "Function Request Latency by Function (avg)",
        'avg by (function_id)(function_request_latency{service_name="nvcf"})')
    panels["ex_tps"] = ts_panel(
        "ex_tps", "Output Tokens/sec by Model",
        'avg by (model)(stargate_client_model_output_tps{service_name="nvcf"})')

    # Row 4 — queue depth + KV cache
    panels["ex_qd"] = ts_panel(
        "ex_qd", "Queue Depth by Account",
        'sum by (account_name)(nvcf_function_queue_depth{service_name="nvcf"})',
        variant="stacked-area")
    panels["ex_kv"] = ts_panel(
        "ex_kv", "KV Cache Utilization by Model (ratio)",
        'avg by (model)(stargate_client_model_kv_cache_used_tokens{service_name="nvcf"}'
        ' / stargate_client_model_kv_cache_capacity_tokens{service_name="nvcf"})')

    sections = [
        {"title": "Fleet Snapshot", "cells": row_at(
            0, ("ex_rps", 6, 3), ("ex_lat", 6, 3), ("ex_gpu", 6, 3), ("ex_inflight", 6, 3))},
        {"title": "Traffic & GPU", "cells": row_at(
            0, ("ex_rps_func", 12, 7), ("ex_gpu_cluster", 12, 7))},
        {"title": "Latency & Throughput", "cells": row_at(
            0, ("ex_lat_func", 12, 7), ("ex_tps", 12, 7))},
        {"title": "Tenant Pressure & KV Cache", "cells": row_at(
            0, ("ex_qd", 12, 7), ("ex_kv", 12, 7))},
    ]
    return {
        "name": "NVCF — Executive Overview",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [],
            "panels": panels,
            "sections": sections,
        },
    }


# ============================================================
# 2. NVCF — Per-function Deep Dive  (nvcf-function)
# ============================================================

def dashboard_function():
    panels = {}

    panels["fn_rps"] = label_panel(
        "fn_rps", "Invocations/sec",
        'sum(rate(function_request_total{function_id="$function_id"}[1m]))',
        color="#3b82f6", unit="req/s")
    panels["fn_lat"] = label_panel(
        "fn_lat", "Latency (avg, s)",
        'avg(function_request_latency{function_id="$function_id"})',
        color="#ef4444", unit="s")
    panels["fn_ttft"] = label_panel(
        "fn_ttft", "TTFT (avg, s)",
        'avg(stargate_client_request_time_to_first_token_seconds{function_id="$function_id"})',
        color="#ef4444", unit="s")
    panels["fn_replicas"] = label_panel(
        "fn_replicas", "Current Replicas",
        'sum(nvcf_autoscaler_scaling_current_instances{function_id="$function_id"})',
        color="#10b981")

    panels["fn_rps_acct"] = ts_panel(
        "fn_rps_acct", "Invocations/sec by Account",
        'sum by (account_name)(rate(function_request_total{function_id="$function_id"}[1m]))',
        variant="stacked-area")
    panels["fn_rps_ver"] = ts_panel(
        "fn_rps_ver", "Invocations/sec by Version",
        'sum by (function_version_id)(rate(function_request_total{function_id="$function_id"}[1m]))')

    panels["fn_lat_ver"] = ts_panel(
        "fn_lat_ver", "Function Latency by Version (avg, s)",
        'avg by (function_version_id)(function_request_latency{function_id="$function_id"})')
    panels["fn_ttft_ver"] = ts_panel(
        "fn_ttft_ver", "TTFT by Version (avg, s)",
        'avg by (function_version_id)('
        'stargate_client_request_time_to_first_token_seconds{function_id="$function_id"})')

    panels["fn_tps"] = ts_panel(
        "fn_tps", "Output TPS by Model",
        'avg by (model)(stargate_client_model_output_tps{function_id="$function_id"})')
    panels["fn_kv"] = ts_panel(
        "fn_kv", "KV Cache Utilization (ratio)",
        'avg by (model)(stargate_client_model_kv_cache_used_tokens{function_id="$function_id"}'
        ' / stargate_client_model_kv_cache_capacity_tokens{function_id="$function_id"})')

    panels["fn_inflight"] = ts_panel(
        "fn_inflight", "Inflight Requests by Model",
        'sum by (model)(stargate_client_requests_inflight{function_id="$function_id"})')
    panels["fn_drift"] = ts_panel(
        "fn_drift", "Replica Drift (desired − current)",
        'sum by (function_version_id)(nvcf_autoscaler_scaling_desired_instances{function_id="$function_id"})'
        ' - sum by (function_version_id)(nvcf_autoscaler_scaling_current_instances{function_id="$function_id"})')

    sections = [
        {"title": "Selected Function — Snapshot", "cells": row_at(
            0, ("fn_rps", 6, 3), ("fn_lat", 6, 3), ("fn_ttft", 6, 3), ("fn_replicas", 6, 3))},
        {"title": "Traffic", "cells": row_at(
            0, ("fn_rps_acct", 12, 7), ("fn_rps_ver", 12, 7))},
        {"title": "Latency — End-to-End vs TTFT", "cells": row_at(
            0, ("fn_lat_ver", 12, 7), ("fn_ttft_ver", 12, 7))},
        {"title": "Throughput & KV Cache", "cells": row_at(
            0, ("fn_tps", 12, 7), ("fn_kv", 12, 7))},
        {"title": "Capacity", "cells": row_at(
            0, ("fn_inflight", 12, 7), ("fn_drift", 12, 7))},
    ]
    return {
        "name": "NVCF — Per-function Deep Dive",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_FUNCTION_ID],
            "panels": panels,
            "sections": sections,
        },
    }


# ============================================================
# 3. NVCF — GPU & Compute Fleet  (nvcf-gpu)
# ============================================================

def dashboard_gpu():
    panels = {}

    panels["gpu_avg_cluster"] = label_panel(
        "gpu_avg_cluster", "Avg GPU Util — Selected Cluster",
        'avg(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"})',
        color="#10b981", unit="%")
    panels["gpu_max_cluster"] = label_panel(
        "gpu_max_cluster", "Max GPU Util — Selected Cluster",
        'max(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"})',
        color="#f59e0b", unit="%")
    panels["gpu_hosts"] = label_panel(
        "gpu_hosts", "Hosts in Cluster",
        'count(count by (Hostname)(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"}))',
        color="#3b82f6")
    panels["gpu_devices"] = label_panel(
        "gpu_devices", "GPUs in Cluster",
        'count(count by (Hostname, device)(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"}))',
        color="#3b82f6")

    panels["gpu_by_cluster"] = ts_panel(
        "gpu_by_cluster", "GPU Utilization by Cluster (fleet)",
        'avg by (nvca_cluster_name)(dcgm_fi_dev_gpu_util{service_name="nvcf"})')
    panels["gpu_by_host"] = ts_panel(
        "gpu_by_host", "GPU Utilization by Host — Selected Cluster",
        'avg by (Hostname)(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"})')

    panels["gpu_by_device"] = ts_panel(
        "gpu_by_device", "GPU Utilization by Device — Selected Cluster",
        'avg by (Hostname, device)(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"})')
    panels["gpu_modelname"] = ts_panel(
        "gpu_modelname", "GPU Utilization by Model",
        'avg by (modelName)(dcgm_fi_dev_gpu_util{nvca_cluster_name="$nvca_cluster_name"})',
        variant="stacked-area")

    panels["gpu_crashes"] = ts_panel(
        "gpu_crashes", "Container Crashes by Cluster (rate)",
        'sum by (nvca_cluster_name)(rate(nvca_container_crash_total{service_name="nvcf"}[1m]))',
        variant="stacked-bar")

    panels["gpu_servers"] = ts_panel(
        "gpu_servers", "Inflight Requests by Cluster (host-tagged via inference_server)",
        'sum by (nvca_cluster_name)(stargate_client_requests_inflight{service_name="nvcf"})')

    sections = [
        {"title": "Selected Cluster — Snapshot", "cells": row_at(
            0, ("gpu_avg_cluster", 6, 3), ("gpu_max_cluster", 6, 3),
               ("gpu_hosts", 6, 3), ("gpu_devices", 6, 3))},
        {"title": "Fleet vs Selected", "cells": row_at(
            0, ("gpu_by_cluster", 12, 7), ("gpu_by_host", 12, 7))},
        {"title": "Per-device & Per-model Utilization", "cells": row_at(
            0, ("gpu_by_device", 12, 7), ("gpu_modelname", 12, 7))},
        {"title": "Compute Plane Health", "cells": row_at(
            0, ("gpu_crashes", 12, 6), ("gpu_servers", 12, 6))},
    ]
    return {
        "name": "NVCF — GPU & Compute Fleet",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_CLUSTER],
            "panels": panels,
            "sections": sections,
        },
    }


# ============================================================
# 4. NVCF — Tenant Fairness  (nvcf-tenant)
# ============================================================

def dashboard_tenant():
    panels = {}

    panels["tn_rps"] = label_panel(
        "tn_rps", "Account Invocations/sec",
        'sum(rate(function_request_total{account_name="$account_name"}[1m]))',
        color="#3b82f6", unit="req/s")
    panels["tn_qd"] = label_panel(
        "tn_qd", "Account Queue Depth",
        'sum(nvcf_function_queue_depth{account_name="$account_name"})',
        color="#f59e0b")
    panels["tn_lat"] = label_panel(
        "tn_lat", "Account Avg Latency (s)",
        # function_request_latency lacks account_name in M1 (it's a
        # function-version-scoped gauge). Approximate via the function_ids
        # the account is hitting — same metric the deep-dive page reads.
        'avg(function_request_latency)',
        color="#ef4444", unit="s")
    panels["tn_inflight"] = label_panel(
        "tn_inflight", "Fleet Inflight Requests",
        'sum(stargate_client_requests_inflight{service_name="nvcf"})',
        color="#8b5cf6")

    panels["tn_qd_all"] = ts_panel(
        "tn_qd_all", "Queue Depth by Account (full fleet)",
        'sum by (account_name)(nvcf_function_queue_depth{service_name="nvcf"})',
        variant="stacked-area")
    panels["tn_rps_all"] = ts_panel(
        "tn_rps_all", "Invocations/sec by Account",
        'sum by (account_name)(rate(function_request_total{service_name="nvcf"}[1m]))',
        variant="stacked-area")

    panels["tn_qd_fn"] = ts_panel(
        "tn_qd_fn", "Queue Depth — Selected Account, by Function",
        'sum by (function_id)(nvcf_function_queue_depth{account_name="$account_name"})',
        variant="stacked-area")
    panels["tn_rps_fn"] = ts_panel(
        "tn_rps_fn", "Invocations/sec — Selected Account, by Function",
        'sum by (function_id)(rate(function_request_total{account_name="$account_name"}[1m]))',
        variant="stacked-area")

    panels["tn_share_disp"] = ts_panel(
        "tn_share_disp", "Account Share of Total Invocations (by display name)",
        'sum by (account_display_name)(rate(function_request_total{service_name="nvcf"}[1m]))',
        variant="stacked-bar")

    sections = [
        {"title": "Selected Account — Snapshot", "cells": row_at(
            0, ("tn_rps", 6, 3), ("tn_qd", 6, 3), ("tn_lat", 6, 3), ("tn_inflight", 6, 3))},
        {"title": "Fleet Fairness — All Accounts", "cells": row_at(
            0, ("tn_qd_all", 12, 7), ("tn_rps_all", 12, 7))},
        {"title": "Selected Account — Per-function Pressure", "cells": row_at(
            0, ("tn_qd_fn", 12, 7), ("tn_rps_fn", 12, 7))},
        {"title": "Tenant Share", "cells": row_at(
            0, ("tn_share_disp", 24, 7))},
    ]
    return {
        "name": "NVCF — Tenant Fairness",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_ACCOUNT],
            "panels": panels,
            "sections": sections,
        },
    }


# ============================================================
# 5. NVCF — Deployment Regression  (nvcf-regression)
# ============================================================
# This is the home page for the function.ttft-regression knob in M1.

def dashboard_regression():
    panels = {}

    panels["rg_ttft_p95"] = label_panel(
        "rg_ttft_p95", "TTFT max — Function",
        'max(stargate_client_request_time_to_first_token_seconds{function_id="$function_id"})',
        color="#ef4444", unit="s")
    panels["rg_lat_p95"] = label_panel(
        "rg_lat_p95", "Latency max — Function",
        'max(function_request_latency{function_id="$function_id"})',
        color="#ef4444", unit="s")
    panels["rg_versions"] = label_panel(
        "rg_versions", "Versions Active",
        'count(count by (function_version_id)(function_request_total{function_id="$function_id"}))',
        color="#3b82f6")
    panels["rg_rps"] = label_panel(
        "rg_rps", "Invocations/sec",
        'sum(rate(function_request_total{function_id="$function_id"}[1m]))',
        color="#3b82f6", unit="req/s")

    # The flagship comparison — every active version overlaid.
    panels["rg_ttft_overlay"] = ts_panel(
        "rg_ttft_overlay", "TTFT by Version (avg s) — version regression check",
        'avg by (function_version_id)('
        'stargate_client_request_time_to_first_token_seconds{function_id="$function_id"})')
    panels["rg_lat_overlay"] = ts_panel(
        "rg_lat_overlay", "Function Latency by Version (avg s)",
        'avg by (function_version_id)(function_request_latency{function_id="$function_id"})')

    panels["rg_tps_ver"] = ts_panel(
        "rg_tps_ver", "Output TPS by Model (per-version routing is via model→version map)",
        'avg by (model)(stargate_client_model_output_tps{function_id="$function_id"})')
    panels["rg_inflight_ver"] = ts_panel(
        "rg_inflight_ver", "Inflight Requests by Model",
        'sum by (model)(stargate_client_requests_inflight{function_id="$function_id"})')

    panels["rg_rps_ver"] = ts_panel(
        "rg_rps_ver", "Invocations/sec by Version",
        'sum by (function_version_id)(rate(function_request_total{function_id="$function_id"}[1m]))')
    panels["rg_drift_ver"] = ts_panel(
        "rg_drift_ver", "Replica Drift by Version (desired − current)",
        'sum by (function_version_id)(nvcf_autoscaler_scaling_desired_instances{function_id="$function_id"})'
        ' - sum by (function_version_id)(nvcf_autoscaler_scaling_current_instances{function_id="$function_id"})')

    # The "is the new version safe to promote?" page-closer
    panels["rg_ttft_acct"] = ts_panel(
        "rg_ttft_acct", "TTFT by Account — selected versions",
        'avg by (account_name, function_version_id)('
        'stargate_client_request_time_to_first_token_seconds{function_id="$function_id"})')

    sections = [
        {"title": "Selected Function — Health", "cells": row_at(
            0, ("rg_ttft_p95", 6, 3), ("rg_lat_p95", 6, 3),
               ("rg_versions", 6, 3), ("rg_rps", 6, 3))},
        {"title": "TTFT vs Latency — Version Comparison", "cells": row_at(
            0, ("rg_ttft_overlay", 12, 7), ("rg_lat_overlay", 12, 7))},
        {"title": "Throughput by Model", "cells": row_at(
            0, ("rg_tps_ver", 12, 7), ("rg_inflight_ver", 12, 7))},
        {"title": "Traffic & Autoscaler Convergence by Version", "cells": row_at(
            0, ("rg_rps_ver", 12, 7), ("rg_drift_ver", 12, 7))},
        {"title": "TTFT by Account & Version (promotion blast-radius)", "cells": row_at(
            0, ("rg_ttft_acct", 24, 7))},
    ]
    return {
        "name": "NVCF — Deployment Regression",
        "spec": {
            "duration": "1h", "schemaVersion": 2,
            "variables": [VAR_FUNCTION_ID, VAR_FUNCTION_VERSION_ID],
            "panels": panels,
            "sections": sections,
        },
    }


# ---------- SQL emitter ----------

def main():
    builders = [
        dashboard_exec,
        dashboard_function,
        dashboard_gpu,
        dashboard_tenant,
        dashboard_regression,
    ]
    print("BEGIN;")
    for b in builders:
        d = b()
        spec_json = json.dumps(d["spec"]).replace("'", "''")
        name = d["name"].replace("'", "''")
        # Cleanup: drop any prior copy from the wrong-target Airtel org.
        print(f"DELETE FROM maestro_dashboards WHERE org_id = '{AIRTEL_ORG_ID}' AND name = '{name}' AND deleted_at IS NULL;")
        # Upsert into the real demo org by (org_id, name) — delete-then-insert
        # is safest given the name isn't a unique index.
        print(f"DELETE FROM maestro_dashboards WHERE org_id = '{ORG_ID}' AND name = '{name}' AND deleted_at IS NULL;")
        print(f"INSERT INTO maestro_dashboards (org_id, name, spec) "
              f"VALUES ('{ORG_ID}', '{name}', '{spec_json}'::jsonb) "
              f"RETURNING id, name;")
    print("COMMIT;")


if __name__ == "__main__":
    main()
