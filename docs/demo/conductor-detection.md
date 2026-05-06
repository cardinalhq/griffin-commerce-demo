# Conductor Detection — Demo Script

This is the per-knob playbook used when demoing fault injection against
Conductor's `detect_anomalies` and `detect_outliers` tools (see
`~/workspace/conductor/packages/mcp-gateway/lakerunner/tools/`).

## Prerequisites

1. Stack is running with the chaos overlay applied:
   `kubectl apply -k k8s/overlays/chaos`
2. OTLP is wired to a lakerunner instance (compose with one of the
   `with-otlp-*` overlays).
3. Locust is generating baseline traffic (≥ 5 minutes of warm-up before
   activating any knob — without baseline, `detect_anomalies` has nothing
   to compare against and `detect_outliers` has too few peer samples).
4. Admin UI is reachable at `<frontend>/chaos`.

## Demo flow shape

For every knob:

1. Wait for ≥ 5 min of baseline traffic.
2. Activate the knob via `<frontend>/chaos`.
3. Wait 60–120 s of fault traffic.
4. Run the **primary detection** call below — its output is the demo
   punchline.
5. Clear the knob; confirm a follow-up call returns
   `IsAnomalous=false` after a ~30 s settle.

The "primary" tool/argument set for each knob is documented under the
canonical scenario plan in `docs/plans/fault-injection.md` —
the tool calls below match that plan exactly.

---

## 1. catalog.error — outlier on a target product

**Activate**: knob `catalog.error`, `target=PROD-001`, `statusCode=500`.

**Primary** (`detect_outliers`):

```json
{
  "metric_name": "griffin.catalog.product.requests_total",
  "focus_tags": ["product_id", "http_status_code"],
  "direction": "high",
  "start": "now-10m"
}
```

Expect: top cohort with `tags={product_id: "PROD-001", http_status_code: "500"}`,
peak z-score > 3.

**Secondary** (`detect_anomalies`):

```json
{
  "metric_name": "griffin.catalog.product.requests_total",
  "filters": {"http_status_code": "500"},
  "direction": "high",
  "compare_window": "previous"
}
```

Expect `IsAnomalous=true`, `count_ratio` ≫ 1.25 vs near-zero baseline.

**Tertiary cascade** (`detect_anomalies`):

```json
{
  "metric_name": "griffin.cart.operations_total",
  "filters": {"http_status_code": "500", "operation": "add"},
  "direction": "high"
}
```

Confirms the cascade through cart.

---

## 2. catalog.slow — anomaly on catalog p99

**Activate**: knob `catalog.slow`, `latencyMs=2000`.

**Primary** (`detect_anomalies`):

```json
{
  "metric_name": "http.server.duration",
  "filters": {"service_name": "catalog-service"},
  "aggregation": "p99",
  "direction": "high",
  "compare_window": "previous"
}
```

Expect `p99_ratio` ≈ `latencyMs / baseline_p99`.

**Secondary** (`detect_outliers`):

```json
{
  "metric_name": "http.server.duration",
  "focus_tags": ["service_name", "http_route"],
  "direction": "high",
  "start": "now-10m"
}
```

Cart's outbound spans climb due to connection pool saturation —
useful "which service is the root" demo.

---

## 3. cart.error — service-level error spike

**Activate**: knob `cart.error`, `probability=0.5`.

**Primary** (`detect_anomalies`):

```json
{
  "metric_name": "griffin.cart.operations_total",
  "filters": {"http_status_code": "500"},
  "direction": "high",
  "compare_window": "previous"
}
```

`count_ratio` ≈ `probability / baseline_error_rate`.

**Secondary** (`detect_outliers`):

```json
{
  "metric_name": "griffin.cart.operations_total",
  "focus_tags": ["service_name", "http_status_code"],
  "direction": "high"
}
```

Demonstrates "cart-service / 500" as the outlier cohort vs healthy peers.

---

## 4. cart.outlier — long-tail latency

**Activate**: knob `cart.outlier`, `probability=0.05`, `latencyMs=30000`.

**Primary** (`detect_anomalies`):

