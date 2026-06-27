# NVCF demo: GPU-inference simulator + Cardinal dashboards

**Status:** in-progress build.
**Date:** 2026-06-26.
**Scope:** the NVCF persona inside `griffin-commerce-demo`. Adds
`services/nvcf/` — a pure OTLP metric emitter mirroring the `dbaas` and
`solar` pattern — plus the chaos knobs, the dashboard pack, and the
Cardinal-detection playbook. One griffin pod, all personas running
simultaneously: a single live demo can flip between dashboards for
e-commerce, telco DBaaS (airtel), solar, and NVCF without redeploying.

## Summary

The demo is a synthesizer, not a working app. It does not require a real
GPU, does not run a real inference engine, and emits no real HTTP traffic
between services. Instead, the **`services/nvcf/`** package synthesizes
**NVCF-shaped OTLP metrics** (Table A below — verbatim native NVCF metric
names, verbatim label vocabulary) representing a fleet of fake functions
× versions × accounts × clusters × instances at sub-second resolution.
A controlplane fault knob can modulate the synth so a specific cohort
(function, version, account, cluster, instance type) takes on incident
values — and the matching Cardinal dashboard panel lights up.

The story when the demo is running:

1. The `nvcf` synth pod is emitting native NVCF metrics steadily — invocation
   rate, queue depth, TTFT, KV-cache usage, NVCA container health, autoscaler
   convergence, DCGM GPU utilization — across the seeded fleet of 4
   functions × 2 versions × 4 accounts × 2 clusters.
2. An SE opens the Cardinal NVCF dashboard pack and walks the prospect
   through the steady-state panels.
3. The SE activates a chaos knob via `/faults/activate?profile=ttft-regression&function=summarize-doc`.
4. Within seconds, the named panel (`p_ttft_by_function_version`)
   visibly diverges between v1 and v2 of the targeted function. Other
   cohorts stay at baseline.
5. The SE runs the playbook's Cardinal MCP call:
   `detect_outliers({metric: "stargate_client_request_time_to_first_token_seconds", focus_tags: ["function_id", "function_version_id"]})` —
   and Cardinal returns the exact cohort.

## Non-goals

- No real inference. No model weights, no vLLM, no Triton, no actual GPU
  work. All numbers are synthesized to look plausible.
- No real services. There is **no fake NVCF api-gateway, router,
  control-plane, cluster-agent, or function-worker process**. There is
  one synth process emitting the metrics those components would. This
  trades architectural impressiveness for a 10× smaller blast radius
  inside griffin and matches how `dbaas` and `solar` already work.
- No fork or modification of upstream NVCF code. The demo emulates NVCF
  *shape*; it does not run NVCF.
- Not a load test against Lakerunner. Volumes are demo-scale (~200
  metric streams, 1Hz scrape, ~few trace samples/min).
- Not a replacement for the production `lakerunner-otel-gateway` Helm
  chart. Telemetry leaves griffin via the existing OTel collector
  configuration; the demo doesn't ship its own collector.

## What we reuse from griffin (already in place)

- **Single Go binary, `SERVICE_NAME` dispatch.** Existing
  `entrypoint.sh`; we add an `nvcf` case alongside `dbaas`/`solar`.
- **`common/telemetry.go`.** Standard OTel SDK init via
  `common.SetupTelemetry("nvcf-service", nil)`.
- **`services/controlplane/` chaos knobs.** Existing knob catalog +
  SSE audit + `/admin/faults` POST. We add the 11 NVCF knobs to its
  catalog; nothing else in controlplane changes.
- **Scenario-with-trapezoid-ramp pattern from `services/dbaas/scenario.go`.**
  Single active profile at a time; per-entity, per-metric `IncidentRange`
  + `OnsetOffsetMin`; `trapezoidFactor` ramps baseline → incident over
  2min, plateau, ramp down over 5min, default 35min total. NVCF reuses
  this verbatim with NVCF-shaped entities and metrics.
- **Per-overlay dashboards.** `k8s/overlays/{adani-prod,airtel-prod}/dashboards/`
  is the pattern; `k8s/overlays/nvcf-prod/dashboards/` joins them.
- **Per-knob detection playbook.** Same shape as
  `docs/demo/conductor-detection.md` adds an `nvcf-detection.md` file.

## Architecture

### One service, one process, one ticker

```
griffin pod (single)
  ├─ catalog / cart / payment / shipping ...   (e-commerce, existing)
  ├─ controlplane                              (chaos knobs, existing — gains NVCF knobs)
  ├─ dbaas                                     (airtel DBaaS synth, existing)
  ├─ solar                                     (adani solar synth, existing)
  └─ nvcf                                      (NEW — NVCF synth, this spec)
        │
        ├─ catalog.go        seeds 4 functions × 2 versions × 4 accounts × 2 clusters × ~16 instances
        ├─ state.go          fleet entity types: Function, FunctionVersion, Account, Cluster, Instance
        ├─ metrics.go        registers all Table A metrics as OTel observable gauges/counters
        ├─ scenario.go       11 chaos knobs as Profiles with per-entity IncidentRanges
        ├─ httpserver.go     :9998 local /faults/activate?profile=… (mirrors dbaas :9999)
        └─ main.go           Start() — SetupTelemetry + RegisterMetrics + StartHTTPServer + block
```

The synth process emits metrics tagged with cohort labels (`function_id`,
`function_version_id`, `account_name`, `nvca_cluster_name`,
`instance_type`, `inference_server_id`, `model`) that span the seeded
fleet. **Cardinality is in the labels, not in processes**: one process,
~200 metric streams. Knobs target a cohort by label, not by killing a
process.

### Why no fake NVCF api-gateway / router / control-plane processes

Originally specced as 9 separate Go services with HTTP/gRPC between
them. Dropped after consolidating into griffin: the trace shape that
required would have produced *fake* spans anyway (no real router
decision, no real model inference), and would have multiplied the
`services/nvcf/` LoC ~5×. The synth process emits synthetic spans for
the inference flame-graph (see "Trace shape" below) — same visual
result, far less code.

### Two clusters, four functions, two versions, four accounts

Seeded in `services/nvcf/catalog.go`:

- Clusters: `us-west-2-a` (instance_type `NCP.GPU.A100_80GB_1x`, 2 nodes × 8 GPUs),
  `us-east-1-a` (instance_type `NCP.GPU.H100_80GB_1x`, 2 nodes × 4 GPUs)
- Functions: `chat-helpful` (streaming, LLM-shaped), `summarize-doc`
  (HTTP, LLM-shaped), `fraud-detect` (gRPC, latency-only),
  `embed-text` (HTTP, embeddings)
- Versions: each function has `v1` and `v2` `function_version_id`s
  (`fraud-detect` and `embed-text` have v1 only).
- Accounts: `acme`, `globex`, `initech`, `umbrella` — each with a
  weighted traffic mix across functions.
- Inference servers: one per (function_version_id × instance) pair —
  ~16 `inference_server_id`s total. Carries the routing-imbalance signal.

### NVCF realism — what we actually emulate

The point of the demo is that **the dashboards are real NVCF dashboards** —
they must be pointable at a production NVCF cluster and render unchanged.
That means the demo emits metrics under the **verbatim names NVCF itself
emits**, with the **verbatim label vocabulary NVCF itself uses**. Anything
the simulator emits that NVCF does not is labeled as such.

This section is the source-of-truth contract. Everything downstream
(dashboards, knob "what lights up", Cardinal MCP queries) keys off these
names exactly.

### Resource attribute / label vocabulary (NVCF-native)

These keys come from the inventory pass over `~/workspace/nvcf` —
specifically `src/compute-plane-services/nvca/internal/metrics/metrics.go`,
`src/invocation-plane-services/grpc-proxy/proxy/metrics/expiring_metrics.go`,
`src/compute-plane-services/byoo-otel-collector/generator/source-config.yaml`,
and `docs/user/metrics/state-metrics/metrics.md`.

| Key | Type | Cardinality control | Example | Notes |
|---|---|---|---|---|
| `function_id` | UUID | Bounded by tenant count × functions; expiring metrics (6h TTL) on hot paths | `f1a2b3c4d5e6f7g8h9i0` | Primary function identity |
| `function_version_id` | UUID | One per deployed version of a function | `fb1c2d3e4f5a6b7c8d9e` | **This is NVCF's `v1`/`v2` cohort dimension. Use this — not `deployment_version`.** |
| `nca_id` | string | One per cluster-agent instance | `nca-12345` | Invocation-plane label name |
| `nvca_nca_id` | string | Same as `nca_id` | `nca-12345` | NVCA-side label name (different prefix; both observed in real telemetry) |
| `nvca_cluster_name` | string | One per cluster | `us-west-2-a` | **Use this — not `nca_cluster`** |
| `nvca_cluster_group` | string | One per cluster group | `gpu-heavy` | Grouping above `nvca_cluster_name` |
| `nvca_version` | string | Low | `v1.2.3` | NVCA agent version |
| `instance_type` | string | Bounded enum | `NCP.GPU.H100_1x` | **Use this — not `gpu_model`** |
| `task_id` | UUID | One per task lifecycle | `task-xyz` | Workload identity |
| `account_name` | string | Bounded | `acme` | **NVCF's tenant identifier** (paired with `account_display_name`) |
| `account_display_name` | string | Bounded | `Acme Inc.` | Human label for `account_name` |

**Demo-only attributes** — emitted by the simulator but not part of NVCF
real telemetry. Marked clearly so anyone reading dashboards understands
which queries are NVCF-native vs demo-only:

| Key | Why it's demo-only |
|---|---|
| `function_name` | Real NVCF dashboards key off `function_id` + state-metrics-service join; we keep `function_name` for demo legibility (`fraud-detect`, `chat-helpful`, etc.) but the Cardinal dashboard joins through `nvcf_function_info` like a real NVCF deploy would |
| `nvcf_demo.workload_type` | Pure simulator hint (`http` \| `streaming` \| `grpc`) for the loadgen personas |
| `nvcf_demo.knob_id` | Chaos-knob identifier, set on `nvcf_demo.faults.injected_total` only |

### Table A — Native NVCF Signals (drive the Cardinal dashboard)

These are emitted with the **verbatim NVCF metric names**. The dashboard
panel queries reference these and nothing else. When the demo is pointed
at a real NVCF cluster, the same queries return real data with no
renaming.

**Invocation plane**