```json
{
  "metric_name": "griffin.cart.operation.duration_ms",
  "aggregation": "p99",
  "direction": "high",
  "compare_window": "previous"
}
```

Canonical "long tail without average shift": p99 jumps, p50 holds.

**Note**: `detect_outliers` is the wrong tool here — the dimension is
*temporal*, not cohort. Useful teaching moment for the demo.

---

## 5. payment.fail — per-processor outlier

**Activate**: knob `payment.fail`, `target=kittycard`, `probability=0.8`.

**Primary** (`detect_outliers`):

```json
{
  "metric_name": "griffin.payment.charges_total",
  "focus_tags": ["processor", "status"],
  "direction": "high",
  "start": "now-10m"
}
```

Expect cohort `tags={processor: "kittycard", status: "failed"}`.

**Secondary** (`detect_anomalies`):

```json
{
  "metric_name": "griffin.payment.charges_total",
  "filters": {"status": "failed"},
  "direction": "high",
  "compare_window": "previous"
}
```

`count_ratio` matches the lift vs YAML baseline (kittycard 20% → 80% = 4×).

---

## 6. payment.gc-storm — STW pause jitter

**Activate**: knob `payment.gc-storm`, `latencyMs=200` (interval).

**Primary** (`detect_anomalies`, full PromQL):

```json
{
  "query": "histogram_quantile(0.99, sum by (le, service_name)(rate(process_runtime_go_gc_pause_ns_bucket[1m])))",
  "direction": "high",
  "compare_window": "previous"
}
```

Expect `p99_ratio` > 5×.

**Secondary** (`detect_outliers`):

```json
{
  "metric_name": "process.runtime.go.gc.pause_ns",
  "focus_tags": ["service_name"],
  "direction": "high"
}
```

payment-service stands out; needs ≥ 3 services in dataset (Griffin has 6).

**Tertiary** (`detect_anomalies`, user-facing impact):

```json
{
  "metric_name": "griffin.payment.charge.duration_ms",
  "aggregation": "p99",
  "direction": "high"
}
```

---

## 7. shipping.fail — per-carrier outlier

**Activate**: knob `shipping.fail`, `target=catcarrier`, `probability=0.9`.

**Primary** (`detect_outliers`):

```json
{
  "metric_name": "griffin.shipping.shipments_total",
  "focus_tags": ["carrier", "status"],
  "direction": "high"
}
```

Expect cohort `tags={carrier: "catcarrier", status: "failed"}` vs the
two healthy carriers.

**Secondary** (`detect_anomalies`):

```json
{
  "metric_name": "griffin.shipping.shipments_total",
  "filters": {"carrier": "catcarrier", "status": "failed"},
  "direction": "high",
  "compare_window": "previous"
}
```

---

## 8. images.slow — single-service latency outlier

**Activate**: knob `images.slow`, `latencyMs=1500`.

**Primary** (`detect_outliers`):

```json
{
  "metric_name": "http.server.duration",
  "focus_tags": ["service_name"],
  "direction": "high"
}
```

`image-service` is the outlier; other services hold steady.

**Secondary** (`detect_anomalies`):

```json
{
  "metric_name": "http.server.duration",
  "filters": {"service_name": "image-service"},
  "aggregation": "p99",
  "direction": "high",
  "compare_window": "previous"
}
```

---

## 9. recs.memleak — RSS climb

**Activate**: knob `recs.memleak` (no params).

**Primary** (`detect_outliers`):

```json
{
  "metric_name": "process.resident_memory_bytes",
  "focus_tags": ["service_name"],
  "direction": "high"
}
```

`recommendations-service` stands out; other Griffin services share a
near-identical baseline RSS.

**Secondary** (`detect_anomalies`):

```json
{
  "metric_name": "process.resident_memory_bytes",
  "filters": {"service_name": "recommendations-service"},
  "direction": "high",
  "compare_window": "previous",
  "start": "now-2m"
}
```

Best ≥ 1 minute into the leak so the ramp is visible.

**Tertiary** (`detect_anomalies`):

```json
{
  "metric_name": "process.runtime.go.gc.pause_ns",
  "filters": {"service_name": "recommendations-service"},
  "aggregation": "p99",
  "direction": "high"
}
```

Heap growth → GC pressure rises.

---

## 10. global.cpu-burn-traffic — per-request CPU burn

**Activate**: knob `global.cpu-burn-traffic`, `latencyMs=50`.

**Primary** (`detect_anomalies`):

```json
{
  "metric_name": "http.server.duration",
  "aggregation": "p99",
  "direction": "high",
  "compare_window": "previous"
}
```

`p99_ratio` ≈ `(baseline + latencyMs) / baseline`.

**Secondary** (`detect_anomalies`):

```json
{
  "metric_name": "process.cpu.utilization",
  "direction": "high",
  "compare_window": "previous"
}
```

No `detect_outliers` expected — load is broad. That's the demo.

---

## 11. global.cpu-burn-bg — background CPU saturation

**Activate**: knob `global.cpu-burn-bg`, `latencyMs=0` (full burn).

**Primary** (`detect_anomalies`):

```json
{
  "metric_name": "process.cpu.utilization",
  "direction": "high",
  "compare_window": "previous"
}
```

**Secondary** (`detect_outliers`): degenerate at full burn (every service
saturates equally → no outlier). Document as the "anomaly catches it,
outlier correctly finds nothing" teaching moment.

---

## 12. cart.poison-product — logs explain the failure

**Activate**: knob `cart.poison-product`, `target=PROD-003`.

**This is the trace → side-drawer → logs UX demo.** No automated tool
call is the punchline; the operator's clicks are.

1. From `<frontend>/chaos`, activate the knob.
2. Open the storefront in another tab, add PROD-003 to a cart.
3. Click "Add to cart" again, or just refresh the cart view.
4. Observe a **generic 500** with body
   `"The service encountered an unexpected error"`.
5. Open the trace in Conductor's trace UI, click the failed cart span,
   open the new side drawer, switch to the logs tab.
6. The log query is `{trace_id="<id>"}`. The result names the cause:
   `"cart contains tainted item: poison product detected"` with
   `product_id`, `cart_id`, `griffin.fault`, and a structured `cause`
   field.

**Secondary** (`detect_anomalies`, log mode):

```json
{
  "log_selector": "{service_name=\"cart-service\", level=\"error\"}",
  "log_metric": "count_over_time",
  "direction": "high",
  "compare_window": "previous"
}
```

Sharp rise in cart error logs vs the previous window. Not the headline
detection — the log-correlation UX is.

---

## 13. loadgen.flood — traffic spike

**Activate**: knob `loadgen.flood`, `probability=1.0` (full ramp),
`latencyMs=30000` (ramp duration).

**Primary** (`detect_anomalies`):

```json
{
  "metric_name": "http.server.duration",
  "direction": "high",
  "compare_window": "previous",
  "detect_sparse": true
}
```

Large positive `count_ratio`.

**Secondary** (`detect_anomalies`, p99):

```json
{
  "metric_name": "http.server.duration",
  "aggregation": "p99",
  "direction": "high"
}
```

Saturation latency rise alongside RPS.

As one service saturates first (frontend), `detect_outliers` on
`http.server.duration` with `focus_tags=["service_name"]` may then flag
the saturated service — useful "watch the bottleneck propagate" step.

---

## A typical demo session (4 knobs, ~15 min)

Chain these in order so each detection style gets airtime:

| Step | Knob | Detection style | Story |
|---|---|---|---|
| 1 | `catalog.error` (target=PROD-001) | `detect_outliers` | "PROD-001 is the bad product." |
| 2 | `payment.gc-storm` | `detect_anomalies` p99 | "payment p99 is 5× baseline." |
| 3 | `catalog.slow` | trace cascade | "the slowness propagates from catalog → cart → frontend." |
| 4 | `cart.poison-product` (target=PROD-003) | trace → side-drawer logs | "the trace shows a 500, the logs explain why." |

Total fault budget per session: < 8 min of injected fault, > 7 min of
baseline recovery between knobs (so each `compare_window: "previous"`
window is clean).