| Metric | Type | Key labels | Emitted by | NVCF source |
|---|---|---|---|---|
| `nvcf_grpc_proxy_service_session_init_seconds_total` | histogram | `is_reconnect` | `api-gateway` | `src/invocation-plane-services/grpc-proxy/proxy/metrics/metrics.go:56` |
| `nvcf_grpc_proxy_service_active_connections_total` | gauge | `namespace` | `api-gateway` | `…/metrics.go:49` |
| `nvcf_grpc_proxy_service_active_http_requests_total` | gauge | `namespace` | `api-gateway` | `…/metrics.go:42` |
| `nvcf_grpc_proxy_service_nats_error_total` | counter | — | `api-gateway` | `…/metrics.go:63` |
| `nvcf_grpc_proxy_service_nats_reconnect_total` | counter | — | `api-gateway` | `…/metrics.go:70` |
| `function_request_total` | counter | `function_id`, `function_version_id`, `nca_id` | `api-gateway` | `…/expiring_metrics.go:56` |
| `function_request_latency` | histogram | `function_id`, `function_version_id` | `state-metrics-service` (demo emits in `api-gateway`) | `docs/user/metrics/state-metrics/metrics.md:13` |
| `function.request` | counter | `function_id`, `function_version_id`, `nca_id` | `api-gateway` (Rust HTTP path) | `src/invocation-plane-services/http-invocation/crates/server/src/metrics/mod.rs:74` |
| `function.request.latency` | histogram | `function_id`, `function_version_id`, `nca_id` | `api-gateway` | `…/mod.rs:64` |
| `app.invocation.error` | counter | `http_status_code` | `api-gateway` | `…/mod.rs:69` |
| `llm_api_gateway_http_requests_total` | counter | `method`, `route`, `status` | `api-gateway` (LLM path) | `src/invocation-plane-services/llm-api-gateway/telemetry/metrics.go:136` |
| `llm_api_gateway_stream_first_token_seconds` | histogram | `endpoint` | `api-gateway` (LLM streaming) | `…/metrics.go:191` |
| `llm_api_gateway_stream_duration_seconds` | histogram | `endpoint`, `status` | `api-gateway` (LLM streaming) | `…/metrics.go:200` |
| `llm_api_gateway_llm_tokens_total` | counter | `endpoint`, `token_type`, `stream` | `api-gateway` | `…/metrics.go:175` |
| `llm_api_gateway_rate_limit_events_applied_total` | counter | — | `api-gateway` | `…/metrics.go:255` |
| `llm_api_gateway_rate_limit_events_failed_apply_total` | counter | — | `api-gateway` | `…/metrics.go:262` |
| `llm_request_router_requests_total` | counter | `routing_key`, `model`, `inference_server_id`, `status` | `router` | `docs/user/metrics/llm-request-router/metrics.md:26` |
| `llm_request_router_active_inference_servers` | gauge | `routing_key`, `model` | `router` | `…/metrics.md:35` |
| `llm_request_router_quic_connection_evictions_total` | counter | `inference_server_id`, `reason` | `router` | `…/metrics.md:30` |
| `llm_request_router_routing_duration_seconds` | histogram | `routing_key`, `model` | `router` | `…/metrics.md:34` |

**Function workload (stargate-client sidecar shape)** — emitted by `function-worker`

| Metric | Type | Key labels | NVCF source |
|---|---|---|---|
| `stargate_client_request_time_to_first_token_seconds` | histogram | `model`, `routing_key` | `docs/user/metrics/llm-function-invocation-path.md:75` |
| `stargate_client_request_duration_seconds` | histogram | `model`, `routing_key`, `status` | `…/metrics.md:76` |
| `stargate_client_requests_total` | counter | `model`, `routing_key`, `status` | `…/metrics.md:72` |
| `stargate_client_requests_inflight` | gauge | `model` | `…/metrics.md:69` |
| `stargate_client_model_output_tps` | gauge | `model` | `…/metrics.md:84` |
| `stargate_client_model_input_tps` | gauge | `model` | `…/metrics.md:83` |
| `stargate_client_model_queue_size` | gauge | `model` | `…/metrics.md:87` |
| `stargate_client_model_kv_cache_used_tokens` | gauge | `model` | `…/metrics.md:90` |
| `stargate_client_model_kv_cache_free_tokens` | gauge | `model` | `…/metrics.md:91` |
| `stargate_client_request_input_tokens_total` | counter | `model`, `routing_key`, `status` | `…/metrics.md:77` |
| `stargate_client_request_output_tokens_total` | counter | `model`, `routing_key`, `status` | `…/metrics.md:78` |
| `stargate_client_retryable_responses_total` | counter | `inference_server_id`, `reason`, `status` | `…/metrics.md:93` |
| `stargate_client_nonretryable_failures_total` | counter | `inference_server_id`, `reason` | `…/metrics.md:94` |

**Compute plane (NVCA)** — emitted by `cluster-agent`

| Metric | Type | Key labels | NVCF source |
|---|---|---|---|
| `nvca_container_crash_total` | counter | `nvca_nca_id`, `nvca_cluster_name`, `nvca_cluster_group`, `nvca_version`, `container` | `src/compute-plane-services/nvca/internal/metrics/metrics.go:387` |
| `nvca_container_restart_total` | counter | …same + `container` | `…/metrics.go:392` |
| `nvca_image_pull_issue_total` | counter | …same + `image_registry` | `…/metrics.go:423` |
| `nvca_model_cache_result_total` | counter | …same + `result`, `failure_reason` | `…/metrics.go:579` |
| `nvca_instance_type_capacity` | gauge | …same + `instance_type` | `…/metrics.go:457` |
| `nvca_instance_type_allocatable` | gauge | …same + `instance_type` | `…/metrics.go:461` |
| `nvca_instance_type_unschedulable` | gauge | …same + `instance_type` | `…/metrics.go:465` |
| `nvca_workload_result_total` | counter | …same + `workload_type`, `workload_kind`, `workload_status`, `failure_category` | `…/metrics.go:600` |
| `nvca_miniservice_controller_reconcile_phase_total` | counter | …same + `miniservice_phase` | `…/metrics.go:485` |
| `nvca_miniservice_controller_phase_transitions_total` | counter | …same + `from_phase`, `to_phase` | `…/metrics.go:490` |
| `nvca_miniservice_controller_miniservice_ready_status` | gauge | …same + `function_id`, `function_version_id`, `task_id` | `…/metrics.go:500` |
| `nvca_miniservice_controller_failures_total` | counter | …same + `failure_reason` | `…/metrics.go:495` |
| `nvca_event_queue_length` | gauge | …same + `nvca_event_name` | `…/metrics.go:375` |
| `nvca_event_process_latency` | summary | …same + `nvca_event_name` | `…/metrics.go:380` |
| `nvca_k8s_api_failure_total` | counter | …same + `resource` | `…/metrics.go:434` |

**Control plane (autoscaler + state-metrics)** — emitted by `control-plane`

| Metric | Type | Key labels | NVCF source |
|---|---|---|---|
| `nvcf_autoscaler.scaling.current_instances` | gauge | `function_id`, `function_version_id` | `src/control-plane-services/function-autoscaler/crates/server/src/metrics/mod.rs:72` |
| `nvcf_autoscaler.scaling.desired_instances` | gauge | `function_id`, `function_version_id` | `…/mod.rs:77` |
| `nvcf_autoscaler.scaling.utilization` | gauge | `function_id`, `function_version_id` | `…/mod.rs:82` |
| `nvcf_autoscaler.autoscaling.status` | gauge | `function_id`, `function_version_id` (encoded reason code) | `…/mod.rs:37` |
| `nvcf_autoscaler.processing.utilization_data_age_milliseconds` | histogram | — | `…/mod.rs:153` |
| `nvcf_autoscaler.function_table_state` | gauge | `function_id`, `function_version_id` | `…/mod.rs:111` |
| `nvcf_function_info` | gauge | `function_id`, `function_version_id`, `name`, `container_image`, `helm_chart`, `endpoint`, `secrets` | `docs/user/metrics/state-metrics/metrics.md:8` |
| `nvcf_function_status` | gauge | `account_name`, `account_display_name`, `function_id`, `function_version_id`, `name`, `nca_id`, `status`, `version` | `…/metrics.md:9` |
| `nvcf_function_instances_current` | gauge | `function_version_id`, `nca_id`, `version` | `…/metrics.md:10` |
| `nvcf_function_queue_depth` | gauge | `account_name`, `account_display_name`, `function_id`, `function_version_id`, `name`, `nca_id`, `version` | `…/metrics.md:11` |

**GPU — NVCF BYOO DCGM allowlist (exactly 11 metrics, no more)**

> ⚠️ **Critical**: NVCF's BYOO collector scrapes a **fixed allowlist** of
> DCGM fields (`src/compute-plane-services/byoo-otel-collector/generator/source-config.yaml:403-426`).
> `DCGM_FI_DEV_GPU_TEMP`, `DCGM_FI_DEV_SM_CLOCK`, `DCGM_FI_DEV_FB_USED`,
> and `DCGM_FI_DEV_FB_FREE` are **not in the allowlist**. The Cardinal
> dashboard cannot rely on them as drop-in metrics. Demo emits the
> allowlist exactly; "thermal" and "memory fill" are inferred from
> allowlisted metrics (see `cluster.thermal-throttle` knob for the
> indirect-signal approach).

| Metric | Type | Key labels |
|---|---|---|
| `DCGM_FI_DEV_GPU_UTIL` | gauge | `device`, `modelName`, `pci_bus_id`, `DCGM_FI_DRIVER_VERSION` |
| `DCGM_FI_PROF_PIPE_TENSOR_ACTIVE` | gauge | …same |
| `DCGM_FI_PROF_DRAM_ACTIVE` | gauge | …same |
| `DCGM_FI_PROF_SM_ACTIVE` | gauge | …same |
| `DCGM_FI_PROF_SM_OCCUPANCY` | gauge | …same |
| `DCGM_FI_PROF_PCIE_TX_BYTES` | gauge | …same |
| `DCGM_FI_PROF_PCIE_RX_BYTES` | gauge | …same |
| `DCGM_FI_PROF_NVLINK_TX_BYTES` | gauge | …same |
| `DCGM_FI_PROF_NVLINK_RX_BYTES` | gauge | …same |
| `DCGM_FI_DEV_POWER_USAGE` | gauge | …same |
| `DCGM_FI_DEV_VGPU_MEMORY_USAGE` | gauge | …same |

**Worker service sidecar** — emitted by `function-worker`

| Metric | Type | NVCF source |
|---|---|---|
| `nvcf_worker_service_request_total` | counter | `…/byoo-otel-collector/generator/source-config.yaml:92` |
| `nvcf_worker_service_response_total` | counter | `…:93` |
| `nvcf_worker_service_inference_request_time_seconds_total` | counter | `…:99` |
| `nvcf_worker_service_inference_failure_total` | counter | `…:102` |
| `nvcf_worker_service_stream_session_duration_seconds` | histogram | `…:96` |

### Table B — Demo App Signals (loadgen / synthesizer telemetry)

These metrics exist because NVCF doesn't expose them and the demo needs
them to tell the story. They are **clearly namespaced under
`nvcf_demo.*`** so no one mistakes them for native NVCF telemetry.
Dashboard panels that use these must be marked "demo-only" in the panel
description.

| Metric | Type | Labels | Purpose |
|---|---|---|---|
| `nvcf_demo.loadgen.requests_total` | counter | `function_name`, `function_version_id`, `account_name`, `persona` | Loadgen's client-side issued-request counter — the source-of-truth for "how much did we ask for", before any router or rate limit |
| `nvcf_demo.loadgen.errors_total` | counter | `function_name`, `account_name`, `error_kind` | Loadgen-side observed errors (timeouts, 429s, 5xx) before retry |
| `nvcf_demo.synth.tokens_per_sec` | gauge | `function_name`, `function_version_id` | Demo's underlying truth-value for synthesized tokens/sec; the **NVCF-native equivalent the dashboard queries is `stargate_client_model_output_tps`** — included only for unit-testing the synth loop |
| `nvcf_demo.synth.cold_start_seconds` | histogram | `function_name`, `function_version_id` | Cold-start cost the synth loop applied; ground-truth used in tests. **Dashboard queries `nvcf_grpc_proxy_service_session_init_seconds_total{is_reconnect="false"}`** |
| `nvcf_demo.faults.injected_total` | counter | `nvcf_demo.knob_id`, `severity` | Chaos-knob audit trail (Griffin pattern) |
| `nvcf_demo.faults.added_latency_ms` | histogram | `nvcf_demo.knob_id` | Per-knob injected latency |

**Rule of thumb for new dashboards.** If you find yourself reaching for an
`nvcf_demo.*` metric in a Cardinal panel, stop and ask: is there a
native NVCF metric in Table A that carries the same signal? In every
case so far the answer has been yes (Table B's synth metrics are
internal-correctness instruments, not user-facing). The dashboard panel
registry below cites Table A only.

**Trace shape**. Spans use the NVCF-native attribute keys above
(`function_id`, `function_version_id`, `account_name`, `nvca_cluster_name`,
`inference_server_id`, `instance_type`). Span names mirror the components
real NVCF emits from:

```
client.invoke (loadgen)
  └─ http-invocation.invoke      [function_id, function_version_id, account_name, http.status_code]
      └─ llm-request-router.dispatch  [inference_server_id, routing_key, model]
          └─ stargate.queue_wait      [model, queue_size_at_enqueue]
          └─ grpc-proxy.session_init  [is_reconnect — only on cold path]
          └─ stargate.inference       [prompt_tokens, completion_tokens,
                                        time_to_first_token_seconds,
                                        output_tps_at_start,
                                        kv_cache_used_tokens_at_start]
              └─ gpu.kernel           [synthetic 15ms span — visual only]
```

Span names map onto real NVCF components (http-invocation, llm-request-router,
stargate-client) so an inference flame-graph looks like one captured from a
real cluster.

**Synthesis loops**. The function-worker doesn't run a model. It synthesizes
LLM-shaped numbers per request:

- Sample `prompt_tokens` from a per-function lognormal (chat: small,
  batch: huge).
- TTFT = `base_ttft_ms[function] * (1 + load_factor) * chaos_multiplier`
- Tokens/sec = sample from per-function normal, narrowed by GPU util
- GPU util follows a leaky-integrator of recent inference span density per
  GPU. Idle workers settle to 2–8%, busy workers ride 65–92%.
- GPU temp = `40 + 0.4 * util + small_jitter`, capped 92°C. Thermal-throttle
  knob clips util gain when temp > 85°C.
- DCGM metrics emitted at 1Hz from each worker (matches dcgm-exporter
  default).

The synthesis is the demo's most important code path. It lives in
`services/worker/synth/` and is unit-tested for "doesn't drift,
distributions stay sane."

## Chaos knobs (the demo's punchline)

Every knob in this list represents a **real, publicly documented failure
mode** in production GPU-inference serving. Each card below names the
incident the knob is modeled on, NVCF's own handling code/metrics for that
class of failure, and what the operator watching the demo will see.

This is the working-backwards check: if a knob doesn't have a defensible
real-world story, it doesn't ship. Two candidates from the draft list were
cut here — `router.cross-cluster-leak` (no public evidence, NVCF
multi-region architecture is documented but validation gaps are not) and
the original `streaming.connection-leak` framing (no named LLM-provider
postmortem); the second is re-shaped below as `gateway.fd-exhaustion` and
labeled honestly.

Knobs are lifted-and-adapted from `services/controlplane/catalog.go`
(griffin). Each knob is short-lived (auto-clear after 2 min unless held)
and lights up exactly one named dashboard panel.

Quick reference:

| Knob ID | Confidence | Lights up |
|---|---|---|
| `function.cold-start-spike` | strong | Cold-start rate; p99 latency |
| `function.ttft-regression` | strong | TTFT v1-vs-v2 split |
| `function.gpu-oom-flap` | strong | OOM events; replicas_ready sag |
| `function.token-rate-collapse` | strong | Tokens/sec per function |
| `cluster.thermal-throttle` | medium | GPU temp; tokens/sec per cluster |
| `cluster.partial-outage` | strong | Available GPUs; replica drift |
| `router.imbalance` | strong | Invocations per worker heatmap |
| `tenant.noisy-neighbor` | strong | Queue depth; tenant fairness |
| `gateway.fd-exhaustion` | medium (extrapolated) | Streaming connections; gateway errors |
| `registry.fetch-fail` | strong | pull_fail lifecycle events |
| `control-plane.dispatch-lag` | medium | Replica convergence time |
| `quota.exhausted` | strong | HTTP 429 by tenant |

---

### `function.cold-start-spike`

**Real-world story.** Modal's published "GPU Memory Snapshots" benchmark
puts production cold-start cost in concrete numbers: a vLLM function
serving `Qwen2.5-0.5B-Instruct` took **45s for a P0 cold boot vs ~5s warm**;
a Parakeet ASR function 20s vs 2s; a ViT under `torch.compile` 8.5s because
optimized code is hardware-dependent and must be rebuilt after restore.
Replicate publicly acknowledges 30–120s custom-model cold starts. The root
cause is the same everywhere: GPU state (VRAM-resident weights, CUDA
sessions) can't ride along in a process snapshot, so every scale-up replay
pays weight-copy + compile.

**Evidence.**
- NVCF native: `docs/user/grpc-load-test-sli-guide.md:50` —
  `nvcf_grpc_proxy_service_session_init_seconds_total` with
  `is_reconnect="false"` isolates cold sessions; OTel
  `faas.coldstart` attribute is wired through NVCA.
- OSS: vLLM RFC [#34303](https://github.com/vllm-project/vllm/issues/34303)
  "CUDA Checkpoint/Restore for Near-Zero Cold Starts" and
  [#35409](https://github.com/vllm-project/vllm/issues/35409) warm GPU
  worker pooling — cold start is significant enough to warrant CUDA-level
  fixes.
- Industry: [Modal — GPU Memory Snapshots](https://modal.com/blog/gpu-mem-snapshots).

**Knob behavior.** Kill all replicas of target function. Next 1–3
invocations synthesize 25–40s session init; emit
`nvcf_grpc_proxy_service_session_init_seconds_total{is_reconnect="false"}`
plus an extended `function.request.latency` distribution.

**What lights up.** Panel `p_cold_start_init_seconds` on the
per-function dashboard:
```
histogram_quantile(0.95,
  sum(rate(nvcf_grpc_proxy_service_session_init_seconds_total{is_reconnect="false"}[1m]))
    by (function_id, function_version_id, le))
```
Cardinal: `detect_outliers({metric:
"nvcf_grpc_proxy_service_session_init_seconds_total", focus_tags:
["function_id", "is_reconnect"], filter: 'is_reconnect="false"',
direction: "high"})` → top cohort = target `function_id`, z ≥ 4.

---

### `function.ttft-regression`

**Real-world story.** Anthropic's **April 23, 2026 Claude Code
postmortem** is the cleanest public case of a deploy-induced regression
that hid for weeks: on March 4 they shipped a change that lowered the
default reasoning effort from "high" to "medium" to cut latency, and
quality dropped enough to generate six weeks of user complaints across
three overlapping changes each on its own rollout schedule. vLLM's tracker
has the same pattern at the engine layer — [#39790](https://github.com/vllm-project/vllm/issues/39790)
"Significant TTFT regression with Speculative Decoding (EAGLE3)",
[#37308](https://github.com/vllm-project/vllm/issues/37308) "147× TTFT
under Prefix Caching with Asymmetric Batches",
[#35048](https://github.com/vllm-project/vllm/issues/35048) "perf
degradation 0.14.0 → 0.15.1". A new deployment version measurably
slower than its predecessor is one of the highest-frequency real-world
GPU-serving incidents.

**Evidence.**
- NVCF native: latency metrics exist (`function_request_latency`,
  `nvcf_grpc_proxy_service_session_init_seconds_total`) but NVCF has
  **no built-in canary/regression detection** — operators must own it.
  This makes the Cardinal-side detect_outliers call the actual product
  pitch.
- OSS: vLLM
  [#39790](https://github.com/vllm-project/vllm/issues/39790),
  [#37308](https://github.com/vllm-project/vllm/issues/37308),
  [#35048](https://github.com/vllm-project/vllm/issues/35048).
- Industry: [Anthropic — April 23 postmortem on Claude Code](https://www.anthropic.com/engineering/april-23-postmortem).

**Knob behavior.** Multiply TTFT by 2.5× on the target function's newer
`function_version_id` only; older version unchanged. Runs for 5 min so the
regression is statistically distinguishable from noise. Demo emits the
inflated TTFT on `stargate_client_request_time_to_first_token_seconds` and
on the gateway-side `llm_api_gateway_stream_first_token_seconds`.

**What lights up.** Panel `p_ttft_by_function_version` on the
deployment-regression dashboard:
```
histogram_quantile(0.95,
  sum(rate(stargate_client_request_time_to_first_token_seconds_bucket[1m]))
    by (model, function_version_id, le))
```
Cardinal: `detect_outliers({metric:
"stargate_client_request_time_to_first_token_seconds", focus_tags:
["function_id", "function_version_id"], direction: "high"})` → cohort
`{function_id: target, function_version_id: <new>}`.

---

### `function.gpu-oom-flap`

**Real-world story.** vLLM issue
[#21172](https://github.com/vllm-project/vllm/issues/21172) ("Qwen3-32B-AWQ
on A100 40GB under vLLM 0.9.2 + Triton 25.06") reports a reproducible
crash-loop where requested GPU memory creeps upward over time as CUDA
graphs are created — "this process continues over time until I run out of
memory," especially when traffic resumes after an idle pause. vLLM's
official troubleshooting docs name two structural causes: KV-cache
pre-reservation sized for `max_model_len × max_num_seqs`, and CUDA-graph
allocation fragmenting VRAM enough that OOM fires with apparently-free
memory. The community fix (`--enforce-eager`, halved `max_model_len`)
confirms how routinely this is hit in production Triton/KServe stacks.

**Evidence.**
- NVCF native: `nvca_container_crash_total` metric
  (`docs/user/grpc-load-test-sli-guide.md:167`) explicitly defined as
  "Worker pod OOM or crash"; `docs/user/troubleshooting.md:514–550`
  documents OOMKilled detection (exit code 137); health-check callbacks
  on unhealthy containers in `src/libraries/go/worker/health/health.go:51–80`.
- OSS: vLLM
  [#21172](https://github.com/vllm-project/vllm/issues/21172),
  [#37777](https://github.com/vllm-project/vllm/issues/37777),
  [#44181](https://github.com/vllm-project/vllm/issues/44181) (multi-node
  memory leak); TRT-LLM
  [#12228](https://github.com/NVIDIA/TensorRT-LLM/issues/12228).
- Docs: [vLLM Troubleshooting](https://docs.vllm.ai/en/v0.7.2/getting_started/troubleshooting.html).

**Knob behavior.** Return 500/OOM for ~30% of requests on target function.
Worker exits with code 137 on alternating invocations; cluster-agent
restarts it (flap loop). NVCA emits `nvca_container_crash_total` and
`nvca_container_restart_total` for the worker container;
`nvca_workload_result_total{workload_status="failed",
failure_category="oom"}` increments; the gap between
`nvcf_autoscaler.scaling.current_instances` and
`nvcf_autoscaler.scaling.desired_instances` widens during the restart
window.

**What lights up.** Panels `p_nvca_container_crashes` and
`p_replicas_drift` on the GPU/compute fleet dashboard:
```
sum(rate(nvca_container_crash_total[1m])) by (nvca_cluster_name, container)
sum(rate(nvca_workload_result_total{workload_status="failed"}[1m]))
  by (function_id, function_version_id, failure_category)
(nvcf_autoscaler.scaling.desired_instances - nvcf_autoscaler.scaling.current_instances)
```
Cardinal: `investigate_alert({metric: "nvca_container_crash_total",
focus_tags: ["nvca_cluster_name", "container"]})` →
returns spike cohort + correlated `nvca_workload_result_total` cohort
(function + failure_category).

---

### `function.token-rate-collapse`

**Real-world story.** Continuous batching's own pathology: Anyscale's
foundational post quantifies the magnitude — naive static batching falls to
**81 tok/s** as generation-length variance increases, leaving **~60% of
GPU idle**; continuous batching trades that for a different failure mode,
sustained KV-cache pressure (>90% `gpu_cache_usage_perc`) where vLLM
preempts running sequences and throughput collapses. The PagedAttention
paper (Kwon et al., SOSP '23) documents the mechanism formally. Real
deploys hit it: vLLM
[#41306](https://github.com/vllm-project/vllm/issues/41306) (v0.20 MoE
throughput regression),
[#45741](https://github.com/vllm-project/vllm/issues/45741) (NVFP4 decode
collapse on Blackwell),
[#43700](https://github.com/vllm-project/vllm/issues/43700) (INT8
weight-only causing 4× throughput regression at batch=1).

**Evidence.**
- NVCF native: `docs/user/grpc-load-test-sli-guide.md:71–99` defines
  saturation indicators — `nvcf_grpc_proxy_service_active_connections_total`
  and `function_request_total` plateau at the capacity wall;
  `nvcf_grpc_proxy_service_nats_error_total` and `_nats_reconnect_total`
  on the queue path identify NATS as a documented bottleneck.
- OSS: vLLM
  [#41306](https://github.com/vllm-project/vllm/issues/41306),
  [#45741](https://github.com/vllm-project/vllm/issues/45741),
  [#43700](https://github.com/vllm-project/vllm/issues/43700);
  TGI [#3011](https://github.com/huggingface/text-generation-inference/issues/3011).
- Background: [Anyscale — How continuous batching enables 23× throughput](https://www.anyscale.com/blog/continuous-batching-llm-inference);
  [PagedAttention paper (arxiv 2309.06180)](https://arxiv.org/pdf/2309.06180).

**Knob behavior.** Push `stargate_client_model_kv_cache_used_tokens /
stargate_client_model_kv_cache_capacity_tokens` above 90% on target
function, then collapse `stargate_client_model_output_tps` to ~10% of
normal. `stargate_client_model_queue_size` grows in lockstep;
`nvcf_function_queue_depth` rises on the state-metrics-service side;
`stargate_client_nonretryable_failures_total{reason="kv_cache_full"}`
optional on tail.

**What lights up.** Panels `p_kv_cache_pressure` and
`p_output_tps_by_model` on the per-function dashboard:
```
stargate_client_model_kv_cache_used_tokens
  / stargate_client_model_kv_cache_capacity_tokens                 # > 0.9 = preempt regime
stargate_client_model_output_tps                                   # collapse visible
nvcf_function_queue_depth                                          # queue grows
```
Cardinal: `detect_anomalies({metric: "stargate_client_model_output_tps",
focus_tags: ["model"], direction: "low"})` + `detect_outliers({metric:
"stargate_client_model_kv_cache_used_tokens", focus_tags: ["model"],
direction: "high"})` together pinpoint cohort + cause.

---

### `cluster.thermal-throttle`

**Real-world story.** Matt Stancliff's instrumented teardown of a
$1500/mo H100 host is the cleanest public DCGM-shaped example: GPU pinned
at 100% utilization but **clocks throttled between 450 and 550 MHz vs the
1755 MHz rated speed**, temperature climbing **80°C → 90°C → 88°C limit**
in under a minute, power oscillating between 150W and 250W instead of the
350W rated — yielding 25–50% of peak speed across the entire training
window. NVIDIA's spec: H100s begin clock-reducing at 83°C and drop ~15 MHz
per °C above threshold. DCGM exposes the reason at
`DCGM_FI_DEV_CLOCK_THROTTLE_REASONS` / `DCGM_CLOCKS_THROTTLE_REASON_HW_THERMAL`.

**Evidence.** Medium confidence — DCGM behavior is vendor-canonical but
named LLM-provider postmortems are thin. NVCF itself has no thermal
detection (host-level concern).
- Industry: [matt.sh — Cloud GPU thermal throttling](https://matt.sh/cloud-gpu-thermal-throttling).
- Docs: [NVIDIA DCGM Diagnostics](https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/dcgm-diagnostics.html);
  [dcgm-exporter #317](https://github.com/NVIDIA/dcgm-exporter/issues/317)
  (operators wiring `DCGM_EXP_CLOCK_EVENTS_COUNT`).

**Knob behavior.** *Constraint*: NVCF's BYOO DCGM allowlist excludes
`DCGM_FI_DEV_GPU_TEMP` and `DCGM_FI_DEV_SM_CLOCK`, so the demo cannot
surface temperature directly the way real ops would on a dcgm-exporter
scrape. Instead the knob produces the **observable shape of thermal
throttle within the allowlist**: `DCGM_FI_DEV_GPU_UTIL` stays high while
`DCGM_FI_PROF_SM_ACTIVE` and `DCGM_FI_PROF_PIPE_TENSOR_ACTIVE` *drop*,
and `DCGM_FI_DEV_POWER_USAGE` oscillates between rated and ~60% of rated.
This is the genuine NVCF-observable thermal-throttle signature; real
operators infer thermal from exactly these metrics when temp is not
scraped.

**What lights up.** Panels `p_thermal_divergence` and `p_power_oscillation`
on the GPU fleet dashboard:
```
DCGM_FI_DEV_GPU_UTIL - 100 * DCGM_FI_PROF_SM_ACTIVE        # divergence ≠ 0 = throttle
stddev_over_time(DCGM_FI_DEV_POWER_USAGE[2m])              # high stddev = oscillation
```
Cardinal: `detect_outliers({metric: "DCGM_FI_PROF_SM_ACTIVE",
focus_tags: ["nvca_cluster_name", "device"], direction: "low"})` filtered
to GPUs with `DCGM_FI_DEV_GPU_UTIL > 0.8`. Panel includes a
"derived-signal" badge noting the indirection.

---

### `cluster.partial-outage`

**Real-world story.** Meta's "Llama 3 Herd of Models" paper (arxiv
2407.21783, §3.3.3) is the canonical citation. On a **16,384 × H100**
cluster across a **54-day** pre-training snapshot: **466 job interruptions,
419 unexpected**, ~78% confirmed-or-suspected hardware. Largest categories
were **148 (30.1%) GPU/NVLink failures** and **72 (17.2%) HBM3 memory
failures** — roughly **one failure every three hours**. Automation handled
all but three events, but the partial-outage *rate* is the load every
GPU-platform operator has to plan for.

**Evidence.**
- NVCF native: `nvca_instance_type_allocatable` metric
  (`docs/user/grpc-load-test-sli-guide.md:88–99`) tracks per-cluster
  capacity loss; reconciliation loop at
  `src/compute-plane-services/nvca/internal/miniservice/reconcile.go:78–100`
  drives recovery; self-healing on node loss documented at
  `docs/user/autoscaling/architecture.md:52–58`.
- OSS: vLLM
  [#30112](https://github.com/vllm-project/vllm/issues/30112) "Elastic DP
  Rank Recovery: Graceful Handling of GPU Hardware Failures";
  dcgm-exporter
  [#500](https://github.com/NVIDIA/dcgm-exporter/issues/500) and
  [#620](https://github.com/NVIDIA/dcgm-exporter/issues/620) — operators
  monitor XID for fall-off-bus / ECC.
- Industry: [Llama 3 Herd of Models (arxiv 2407.21783)](https://arxiv.org/abs/2407.21783).

**Knob behavior.** Mark 50% of target cluster's GPUs unavailable.
`nvca_instance_type_allocatable{instance_type}` drops while
`nvca_instance_type_capacity` stays flat (capacity = total, allocatable
= currently schedulable). Cluster-agent emits an `nvca_workload_result_total{workload_status="failed",
failure_category="node_unavailable"}` spike during reschedule;
`nvcf_autoscaler.scaling.current_instances` lags
`nvcf_autoscaler.scaling.desired_instances` for the convergence window.

**What lights up.** Panels `p_allocatable_vs_capacity` and
`p_scaling_drift_by_function` on the GPU fleet dashboard:
```
nvca_instance_type_capacity - nvca_instance_type_allocatable     # > 0 = capacity loss
(nvcf_autoscaler.scaling.desired_instances - nvcf_autoscaler.scaling.current_instances)
  > 0
```
Cardinal: `detect_anomalies({metric: "nvca_instance_type_allocatable",
focus_tags: ["nvca_cluster_name", "instance_type"], direction: "low"})`.

---

### `router.imbalance`

**Real-world story.** Anyscale's "Faster First Token with Custom Routing"
post documents the exact failure: Ray Serve's default Power-of-Two-Choices
router picks randomly between two replicas, and "as replica count
increases, the power of two router's random choices quickly decreases KV
cache hit rate" — producing both prefix-cache miss storms and skewed load.
Their `PrefixCacheAffinityRouter` fix delivered **60% TTFT reduction on a
32B model and >40% end-to-end throughput improvement**. The magnitude of
the fix is the magnitude of the original imbalance.

**Evidence.**
- NVCF native: routing methods documented at `docs/user/llm-gateway.md:82`
  (`round_robin`, `power_of_two`, `random`, etc.); session affinity via
  TCP pinning at `docs/user/grpc-function-invocation.md:85–110`. NVCF's
  default routing has the same KV-cache-blind problem Anyscale fixed.
- Industry: [Anyscale — Reduce LLM Inference Latency by 60%](https://www.anyscale.com/blog/ray-serve-faster-first-token-custom-routing).

**Knob behavior.** Router sends 85% of traffic for target function to one
`inference_server_id`. The other servers serve <5% each.
`llm_request_router_requests_total{inference_server_id}` shows the skew;
`llm_request_router_active_inference_servers` confirms the eligible pool
size (so the skew isn't just "the other replicas vanished").

**What lights up.** Panel `p_requests_by_inference_server` on the
per-function dashboard (heatmap, rows = `inference_server_id`):
```
sum(rate(llm_request_router_requests_total{model="$function"}[1m]))
  by (inference_server_id)
```
Cardinal: `detect_outliers({metric: "llm_request_router_requests_total",
focus_tags: ["inference_server_id", "model"], direction: "high"})` →
top cohort = target `inference_server_id`.

---

### `tenant.noisy-neighbor`

**Real-world story.** Anthropic's **Aug 5 – Sep 18, 2025 context-window
routing incident** is a textbook noisy-neighbor case: a load-balancing
change on Aug 29 misrouted short-context Sonnet 4 requests onto servers
configured for 1M-token contexts; routing was sticky, so follow-ups landed
on the same wrong server. **At peak on Aug 31, 16% of Sonnet 4 requests
were affected**, with ~30% of Claude Code users seeing at least one bad
message — one tenant-class's traffic shape degrading another's via shared
infrastructure. Bleed onto Bedrock (0.18%) and Vertex (<0.0004%) was much
smaller, evidence of the shared-infra mechanism.

**Evidence.**
- NVCF native: rate-limit metrics at
  `docs/user/metrics/llm-api-gateway/metrics.md:33–39`
  (`llm_api_gateway_rate_limit_events_applied_total`); `tokenRateLimit`
  in LLM config at `docs/user/llm-gateway.md:84`. NVCF has rate-limits
  but **no per-tenant resource isolation** — exactly the gap Cardinal
  surfaces.
- Industry: [Anthropic — A postmortem of three recent issues](https://www.anthropic.com/engineering/a-postmortem-of-three-recent-issues).

**Knob behavior.** Target `account_name` emits 10× normal request rate.
Loadgen holds other tenants steady so the queue-depth tail clearly
belongs to the noisy one. `nvcf_function_queue_depth` carries
`account_name` natively — the demo is just emitting more under one value.

**What lights up.** Panels `p_queue_depth_by_account` and
`p_p99_latency_by_account` on the tenant-fairness dashboard:
```
sum(nvcf_function_queue_depth) by (account_name, function_id)
histogram_quantile(0.99,
  sum(rate(function_request_latency_bucket[1m]))
    by (account_name, function_id, le))
```
Cardinal: `detect_outliers({metric: "nvcf_function_queue_depth",
focus_tags: ["account_name", "function_id"], direction: "high"})` →
top cohort = target `account_name`. The "starved tenants" panel lists
tenants whose p99 grew >50% in the last 15 min via Cardinal
`detect_anomalies` on `function_request_latency` grouped by
`account_name`.

---

### `gateway.fd-exhaustion`

**Real-world story.** *Extrapolated from documented proxy behavior rather
than a named LLM-provider postmortem* — calling that out honestly. The
underlying mechanism is on the record: nginx ticket
[#2145](https://trac.nginx.org/nginx/ticket/2145) documents a CLOSE_WAIT
socket leak in downstream keepalive connections when the upstream sends a
response before reading the request body — the exact shape SSE token
streams produce. Envoy's published FD-exhaustion guidance warns that
listener connection limits must be set below half the system FD limit to
avoid `accept4() failed (24: Too many open files)`. Every LLM gateway in
production has to defend against this; what's missing is a famous public
"we got bitten" writeup.

**Evidence.**
- NVCF native: QUIC connection eviction tracking via
  `llm_request_router_quic_connection_evictions_total`
  (`docs/user/metrics/llm-request-router/metrics.md`); gRPC session
  lifecycle at `docs/user/grpc-function-invocation.md:111–151`. Idle
  timeouts not documented in operator guides — gap that benefits from
  external monitoring.
- Mechanism: [nginx Trac #2145](https://trac.nginx.org/nginx/ticket/2145);
  [Envoy FD-exhaustion FAQ](https://www.envoyproxy.io/docs/envoy/latest/faq/configuration/resource_limits).

**Knob behavior.** API-gateway opens streaming connections that never
close: `nvcf_grpc_proxy_service_active_connections_total` and
`nvcf_grpc_proxy_service_active_http_requests_total` climb without
bound; `llm_request_router_quic_connection_evictions_total{reason="pool_full"}`
fires once the QUIC pool saturates. Synthetic FD ceiling (configurable,
default 1024) triggers `nvcf_grpc_proxy_service_nats_error_total` and
gateway 5xx via `llm_api_gateway_http_requests_total{status="503"}`.

**What lights up.** Panels `p_active_connections` and `p_quic_evictions`
on the gateway-health dashboard:
```
nvcf_grpc_proxy_service_active_connections_total                # slope wrong
nvcf_grpc_proxy_service_active_http_requests_total              # slope wrong
sum(rate(llm_request_router_quic_connection_evictions_total[1m]))
  by (reason)
sum(rate(llm_api_gateway_http_requests_total{status=~"5.."}[1m]))
  by (status)
```
Cardinal: `detect_anomalies({metric:
"nvcf_grpc_proxy_service_active_connections_total", focus_tags:
["namespace"], direction: "high"})`. Panel description marked
"derived from documented Envoy/nginx FD-exhaustion behavior".

---

### `registry.fetch-fail`

**Real-world story.** Two complementary incidents. (a) **Hugging Face Hub,
April 22–24, 2024 (~49h)**: a MongoDB cache thundering-herd from
`/api/models/.../revision/main` calls slowed or failed the Hub; Node.js
didn't cancel in-flight DB queries when clients disconnected, so abandoned
requests piled up until read replicas had to be manually upsized — model
metadata APIs that scale-up paths depend on were down. (b) **Docker Hub
anonymous pull rate-limit rollout, Nov 2, 2020**: 100 pulls per 6h per
source IP, applied per-IP behind NAT — GKE was treated as anonymous by
default, breaking Kubernetes scale-ups even when nodes had cache. Both
classes still hit production today. vLLM
[#24541](https://github.com/vllm-project/vllm/issues/24541) reports HTTP
429 from HF *even when the model is cached in HF_HOME*.

**Evidence.**
- NVCF native: `docs/user/troubleshooting.md:172–198` documents
  `ImagePullBackOff`/`ErrImagePull` diagnosis;
  `docs/user/troubleshooting.md:6–73` covers credential format failures;
  `docs/user/cluster-management/container-cache.md` describes the cache
  layer NVCF ships precisely to mitigate this class of failure.
- OSS: vLLM
  [#24541](https://github.com/vllm-project/vllm/issues/24541); TGI
  [#346](https://github.com/huggingface/text-generation-inference/issues/346),
  [#1725](https://github.com/huggingface/text-generation-inference/issues/1725),
  [#3088](https://github.com/huggingface/text-generation-inference/issues/3088).
- Industry: [HF — Hub Incident Post Mortem 2024-04-22](https://huggingface.co/blog/mcpotato/hub-incident-post-mortem-20240422);
  [The Register — Docker Hub pull rate limits](https://www.theregister.com/2020/10/28/docker_pull_rate_limit_1_november/).

**Knob behavior.** Control-plane refuses to schedule new replicas of
target function. NVCA emits
`nvca_image_pull_issue_total{image_registry=<target>}` and
`nvca_model_cache_result_total{result="failure",
failure_reason="fetch_failed"}`;
`nvcf_autoscaler.scaling.desired_instances` rises while
`nvcf_autoscaler.scaling.current_instances` stays flat;
`nvca_miniservice_controller_failures_total{failure_reason="image_pull"}`
ticks per attempt.

**What lights up.** Panels `p_image_pull_issues_by_registry` and
`p_model_cache_failures` on the lifecycle dashboard:
```
sum(rate(nvca_image_pull_issue_total[1m])) by (image_registry)
sum(rate(nvca_model_cache_result_total{result="failure"}[1m]))
  by (failure_reason)
(nvcf_autoscaler.scaling.desired_instances - nvcf_autoscaler.scaling.current_instances)
  by (function_id, function_version_id)
```
Cardinal: `investigate_alert({metric: "nvca_image_pull_issue_total",
focus_tags: ["image_registry", "nvca_cluster_name"]})`.

---

### `control-plane.dispatch-lag`

**Real-world story.** Medium confidence — no famous public postmortem,
but NVCF itself surfaces this exact failure mode through dedicated metrics
(see Evidence). The pattern is universal in K8s-style operators: bucket
rebalancing during pod churn causes the autoscaler to transiently skip
scaling decisions, opening a window where desired and ready diverge.

**Evidence.**
- NVCF native: `nvca_controller_runtime_reconcile_errors_total`
  (`docs/user/grpc-load-test-sli-guide.md:168`) tracks controller loop
  failures; bucket-rebalancing-skips documented at
  `docs/user/autoscaling/architecture.md:52–58`; autoscaler loop applying
  `requiredNumberOfInstances` at `docs/user/autoscaling/architecture.md:24–33`.
  This knob exercises behavior NVCF *already instruments for*.

**Knob behavior.** Inject 8s latency between autoscaler computing a new
`desired_instances` and NVCA reconciling. Demo emits artificially-aged
`nvcf_autoscaler.processing.utilization_data_age_milliseconds` (right
shift); `nvca_event_queue_length{nvca_event_name="reconcile"}` grows;
`nvca_event_process_latency` summary p95 stretches; the gap between
`nvcf_autoscaler.scaling.desired_instances` and
`nvcf_autoscaler.scaling.current_instances` opens for the duration.

**What lights up.** Panels `p_reconcile_queue_depth`,
`p_data_age_p95`, and `p_scaling_drift_by_function` on the control-plane
dashboard:
```
nvca_event_queue_length                                          # backlog
histogram_quantile(0.95,
  rate(nvcf_autoscaler.processing.utilization_data_age_milliseconds_bucket[1m]))
(nvcf_autoscaler.scaling.desired_instances - nvcf_autoscaler.scaling.current_instances)
```
Cardinal: `detect_anomalies({metric: "nvca_event_queue_length",
focus_tags: ["nvca_cluster_name", "nvca_event_name"], direction: "high"})`.

---

### `quota.exhausted`

**Real-world story.** Multi-tenant rate-limiting is the most-documented
production failure mode in this list. AWS Bedrock's on-demand throttling
guidance describes the canonical 429 ThrottlingException pattern that
hits when a tenant's per-minute token or request quota is exceeded.
Anthropic's Aug–Sep 2025 context-window incident (cited under
`tenant.noisy-neighbor`) shipped Claude tokens into the wrong quota
buckets, triggering the same class of 429s from the other direction.

**Evidence.**
- NVCF native: token rate limit at `docs/user/llm-gateway.md:84, 200–202`;
  rate-limit event metrics at
  `docs/user/metrics/llm-api-gateway/metrics.md:33–39`; CLI flag
  `--rate-limit` at `docs/user/cli.md:671`. NVCF's quota is
  token/request-based (not GPU-second budgets) — same mechanism as
  Bedrock.
- Industry: [AWS — Bedrock 429 throttling guidance](https://repost.aws/knowledge-center/bedrock-throttling-error);
  [Anthropic postmortem](https://www.anthropic.com/engineering/a-postmortem-of-three-recent-issues).

**Knob behavior.** Force target `account_name`'s gateway-side quota into
"exceeded" state; all invocations return 429 with `Retry-After`.
`llm_api_gateway_http_requests_total{status="429"}` spikes for the target;
`llm_api_gateway_rate_limit_events_applied_total` increments per
applied limiter event. Loadgen-side
`nvcf_demo.loadgen.errors_total{error_kind="429"}` confirms the client
saw it.

**What lights up.** Panels `p_gateway_status_codes` and
`p_rate_limit_events` on the tenant-fairness dashboard:
```
sum(rate(llm_api_gateway_http_requests_total{status="429"}[1m]))
  by (route, status)
sum(rate(llm_api_gateway_rate_limit_events_applied_total[1m]))
```
Cardinal: `detect_outliers({metric: "llm_api_gateway_http_requests_total",
focus_tags: ["status", "route"], filter: 'status="429"', direction:
"high"})`. Tenant attribution joins via `nvcf_function_status` carrying
`account_name` for the function being throttled.

---

### Cut from the original list

- **`router.cross-cluster-leak`** — no public evidence. NVCF's multi-region
  architecture is documented (`docs/user/grpc-function-invocation.md:22–29`,
  `docs/user/generic-http-function-invocation.md:28–35`,
  `docs/user/llm-gateway.md:39–48`), but routing-validation gaps are not.
  No OSS issue or industry postmortem ties cross-cluster misrouting to a
  named incident. Demoing a fault that's "plausible but undocumented"
  weakens the rest of the playbook. Revisit if NVCF publishes a runbook
  or someone files a real issue.

Each surviving knob is one row in `services/controlplane/catalog.go` (port
from griffin), plus an `OnActivate`/`OnClear` hook in the targeted
service.

## Dashboards — Cardinal-first

The Cardinal dashboard pack is the product. **Cardinal is v1; Grafana
JSON is a v2 secondary export** (resolving the earlier open question).
Reason: the whole demo exists to make Cardinal valuable to NVCF
operators; building Grafana first would make the v1 pitch "we ship a
Grafana board" rather than "Cardinal understands NVCF out of the box".
The Grafana export is still in scope — it expands the addressable
audience post-launch — but no panel ships there until it ships in
Cardinal first.

### Cardinal dashboard contract

Every panel below carries:
- **`panel_id`** — stable, kebab-case identifier; never reused; required
  for import/export round-trip and for the per-knob playbook to point at
  panels by name rather than position.
- **`title`** — what the operator reads.
- **`operator_question`** — the single question this panel exists to
  answer. If a panel can't crisply name its question, it's cut.
- **`query`** — the literal expression (PromQL-shape — Cardinal's metric
  query syntax is PromQL-compatible). All metrics reference Table A
  unless explicitly noted.
- **`dimensions`** — the labels (axis, color, slice).
- **`covers_knobs`** — list of `knob_id` values whose "what lights up"
  cites this panel. Drives the acceptance test: every knob's named panel
  must exist and the knob must measurably move it.
- **`cardinal_mcp`** — the `detect_outliers` / `detect_anomalies` /
  `investigate_alert` call that goes with the panel; used by the
  playbook.
- **`source_inspiration`** — the real-world dashboard or doc this panel
  pattern mirrors (vLLM Grafana, DCGM Grafana, NVCF SLI guide, etc.)
  Provided so reviewers can sanity-check that NVCF operators will
  recognize the layout.

### Dashboard layout (six pages, registered panels below)

1. **Executive overview** (page id `nvcf-exec`) — 1 screen, 6 stats + 1
   anomaly feed.
2. **Per-function deep dive** (page id `nvcf-function`) — variables
   `$function_id`, `$function_version_id`.
3. **GPU & compute fleet** (page id `nvcf-gpu`) — variable
   `$nvca_cluster_name`.
4. **Cost attribution** (page id `nvcf-cost`) — the flagship.
5. **Tenant fairness** (page id `nvcf-tenant`) — variable
   `$account_name`.
6. **Deployment regression** (page id `nvcf-regression`) — variables
   `$function_id`, `$function_version_id` (multi-select).

### Panel registry

| panel_id | page | title | operator_question | query | dimensions | covers_knobs | cardinal_mcp | source_inspiration |
|---|---|---|---|---|---|---|---|---|
| `p_cold_start_init_seconds` | nvcf-function | Cold-start session-init p95 | How long are operators waiting on the first request after scale-up? | `histogram_quantile(0.95, sum(rate(nvcf_grpc_proxy_service_session_init_seconds_total_bucket{is_reconnect="false"}[1m])) by (function_id, function_version_id, le))` | x=time, y=p95 seconds, split=function_version_id | `function.cold-start-spike` | `detect_outliers(metric="nvcf_grpc_proxy_service_session_init_seconds_total", focus_tags=["function_id","is_reconnect"], filter='is_reconnect="false"', direction="high")` | NVCF gRPC SLI guide §"Session init" |
| `p_ttft_by_function_version` | nvcf-regression | TTFT p95 v1 vs v2 | Did the new version regress time-to-first-token? | `histogram_quantile(0.95, sum(rate(stargate_client_request_time_to_first_token_seconds_bucket[1m])) by (function_version_id, le))` | x=time, y=p95 seconds, overlay=function_version_id | `function.ttft-regression` | `detect_outliers(metric="stargate_client_request_time_to_first_token_seconds", focus_tags=["function_id","function_version_id"], direction="high")` | NVCF LLM function invocation-path metrics doc |
| `p_function_request_latency` | nvcf-function | Function request latency heatmap | Where does end-to-end latency live across this function's traffic? | `sum(rate(function_request_latency_bucket[1m])) by (function_id, function_version_id, le)` | heatmap: x=time, y=latency bucket | `function.ttft-regression`, `function.cold-start-spike` | `detect_anomalies(metric="function_request_latency", focus_tags=["function_id","function_version_id"], direction="high")` | NVCF state-metrics doc |
| `p_nvca_container_crashes` | nvcf-gpu | NVCA container crashes | Is any container flapping? | `sum(rate(nvca_container_crash_total[1m])) by (nvca_cluster_name, container)` | x=time, y=crashes/sec, split=container | `function.gpu-oom-flap` | `investigate_alert(metric="nvca_container_crash_total", focus_tags=["nvca_cluster_name","container"])` | NVCF gRPC SLI guide §"Worker pod OOM or crash" |
| `p_workload_failures_by_category` | nvcf-gpu | Workload failures by category | Why are workloads failing — OOM, image pull, scheduling? | `sum(rate(nvca_workload_result_total{workload_status="failed"}[1m])) by (function_id, failure_category)` | x=time, y=failures/sec, stack=failure_category | `function.gpu-oom-flap`, `cluster.partial-outage`, `registry.fetch-fail` | `detect_outliers(metric="nvca_workload_result_total", focus_tags=["failure_category","function_id"], filter='workload_status="failed"', direction="high")` | NVCF NVCA metrics doc |
| `p_replicas_drift` | nvcf-function | Replica drift (desired − current) | Is the autoscaler converging? | `nvcf_autoscaler.scaling.desired_instances - nvcf_autoscaler.scaling.current_instances` | x=time, y=gap, split=function_version_id | `function.gpu-oom-flap`, `cluster.partial-outage`, `registry.fetch-fail`, `control-plane.dispatch-lag` | `detect_anomalies(metric="nvcf_autoscaler.scaling.current_instances", focus_tags=["function_id","function_version_id"], direction="low")` | NVCF autoscaler architecture doc §"Bucket model" |
| `p_kv_cache_pressure` | nvcf-function | KV cache utilization | Is this function's KV cache in preempt regime? | `stargate_client_model_kv_cache_used_tokens / stargate_client_model_kv_cache_capacity_tokens` | x=time, y=ratio (0–1), threshold line=0.9 | `function.token-rate-collapse` | `detect_outliers(metric="stargate_client_model_kv_cache_used_tokens", focus_tags=["model"], direction="high")` | vLLM Grafana "KV cache" pattern (Anyscale continuous-batching post) |
| `p_output_tps_by_model` | nvcf-function | Output tokens/sec | Is throughput collapsing? | `stargate_client_model_output_tps` | x=time, y=tps, split=model | `function.token-rate-collapse` | `detect_anomalies(metric="stargate_client_model_output_tps", focus_tags=["model"], direction="low")` | NVCF LLM function invocation-path doc |
| `p_thermal_divergence` | nvcf-gpu | Thermal divergence (util − SM_active × 100) | Are GPUs throttled while showing high util? | `DCGM_FI_DEV_GPU_UTIL - 100 * DCGM_FI_PROF_SM_ACTIVE` | x=time, y=divergence, split=device | `cluster.thermal-throttle` | `detect_outliers(metric="DCGM_FI_PROF_SM_ACTIVE", focus_tags=["nvca_cluster_name","device"], direction="low")` | DCGM throttle inference (`DCGM_CLOCKS_THROTTLE_REASON_HW_THERMAL` not in BYOO allowlist — derived) |
| `p_power_oscillation` | nvcf-gpu | GPU power stddev | Is power oscillating (throttle proxy)? | `stddev_over_time(DCGM_FI_DEV_POWER_USAGE[2m])` | x=time, y=watts stddev, split=device | `cluster.thermal-throttle` | (same as above) | matt.sh H100 thermal teardown |
| `p_allocatable_vs_capacity` | nvcf-gpu | Capacity − allocatable per instance type | How many GPUs went unschedulable? | `nvca_instance_type_capacity - nvca_instance_type_allocatable` | x=time, y=count, split=instance_type | `cluster.partial-outage` | `detect_anomalies(metric="nvca_instance_type_allocatable", focus_tags=["nvca_cluster_name","instance_type"], direction="low")` | NVCF gRPC SLI guide §"Allocatable" |
| `p_requests_by_inference_server` | nvcf-function | Request rate per inference server | Is the router balancing? | `sum(rate(llm_request_router_requests_total[1m])) by (inference_server_id)` | heatmap rows=inference_server_id | `router.imbalance` | `detect_outliers(metric="llm_request_router_requests_total", focus_tags=["inference_server_id","model"], direction="high")` | Anyscale Ray Serve custom-routing post |
| `p_active_inference_servers` | nvcf-function | Routable inference servers | Is the eligible-pool size what we expect? | `llm_request_router_active_inference_servers` | x=time, y=count, split=model | `router.imbalance`, `cluster.partial-outage` | — | NVCF LLM-router metrics doc |
| `p_queue_depth_by_account` | nvcf-tenant | Queue depth per tenant | Which tenant's traffic is growing the queue? | `sum(nvcf_function_queue_depth) by (account_name, function_id)` | stacked area, split=account_name | `tenant.noisy-neighbor` | `detect_outliers(metric="nvcf_function_queue_depth", focus_tags=["account_name","function_id"], direction="high")` | NVCF state-metrics doc |
| `p_p99_latency_by_account` | nvcf-tenant | p99 latency per tenant | Is any tenant being starved? | `histogram_quantile(0.99, sum(rate(function_request_latency_bucket[1m])) by (account_name, le))` | x=time, y=p99 seconds, overlay=account_name | `tenant.noisy-neighbor`, `quota.exhausted` | `detect_anomalies(metric="function_request_latency", focus_tags=["account_name"], direction="high")` | Anthropic context-routing postmortem |
| `p_starved_tenants` | nvcf-tenant | Tenants with p99 +50% in 15min | Quick-glance "who is hurting right now" | rendered from above query via `delta(...)` | table | `tenant.noisy-neighbor` | (above) | — |
| `p_active_connections` | nvcf-function | Gateway active connections | Is the gateway leaking sockets? | `nvcf_grpc_proxy_service_active_connections_total` | x=time, y=count, split=namespace | `gateway.fd-exhaustion` | `detect_anomalies(metric="nvcf_grpc_proxy_service_active_connections_total", focus_tags=["namespace"], direction="high")` | nginx FD-exhaustion / Envoy docs |
| `p_quic_evictions` | nvcf-function | LLM router QUIC evictions | Is the proxy connection pool saturating? | `sum(rate(llm_request_router_quic_connection_evictions_total[1m])) by (reason)` | x=time, y=evictions/sec, stack=reason | `gateway.fd-exhaustion` | — | NVCF LLM-router metrics doc |
| `p_image_pull_issues_by_registry` | nvcf-gpu | Image pull issues per registry | Is a registry broken? | `sum(rate(nvca_image_pull_issue_total[1m])) by (image_registry)` | x=time, y=issues/sec, split=image_registry | `registry.fetch-fail` | `investigate_alert(metric="nvca_image_pull_issue_total", focus_tags=["image_registry","nvca_cluster_name"])` | NVCF troubleshooting doc §"ImagePullBackOff" |
| `p_model_cache_failures` | nvcf-gpu | Model-cache result failures | Are weights failing to materialize? | `sum(rate(nvca_model_cache_result_total{result="failure"}[1m])) by (failure_reason)` | x=time, y=failures/sec, stack=failure_reason | `registry.fetch-fail` | — | NVCF cluster-management container-cache doc |
| `p_reconcile_queue_depth` | nvcf-gpu | NVCA reconcile queue depth | Is the controller falling behind? | `nvca_event_queue_length` | x=time, y=depth, split=nvca_event_name | `control-plane.dispatch-lag` | `detect_anomalies(metric="nvca_event_queue_length", focus_tags=["nvca_event_name","nvca_cluster_name"], direction="high")` | NVCF autoscaler architecture doc |
| `p_data_age_p95` | nvcf-gpu | Autoscaler data age p95 | Are scaling decisions being made on stale data? | `histogram_quantile(0.95, rate(nvcf_autoscaler.processing.utilization_data_age_milliseconds_bucket[1m]))` | x=time, y=ms | `control-plane.dispatch-lag` | — | NVCF autoscaler metrics |
| `p_gateway_status_codes` | nvcf-tenant | Gateway HTTP status codes | Are we rate-limiting or 5xx'ing tenants? | `sum(rate(llm_api_gateway_http_requests_total[1m])) by (status)` | x=time, y=req/sec, stack=status | `quota.exhausted`, `gateway.fd-exhaustion` | `detect_outliers(metric="llm_api_gateway_http_requests_total", focus_tags=["status","route"], filter='status="429"', direction="high")` | NVCF LLM-gateway metrics doc |
| `p_rate_limit_events` | nvcf-tenant | Rate-limit events applied | How often is the limiter firing? | `sum(rate(llm_api_gateway_rate_limit_events_applied_total[1m]))` | x=time, y=events/sec | `quota.exhausted` | — | NVCF LLM-gateway metrics doc |
| `p_invocations_per_second` | nvcf-exec | Invocations/sec | Are we serving traffic? | `sum(rate(function_request_total[1m]))` | sparkline | — (always-on) | — | NVCF gRPC SLI guide |
| `p_active_functions_versions_tenants` | nvcf-exec | Active functions/versions/tenants | Fleet size at a glance | `count(count by (function_id)(nvcf_function_status))`, `count(count by (function_version_id)(nvcf_function_status))`, `count(count by (account_name)(nvcf_function_status))` | three stats | — | NVCF state-metrics doc |
| `p_fleet_gpu_util` | nvcf-exec | Cluster GPU utilization | How busy are the GPUs overall? | `avg by (nvca_cluster_name)(DCGM_FI_DEV_GPU_UTIL)` | gauge per nvca_cluster_name | — | DCGM Grafana standard |
| `p_p95_invocation_latency` | nvcf-exec | p95 invocation latency (1h delta) | Is latency drifting? | `histogram_quantile(0.95, sum(rate(function_request_latency_bucket[5m])) by (le))` | stat with 1h delta | `function.ttft-regression`, `function.cold-start-spike` | (covered by per-function detect_anomalies) | — |
| `p_anomaly_feed` | nvcf-exec | Cardinal anomaly feed | What just broke? | (rendered from Cardinal `detect_anomalies` over all native metrics, last 15min) | list, sorted by score | all | `detect_anomalies(focus_tags=["function_id","account_name","nvca_cluster_name"])` periodic | — |

**Cost-attribution dashboard (page `nvcf-cost`)** — this is the flagship,
and the queries are derived rather than direct emissions. They live in a
**Cardinal saved-query template** so the math can be hand-tuned without
touching panel JSON. Open question (deferred to M2): the exact join shape
between `DCGM_FI_DEV_GPU_UTIL × time` and `function_request_total` so a
GPU-second can be attributed to a `function_id`+`account_name` pair. The
proxy is `nvcf_function_instances_current × time × $/GPU-hour` for the
instance type — coarse but credible, and the only shape that works given
DCGM utilization carries no `function_id` label.

### Acceptance tests

The `integration/` suite asserts, per knob:
1. The panel(s) named in the knob's "What lights up" exist in the
   exported Cardinal dashboard JSON (panel_id lookup).
2. Activating the knob and waiting `wait_seconds` causes the panel's
   query to return a value that crosses the knob's `expected_delta`
   relative to the pre-knob baseline.
3. The knob's `cardinal_mcp` call returns a cohort matching `expected_cohort`.

These three checks together guarantee the playbook works end-to-end and
catches drift between knob, metric, panel, and MCP call.

### Dashboard build path

`dashboards/cardinal/author.py` generates Cardinal JSON from the panel
registry in `dashboards/registry.yaml` (single source of truth for the
table above). A second adapter writes Grafana JSON from the same
registry in M5. Registry → Cardinal JSON is a fast-path target for
M1 acceptance.

## Telemetry contract — single source of truth

`common/telemetry/contract.go` defines every metric name, attribute key,
allowed-value set, and span name as Go constants. All services import from
here. Dashboards key off the same constants (generator reads them).

This makes the "rename a thing" cost low and keeps the demo coherent. Drift
between services + dashboards is the #1 way these demos rot.

## Cardinal-detect playbook

`docs/demo/cardinal-detection.md` — one section per knob. Each section:

```
### knob_id: function.ttft-regression

Activate:    POST /admin/faults { id: "function.ttft-regression",
                                  args: {function_id: <target>} }
Wait:        90 seconds
Run:         detect_outliers({
               metric_name: "stargate_client_request_time_to_first_token_seconds",
               focus_tags: ["function_id", "function_version_id"],
               direction: "high"
             })
Expect:      Top cohort {function_id: <target>,
                         function_version_id: <new>}, z >= 3
Panel:       p_ttft_by_function_version (page nvcf-regression)
```

This file is what an SE reads from on the demo. Each knob takes ~3 min
end-to-end including narration. The panel_id is the contract — when the
dashboard is reorganized, the playbook keeps working as long as panel_id
is stable.

## Deploy

### Laptop (`docker-compose up`)

Uses griffin's existing `docker-compose.yml` — adds one service:

```yaml
nvcf:
  build: .
  environment:
    SERVICE_NAME: nvcf
    OTEL_EXPORTER_OTLP_ENDPOINT: ${OTEL_EXPORTER_OTLP_ENDPOINT}
    OTEL_INSECURE: "true"
    CONTROLPLANE_URL: http://controlplane:8086
  ports:
    - "9998:9998"   # /faults/* HTTP for direct knob activation (per dbaas/solar pattern)
  depends_on:
    - controlplane
```

No new collector, no MinIO, no Lakerunner-all-in-one container in this
spec — griffin's existing OTel collector setup carries NVCF metrics out
the same way airtel and adani metrics already flow. Cardinal Cloud
endpoint via the standard `OTEL_EXPORTER_OTLP_ENDPOINT` env.

### Kubernetes (`kubectl apply -k k8s/overlays/nvcf-prod`)

Mirrors `k8s/overlays/adani-prod/` and `k8s/overlays/airtel-prod/`.
Adds:
- One `Deployment` of griffin with `SERVICE_NAME=nvcf` env.
- One `Service` exposing :9998 for `kubectl port-forward` knob activation.
- A `dashboards/` folder with the generated Cardinal JSON.
- Kustomize overlays for `with-cardinal-cloud` and `with-self-hosted`.

The "one pod, all personas" model means a production demo cluster can
also bring up the e-commerce + dbaas + solar + nvcf personas
simultaneously in a single namespace — Cardinal SE flips between
dashboard pages without redeploy.

## Files added or changed in griffin

```
griffin-commerce-demo/
├── cmd/
│   └── nvcf.go                   # NEW — Cobra subcommand: `griffin nvcf`
├── entrypoint.sh                 # CHANGED — add `nvcf)` case
├── services/
│   └── nvcf/                     # NEW — entire package
│       ├── main.go               # Start() — SetupTelemetry + Register + StartHTTPServer + block
│       ├── catalog.go            # NewCatalog() — fleet entities (4 funcs × 2 versions × 4 accounts × 2 clusters)
│       ├── state.go              # entity types: Function, FunctionVersion, Account, Cluster, Instance, InferenceServer
│       ├── metrics.go            # Table A metric registrations; instruments struct; observer callbacks
│       ├── scenario.go           # 11 chaos-knob Profiles; Activate/Clear; RampedValue; trapezoidFactor reused from dbaas pattern
│       ├── httpserver.go         # :9998 /faults/{activate,clear,status} per dbaas pattern
│       ├── logs.go               # slog-shaped emitter for the few span/log samples we synthesize
│       └── *_test.go             # unit tests per file
├── services/controlplane/
│   └── catalog.go                # CHANGED — add 11 NVCF knob IDs (function.cold-start-spike, ...,
│                                 #            so the existing /admin/faults UI lists them alongside e-commerce knobs)
├── docs/specs/
│   └── nvcf.md                   # NEW — this spec
├── docs/demo/
│   └── nvcf-cardinal-detection.md # NEW — per-knob playbook (see "Cardinal-detect playbook" section)
├── k8s/overlays/
│   └── nvcf-prod/                # NEW — Kustomize overlay
│       ├── kustomization.yaml
│       ├── dashboards/           # generated Cardinal JSON, source: dashboards/registry.yaml + author.py
│       ├── values-local.yaml     # overlay-specific config
│       └── with-{cardinal-cloud,self-hosted}/   # sub-overlays for telemetry sink choice
└── dashboards/                   # NEW top-level dir
    ├── registry.yaml             # panel registry from spec — single source of truth
    └── nvcf/
        ├── author.py             # registry.yaml → Cardinal JSON (v1); Grafana JSON (v2 / M5)
        ├── cardinal/             # generated
        └── grafana/              # generated (M5)
```

**Files NOT changed**: `common/telemetry.go`, `common/middleware.go`,
`common/faults/*`, `services/{cart,catalog,payment,shipping,images,recommendations,dbaas,solar}/*`,
`loadgen/*`, `frontend/*`. The NVCF persona is additive.

## config.yaml shape

```yaml
# Seeded so labels match real NVCF: instance_type, nvca_cluster_name,
# account_name, function_id, function_version_id.

clusters:
  - nvca_cluster_name: us-west-2-a       # matches NVCA's label key
    nvca_cluster_group: gpu-heavy
    nodes: 2
    instance_type: NCP.GPU.A100_80GB_1x  # matches `instance_type` enum
  - nvca_cluster_name: us-east-1-a
    nvca_cluster_group: gpu-heavy
    nodes: 2
    instance_type: NCP.GPU.H100_80GB_1x

# Cost model (used by nvcf-cost saved query).
instance_pricing:
  NCP.GPU.A100_80GB_1x: 1.85   # $/instance-hour
  NCP.GPU.H100_80GB_1x: 4.50

functions:
  # function_id is UUID; function_name is demo-only sugar.
  - function_id: 11111111-1111-1111-1111-111111111111
    function_name: chat-helpful
    workload_type: streaming           # demo-only persona hint
    base_ttft_ms: 180
    base_output_tps: 95
    prompt_tokens_lognormal: { mu: 5.2, sigma: 0.7 }
    versions:
      - function_version_id: a1111111-1111-1111-1111-111111111111
      - function_version_id: a2222222-2222-2222-2222-222222222222
    pinned_clusters: [us-west-2-a, us-east-1-a]
  - function_id: 22222222-2222-2222-2222-222222222222
    function_name: summarize-doc
    workload_type: http
    base_ttft_ms: 410
    base_output_tps: 70
    prompt_tokens_lognormal: { mu: 8.0, sigma: 0.5 }
    versions:
      - function_version_id: b1111111-1111-1111-1111-111111111111
      - function_version_id: b2222222-2222-2222-2222-222222222222
  - function_id: 33333333-3333-3333-3333-333333333333
    function_name: fraud-detect
    workload_type: grpc
    base_ttft_ms: 22
    base_output_tps: null                # latency-only, no token shape
    versions: [{ function_version_id: c1111111-1111-1111-1111-111111111111 }]
  - function_id: 44444444-4444-4444-4444-444444444444
    function_name: embed-text
    workload_type: http
    base_ttft_ms: 35
    base_output_tps: null
    versions: [{ function_version_id: d1111111-1111-1111-1111-111111111111 }]

# account_name is NVCF's tenant key; account_display_name is human label.
accounts:
  - account_name: acme
    account_display_name: Acme Inc.
  - account_name: globex
    account_display_name: Globex Corp.
  - account_name: initech
    account_display_name: Initech LLC
  - account_name: umbrella
    account_display_name: Umbrella Corp.
```

## Open questions to resolve before build

1. **Cardinal dashboard JSON shape.** Need a current schema export from a
   live Cardinal dashboard to base the generator's adapter on. Otherwise
   ship only Grafana JSON in v1.
2. **Lakerunner all-in-one container.** Does an "all-in-one" image exist
   today, or do we need to compose it from existing service binaries with a
   minimal Postgres + MinIO? If the latter, document the wiring in
   `docs/customizing.md`.
3. **Cost-attribution join shape.** Most differentiated panel; the only
   one where the panel query is non-trivial. DCGM utilization carries no
   `function_id`, so attribution requires joining
   `nvcf_function_instances_current × time × $/instance-hour` per
   `instance_type`. Need to prototype against a real Lakerunner before
   the saved-query template is final.
4. **Grafana export shape.** Cardinal is v1 (resolved). Grafana JSON is
   v2 secondary export — verify Cardinal's PromQL surface translates
   cleanly to Grafana's, especially for the
   `nvcf_autoscaler.processing.utilization_data_age_milliseconds`-style
   names with dots (need a dot→underscore relabel rule at export).
5. **Repo location.** New repo `cardinalhq/nvcf-demo` (public, OSS), or
   subfolder under `cardinalhq/lakerunner-demos/nvcf/`? Recommend
   dedicated repo for launch SEO; revisit at 2nd demo.
6. **Metric name dot-vs-underscore.** NVCF emits a mix:
   `nvcf_grpc_proxy_service_*` (underscore, gRPC-proxy Go),
   `function.request` / `function.request.latency` (dot, http-invocation
   Rust), `nvcf_autoscaler.scaling.*` (mixed). Cardinal needs to ingest
   both. Demo should emit *exactly the same mix* (don't normalize) so the
   dashboard queries are identical to what real NVCF produces. Confirm
   Cardinal's collector tolerates dot-named OTel metrics.

## Milestones

- **M1 — services/nvcf scaffold + first knob (2-3 days inside griffin).**
  Add `cmd/nvcf.go`, `entrypoint.sh` case, `services/nvcf/{main,catalog,
  state,metrics,scenario,httpserver,logs}.go`. Seed the fleet. Register
  a strict subset of Table A: `function_request_total`,
  `function_request_latency`, `stargate_client_request_time_to_first_token_seconds`,
  `DCGM_FI_DEV_GPU_UTIL`, `nvca_container_crash_total`,
  `nvcf_function_queue_depth`, `nvcf_autoscaler.scaling.{current,desired}_instances`.
  Implement `function.ttft-regression` knob end-to-end with unit test.
  Add 11 knob IDs to controlplane catalog (only ttft-regression has a
  scenario impact spec wired in M1; rest are stubs).
  **Acceptance**: `make check` green; `docker compose up nvcf` emits
  metrics; activating ttft-regression bends `stargate_client_request_time_to_first_token_seconds` for the right `function_version_id` only.
- **M2 — remaining Table A metrics + 3 more knobs (3-4 days).** Full
  DCGM BYOO allowlist, full stargate metric set, autoscaler metrics,
  NVCA metric set. Wire `cluster.thermal-throttle` (derived signal),
  `router.imbalance`, `tenant.noisy-neighbor` scenario impact specs.
- **M3 — remaining 7 knobs + dashboards/registry.yaml + Cardinal JSON gen (3-4 days).**
  All 11 knobs functional. `dashboards/nvcf/author.py` generates Cardinal
  JSON from the spec's panel registry. `docs/demo/nvcf-cardinal-detection.md`
  per-knob playbook.
- **M4 — k8s/overlays/nvcf-prod + acceptance suite (2 days).** Kustomize
  overlay, with-cardinal-cloud sub-overlay, screenshots in README,
  walkthrough script. Integration test per knob: activate → wait →
  assert panel query crosses threshold and Cardinal MCP cohort matches.
- **M5 — Grafana export (optional, 1-2 days).** Add Grafana JSON adapter
  to `dashboards/nvcf/author.py` driven from the same `registry.yaml`.

## What this demo is **not**

- Not a benchmark — don't draw perf conclusions from these numbers.
- Not a reference for "how to build an NVCF function" — for that, see
  `nvcf/examples/`.
- Not a substitute for talking to a real NVCF cluster when stress-testing
  the Lakerunner integration. The demo verifies dashboards and the
  detection playbook; production verification needs real traffic.
