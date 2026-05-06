# Fault Injection Plan

Add otel‑demo–style runtime fault injection to Griffin Commerce Demo. UI‑driven, single‑knob‑at‑a‑time. Each demo is one toggle that produces a clear, observable signal — and **each signal is engineered to be caught by Conductor's `detect_anomalies` or `detect_outliers` tools** at `~/workspace/conductor/packages/mcp-gateway/lakerunner/tools/`.

## Goals

- One central control plane holding **at most one active knob** at any time. Activating a new knob atomically replaces the previous one.
- Knob state is observable from any UI page (active‑fault banner) and changeable from a dedicated admin page.
- Each scenario is engineered to **actually move a metric** — see "Fidelity notes" under each knob.
- Each scenario has an **explicit detection contract** specifying the Conductor tool, arguments, and expected output that catches it (see "Detection model" and per‑knob "Detection" entries).
- Off by default; a `GRIFFIN_ADMIN_ENABLED=true` env on the control plane is the master kill switch.

## Non‑goals (v1)

- Multi‑knob composition / scenario bundles (forbidden by the single‑knob constraint).
- Persistence of knob state across control‑plane restarts (in‑memory only).
- Authentication beyond the env‑var kill switch.
- Rich RBAC, audit, change history beyond a 100‑event ring buffer.

---

## Architecture

### Components

| Component | Role |
|---|---|
| `services/controlplane` (new) | Source of truth for the single active knob. Port `:8086`. Exposes admin REST + SSE event stream. |
| `common/faults` (new) | Knob types, polling client, OTel emission helpers, generic middleware. |
| Per‑service hook calls | Site‑specific code inside catalog / cart / payment / images / recs that consults the local `faults.Client.Active()`. |
| `frontend/src/routes/admin/faults` (new) | Single‑page UI: active‑fault card + knob list + event log. |
| Vite proxy | New entry forwards `/admin/faults*` → `controlplane:8086`. |

### Distribution model

Polling, not push. Every product service starts a `faults.Client` goroutine that calls `GET /admin/faults` every **1 s ± 200 ms jitter**, swaps the active `*Knob` via `atomic.Pointer`, and runs side‑effect transitions (spawn/stop background goroutines). 6 services × 1 RPS = trivial overhead, no SSE plumbing on the consumer side, idempotent on missed updates.

### Single‑knob enforcement

State on the control plane is `active *Knob` behind a `sync.Mutex`. `PUT /admin/faults` replaces it; `DELETE` clears it. There is no merging, no list, no priority ordering. Two concurrent PUTs → last writer wins; both events appear in the SSE log.

### Service dispatch rule

A polling service ignores knobs whose `Service` field doesn't match its own service name, **except** for the `global` namespace which every service applies. Example:

- `catalog.error` → only `catalog` acts; cart/payment/images/recs ignore.
- `global.cpu-burn-bg` → every service spawns burn goroutines.

---

## Detection model (Conductor integration)

Conductor's mcp-gateway exposes two relevant tools (definitions in `~/workspace/conductor/packages/mcp-gateway/lakerunner/tools/`):

### `detect_anomalies` — window vs window

Compares a metric or log stream across two time windows:
- **Trigger logic**: `direction=high` flags when `p50_current > 1.25 × p50_previous` **AND** `p99_current > 1.25 × p99_previous`. `direction=low` flags the inverse. `detect_sparse=true` additionally flags when the data count drops by sparseness > 0.5 (used for "fewer requests" scenarios).
- **Resolution doubling**: runs both at the full window and at the second half, so a fault that exists for only the latter portion of the window is still caught.
- **Inputs of interest**: `metric_name` (or `query` for raw PromQL, or `log_selector` for LogQL), `start`, `end`, `compare_window` ∈ {`previous`, `1w`}, `direction`, optional `group_by`.
- **Demo implication**: every "spike" / "slowdown" knob must be activated *after* a baseline window has accumulated. The recommended demo pattern is **5 minutes of baseline → activate knob → wait 5 minutes → call `detect_anomalies` with `start=now-5m`, `compare_window=previous`**. Without baseline, the previous window is empty and nothing useful comes out.

### `detect_outliers` — peer cohort drilldown

Finds tag/value combinations that stand out vs their peers using modified z‑score:
- **`focus_tags` is required** (1–3 tag dimensions). The tool drills recursively (depth 3, max 20 queries) across tag combinations from `focus_tags`. Recommended sets: `["service_name","http_route","http_status_code"]` for HTTP, plus our custom `["service_name","product_id"]`, `["service_name","payment_processor"]`, `["service_name","shipping_carrier"]`.
- **Cardinality requirement**: at least **3 peer values** per tag dimension are needed for z‑score to be meaningful (`scoreConditionedResults` returns nil if `len(rows) < 3`). Most knobs need a fault that targets one value out of N≥3 (one product, one processor, one carrier).
- **Demo implication**: emit metrics with the *right* label cardinality so the tool can find a cohort. Default OTel HTTP metrics give `service_name × http.route × http.status_code` only — fine for "which route is failing" but blind to "which product / processor / carrier is failing." We add custom counters and histograms with these labels (see "Custom metrics" below).

### Custom metrics required for outlier drilldown

Add these emissions in `common/faults/metrics.go` and call sites in each service. All are OTel SDK metrics, picked up by the existing telemetry pipeline.

| Metric | Type | Labels | Emitted by | Why |
|---|---|---|---|---|
| `griffin.catalog.product.requests_total` | Counter | `product_id`, `http_status_code` | `services/catalog/handlers.go:47` (per `GetProductHandler` call) | `catalog.error` outlier drilldown by `product_id` |
| `griffin.catalog.product.duration_ms` | Histogram | `product_id` | same | `catalog.slow` and outlier drilldown |
| `griffin.payment.charges_total` | Counter | `processor`, `status` | `services/payment/processor.go:103` (after `transactionDB.Set`) | `payment.fail` outlier drilldown by `processor` |
| `griffin.payment.charge.duration_ms` | Histogram | `processor` | `services/payment/processor.go:78` | `payment.gc-storm` p99 jitter visible per processor |
| `griffin.shipping.shipments_total` | Counter | `carrier`, `status` | `services/shipping/carriers.go:152` (after `CreateShipment` returns) | `shipping` outlier drilldown by `carrier` |
| `griffin.cart.operations_total` | Counter | `operation`, `http_status_code` | `services/cart/handlers.go:69, 87, 130` | `cart.error` and `cart.outlier` per‑operation outliers |
| `griffin.cart.operation.duration_ms` | Histogram | `operation` | same | `cart.outlier` p99 spike on a specific operation |

These are the *minimum* labels needed to make the demo tool calls below produce findings. We deliberately *avoid* adding `cart_id` or per‑request unique IDs — they'd blow cardinality past `maxTagCardinality=50` and the outlier tool would skip the tag.

### Detection contract per scenario

Every knob has a documented `detection` block under "Knob catalog" specifying:
1. The Conductor tool to call (`detect_anomalies` or `detect_outliers`).
2. The exact argument set.
3. The expected `IsAnomalous` / cohort output that proves the demo worked.

This contract drives both the demo flow (the slack/maestro queries we'll script) and Phase 2 acceptance tests (we replay the metric stream and run the tool offline against it).

### Scrape interval and lakerunner ingest

For window comparisons to be meaningful, the previous window must contain enough samples that p50/p99 are stable. The OTel SDK metric reader period defaults to 60 s in `metric.NewPeriodicReader`. Override via `OTEL_METRIC_EXPORT_INTERVAL=10s` for demo deployments so a 5‑minute window has 30 samples per series. Document this in the chaos overlay.

---

## Logs ↔ traces correlation

**Context.** Conductor's UI is gaining a span-detail side drawer with a logs tab that queries logs by `trace_id` (LogQL `{trace_id="..."}`). For that to return useful results, every Griffin log line emitted within a request must carry `trace_id` and `span_id`, and the lakerunner ingest must promote those to LogQL labels.

**Current state.** `common/telemetry.go:71` already wires `otelslog.NewHandler(serviceName)` into the slog Fanout, which is the right bridge. But the `otelslog` handler reads trace context from the slog `Record`'s `context.Context` — and **none of the 96 `slog.*` call sites in the codebase use the `*Context` variants** (`slog.InfoContext`, `slog.ErrorContext`, etc.). So OTLP log records leave with empty trace context, the OTel collector has nothing to forward, and lakerunner can't index by `trace_id`.

### What we need

1. **Migrate every slog call inside a request scope to its `*Context` variant**, passing the request's `r.Context()` (or the descendant ctx held inside the handler/business code). Outside request scope (startup, shutdown, ticker callbacks) the bare variants are fine — there's no span anyway.
2. **Wrap the stdout text handler** with a thin custom `slog.Handler` that, in `Handle(ctx, r Record)`, pulls `trace.SpanContextFromContext(ctx)` and, if valid, adds `trace_id`, `span_id` attributes before delegating to the underlying handler. This makes `kubectl logs` lines carry the same fields without depending on OTLP. Wire it as the first leg of the existing Fanout in `common/telemetry.go:71`.
3. **Verify OTLP log emission**: integration test starts the stack, drives a single request, fetches recent OTLP log records from the collector dev endpoint (or from a stub collector), asserts `TraceId` and `SpanId` are non-empty on each record whose `service.name` is one of ours.
4. **Lakerunner side** is out of repo scope, but document the assumption: lakerunner's OTel logs ingest must promote `TraceId` and `SpanId` from each OTLP `LogRecord` into indexed labels (`trace_id`, `span_id`) so `{trace_id="..."}` works as a LogQL stream selector. Flag this as a precondition for the demo.

### Audit checklist (Phase 1 prerequisite)

| File | slog calls to migrate (request-scoped) | Notes |
|---|---|---|
| `common/middleware.go` | `:35` (`LoggingMiddleware`'s `slog.Info("HTTP Request", …)`) | Uses `r.Context()` directly |
| `services/cart/handlers.go` | `:40, 73, 145, 165, 207, 228` (response/error logs in handlers) | Pass `r.Context()` |
| `services/cart/cart.go` | `:155` (cart ID generation fallback log) | Out-of-request, leave bare |
| `services/cart/client.go` | `:54` (close-body warn) | Pass `ctx` already in scope |
| `services/catalog/handlers.go` | `:34, 42, 59, 109, 147` | Pass `r.Context()` |
| `services/catalog/products.go` | startup loaders only | Leave bare |
| `services/payment/handlers.go` | `:33, 98, 126` | Pass `r.Context()` |
| `services/payment/processor.go` | `:51, 137` | `:51` is config close (bare); `:137` is gen-id fallback (bare). No request-scoped logs here today — **add one** at `:97` when knob fires (see knob #5 fidelity update below). |
| `services/shipping/handlers.go` | `:33, 63, 119, 125, 153` | Pass `r.Context()`. Note `:117` and friends use `log.Printf` from the standard `log` package (not slog) — convert to `slog.ErrorContext`. |
| `services/shipping/carriers.go` | `:117, 124, 152` | Convert all `log.Printf` to `slog.*Context` and accept a `ctx` parameter on `CreateShipment` so the carrier-selection / failure logs carry trace context. **Adds a context plumbing change** — handler must pass `r.Context()` into `CreateShipment`. |
| `services/images/main.go` | `:135, 166` (response write errors) | Pass `r.Context()`. `:48, 73, 75, 92, 101, 107, 118, 176, 192, 199, 206` are startup/shutdown — leave bare. |
| `services/recommendations/main.go` | `:55, 67, 155, 174, 185, 210, 233` (mix of request and ticker scope) | Request-scope: pass `r.Context()`. Ticker-scope (`:67, 155, 174`): pass the cancellable bg ctx so they still flow through otelslog (no span, but consistent handler path). |

This is mechanical — a single PR can do it without touching behavior.

### Trace-aware error logs at fault fire sites

Every knob whose `Kind=="error"` (catalog.error, cart.error, cart.poison-product, payment.fail, shipping.fail) **must emit a structured error log at the fire site** carrying:
- `trace_id` / `span_id` (automatic via the migration above),
- `griffin.fault=<key>` (matches the span attribute we already set),
- knob-specific context: `product_id`, `processor`, `carrier`, `cart_id`, etc.

This is what makes the trace → logs side-drawer useful: clicking a failed span surfaces the matching error log explaining the cause. Without it, the logs tab shows only the bland HTTP request log line.

### Span events as a parallel surface

In addition to logs, each fire site adds a span event (`span.AddEvent("griffin.fault.fired", trace.WithAttributes(...))`). Belt-and-suspenders: investigators using a pure trace UI without log correlation still see a "fault fired" event on the span timeline.

---

## Knob model

```go
// common/faults/knob.go
type Knob struct {
    Key         string    `json:"key"`         // catalog of allowed keys; see below
    Service     string    `json:"service"`     // catalog|cart|payment|images|recs|global|loadgen
    Kind        string    `json:"kind"`        // error|slow|outlier|memleak|cpuburn|gcstorm|flood
    Probability float64   `json:"probability"` // 0..1, default 1
    LatencyMs   int       `json:"latencyMs"`
    StatusCode  int       `json:"statusCode"`
    Target      string    `json:"target"`      // optional predicate (product id, etc.)
    StartedAt   time.Time `json:"startedAt"`   // server‑set on PUT
}
```

The catalog (`GET /admin/faults/catalog`) returns server‑declared `KnobDefinition` entries — `{key, service, kind, description, params: [{name, type, min, max, default}], guidance}`. The UI renders inputs from this; new knobs require **no frontend change**.

---

## Control plane API

```
GET    /admin/faults           -> { "active": Knob|null, "updatedAt": RFC3339 }
GET    /admin/faults/catalog   -> [ KnobDefinition, ... ]
GET    /admin/faults/events    -> SSE stream:
                                    event: activate  data: Knob
                                    event: clear     data: { previous: Knob, at: ts }
PUT    /admin/faults           body: Knob (StartedAt ignored) -> 200 with stored Knob
DELETE /admin/faults           -> 204
GET    /healthz                -> 200
```

CORS open via existing `common.CORSMiddleware`. All mutations no‑op (return 503) when `GRIFFIN_ADMIN_ENABLED != "true"`.

Event log is a 100‑entry ring buffer; `GET /admin/faults/events` first replays the buffer then streams new events.

### Operational constraints

- **Replicas = 1.** State is in memory; running multiple replicas would split the single‑knob invariant. K8s overlay sets `replicas: 1`, `strategy.type: Recreate`.
- **Restart semantics.** On control‑plane restart, services next poll returns null → all local copies clear → background goroutines stop. No corruption.

---

## Knob catalog (v1)

Each entry: trigger / mechanism / **fidelity notes** (what guarantees the signal is observable) / expected signals / parameter guidance / cleanup.

### 1. `catalog.error` — error spike on a target product

- **Service / kind**: `catalog` / `error`
- **Trigger**: `GET /api/products/{id}` where `id == Target` returns `StatusCode` (default 500).
- **Mechanism**: hook in `services/catalog/handlers.go:47` (`GetProductHandler`) before the `GetProduct(id)` call.
- **Fidelity notes**:
  - **Pick a hot product.** Locust calls `random.choice(self.products)` — over time every product is hit, but the cascade onto cart depends on add‑to‑cart traffic for *that* product. For demo determinism, default `Target=PROD-001` (dog food) and have the admin UI show the recommended target.
  - **Frontend has no retries** (`frontend/src/lib/api.ts` — every call is a single `fetch`). So a 500 surfaces immediately as a visible failure.
  - **Cart cascade is real**: cart calls catalog at `services/cart/client.go:38` with no retry; `AddItemToCart` returns the wrapped error → handler returns 500 → frontend "add to cart" button errors.
  - **Recs cascade is delayed**: `services/recommendations/main.go:60` refreshes every 5 minutes. Without intervention, the recs‑side error doesn't appear until the next refresh tick. **Fix**: shorten the recs ticker to **30 s** for demo builds (env‑gated `RECS_REFRESH_INTERVAL`, default 30 s), and add `POST /admin/recs/refresh` on recs to force a refresh — the control plane calls it on every knob transition.
- **Expected signals**: `http.server.duration{service=catalog,http.status_code=500}` spike for one route; `cart` 500 rate climbs in lockstep; recs logs `failed to refresh product cache`.
- **Params**: `Target` (string, required), `StatusCode` (int, 400–599, default 500).
- **Cleanup**: none — purely request‑time check.
- **Detection**:
  - **Primary** (`detect_outliers`): `metric_name="griffin.catalog.product.requests_total"`, `focus_tags=["product_id","http_status_code"]`, `direction="high"`, `start="now-10m"`. Expected: top cohort with `tags={product_id: "PROD-001", http_status_code: "500"}`, peak z‑score > 3, headline naming the product.
  - **Secondary** (`detect_anomalies`): `metric_name="griffin.catalog.product.requests_total"`, `filters={"http_status_code":"500"}`, `direction="high"`, `start="now-5m"`, `compare_window="previous"`. Expected: `IsAnomalous=true`, `count_ratio` ≫ 1.25 vs previous window's near‑zero baseline.
  - **Tertiary cascade** (`detect_anomalies`): `metric_name="griffin.cart.operations_total"`, `filters={"http_status_code":"500","operation":"add"}`, `direction="high"`. Confirms the cascade through cart.

### 2. `catalog.slow` — added latency on every catalog response

- **Service / kind**: `catalog` / `slow`
- **Trigger**: every catalog handler sleeps `LatencyMs` before responding.
- **Mechanism**: `faults.Middleware` mounted on the catalog router; service‑match gate ensures only catalog applies.
- **Fidelity notes**:
  - **Cart's HTTP client timeout is 5 s** (`services/cart/client.go:31`). If `LatencyMs >= ~4500`, cart converts slowness into a timeout error — that's a different fault. **Recommend `LatencyMs ≤ 3000`** in the catalog of allowed values; UI slider clamps to `[100, 4000]`.
  - **HTTP/1.1 connection pool saturation is a real, visible secondary effect.** Default Go transport has `MaxIdleConnsPerHost=2`. Under load with 3 s catalog responses, cart's outbound connections queue → cart latency > catalog latency. This is a *good* demo of head‑of‑line blocking; document it.
  - **Recs background refresh** also slows; the 5 s recs HTTP client timeout in `services/recommendations/main.go:46` covers up to ~4 s catalog latency.
- **Expected signals**: catalog server p50/p99 climb by `LatencyMs`; cart p99 climbs further than catalog due to pool saturation.
- **Params**: `LatencyMs` (int, 100–4000, default 2000).
- **Detection**:
  - **Primary** (`detect_anomalies`): `metric_name="http.server.duration"`, `filters={"service_name":"catalog-service"}`, `aggregation="p99"`, `direction="high"`, `start="now-5m"`, `compare_window="previous"`. Expected: `p99_ratio` ≈ `LatencyMs/baseline_p99`, well above 1.25.
  - **Secondary** (`detect_outliers`): `metric_name="http.server.duration"`, `focus_tags=["service_name","http_route"]`, `direction="high"`, `start="now-10m"`. Expected: catalog routes (`/api/products`, `/api/products/{id}`) appear as a cohort, plus the cart's outbound spans climb due to pool saturation — useful "which service is the root" demo.

### 3. `cart.error` — probabilistic cart errors

- **Service / kind**: `cart` / `error`
- **Trigger**: each `GET /api/cart/{id}` and `POST /api/cart/{id}/add` rolls the dice; on hit, returns `StatusCode`.
- **Mechanism**: hook in `services/cart/handlers.go:69, 87`, before existing logic.
- **Fidelity notes**: trivial. The cart error is terminal — no upstream retries — so a 20% probability shows up as a 20% error rate cleanly.
- **Expected signals**: `http.server.duration{service=cart,http.status_code=5xx}` rate matches `Probability`.
- **Params**: `Probability` (float, 0–1, default 0.5), `StatusCode` (int, default 500).
- **Detection**:
  - **Primary** (`detect_anomalies`): `metric_name="griffin.cart.operations_total"`, `filters={"http_status_code":"500"}`, `direction="high"`, `compare_window="previous"`. Expected `count_ratio` ≈ `Probability / baseline_error_rate`.
  - **Secondary** (`detect_outliers`): `metric_name="griffin.cart.operations_total"`, `focus_tags=["service_name","http_status_code"]`. Demonstrates that "cart-service / 500" is the outlier cohort vs healthy peers.

### 4. `cart.outlier` — long‑tail latency on a small fraction

- **Service / kind**: `cart` / `outlier`
- **Trigger**: each cart request rolls the dice; on hit, sleeps `LatencyMs` (default **30 000**) before continuing.
- **Mechanism**: hook in `services/cart/handlers.go:69, 87`.
- **Fidelity notes**:
  - **Vite proxy default has no proxy timeout configured**, but `http-proxy` underneath defaults to undefined → no truncation. **Explicitly set `proxyTimeout: 60000` in `vite.config.ts`** for the cart route so 30 s outliers complete end‑to‑end.
  - **Browser fetch has no client‑side timeout** by default; UI will wait, which is the demo point.
  - **Trace sampling**: `common/telemetry.go` uses the SDK default sampler (parent‑based, always‑on root). 100% sampling on demo scale ensures every outlier is captured. Document this — if anyone ever adds head‑sampling, outlier capture breaks.
  - **Probability matters**: at 1 % and locust 10 RPS you get ~1 outlier per 10 s. Default to **0.05** (5 %) so the long tail is visible inside a 30 s observation window without dominating average latency.
- **Expected signals**: `http.server.duration{service=cart}` p99/p99.9 jumps; mean barely moves (the textbook outlier story).
- **Params**: `Probability` (float, 0.001–0.2, default 0.05), `LatencyMs` (int, 5000–60000, default 30000).
- **Detection**:
  - **Primary** (`detect_anomalies`): `metric_name="griffin.cart.operation.duration_ms"`, `aggregation="p99"`, `direction="high"`, `compare_window="previous"`. Expected `p99_ratio` ≫ 1.25 while `p50_ratio` ≈ 1.0 — this is the canonical "long tail without average shift" story. Note that this requires the **half‑resolution branch** of `compareWindow` to catch it cleanly when probability is low (5% → ~3 outliers per minute at 10 RPS); the half window narrows in on the busier portion.
  - **Why not `detect_outliers` here**: the outlier dimension is *temporal*, not *cohort*. There is no peer service / route / product that's an outlier — every cart op has the same chance. `detect_anomalies` is the right tool. Document this as the "outlier vs anomaly" teaching moment in the demo script.

### 5. `payment.fail` — failure‑rate spike on charges

- **Service / kind**: `payment` / `error`
- **Trigger**: payment processor's runtime failure rate is overridden to `Probability`.
- **Mechanism**: in `services/payment/processor.go:90` (`ProcessPayment`), replace `processor.FailureRate` lookup with `effectiveFailureRate(processor)` which returns `knob.Probability` when active, otherwise the YAML default.
- **Fidelity notes**:
  - **Already wired.** The 402 status path (`services/payment/handlers.go:94`) and `transaction.Status="failed"` flow exist — we just override the input to `shouldFail()`.
  - **Don't mutate the `processors` map.** The map is read‑only after `LoadProcessorConfig`; computing the effective rate at call time avoids reset semantics on knob clear.
  - **Random source** in `processor.go:40` is unbuffered `mrand.New(mrand.NewSource(...))` — fine for low throughput; if we ever crank load, swap to `mrand/v2` per‑goroutine source to avoid mutex contention masking the fault.
- **Expected signals**: `http.server.duration{service=payment,http.status_code=402}` jumps to ~`Probability`; `payment.charge.failed` counter (new) climbs.
- **Params**: `Probability` (float, 0–1, default 0.8). No reset needed on clear.
- **Detection**:
  - **Primary** (`detect_outliers`): `metric_name="griffin.payment.charges_total"`, `focus_tags=["processor","status"]`, `direction="high"`, `start="now-10m"`. With 3 processors (`puppypay`, `kittycard`, `doggiecoin`) and the knob biasing one of them (the random pick that ProcessPayment lands on), the failing processor stands out as a peer‑deviation cohort. **Demo nuance**: `selectRandomProcessor` picks at random, so the fault hits *every* processor proportionally — not one specific peer. To produce a clean per‑processor outlier we extend the knob with a `Target` field naming a processor; the override only applies when `processor.Name == knob.Target`. Update `Params` accordingly: `Target` (string, optional, e.g. "kittycard").
  - **Secondary** (`detect_anomalies`): `metric_name="griffin.payment.charges_total"`, `filters={"status":"failed"}`, `direction="high"`, `compare_window="previous"`. Expected `count_ratio` matches the fault rate's lift vs the YAML baseline (kittycard 20% baseline → 80% knob = 4×).

### 6. `payment.gc-storm` — STW pause jitter on payment

- **Service / kind**: `payment` / `gcstorm`
- **Trigger**: while active, payment service runs (a) a heap‑churn goroutine and (b) a `runtime.GC()` loop.
- **Mechanism**: knob transition spawns two goroutines; clear stops them via context cancel.
- **Fidelity notes — critical**:
  - **`runtime.GC()` alone produces no visible pause** on a small idle heap. The collector has nothing to do. We must generate **real garbage** for the GC to chew on.
  - **Churn design**: a goroutine maintains a sliding window of 20 × 5 MiB byte slices (~100 MiB working set). Every 50 ms it allocates a fresh `make([]byte, 5<<20)`, writes a few bytes (so the OS actually maps the pages), and overwrites the oldest slot. This produces a steady ~100 MiB/s allocation rate.
  - **GC trigger**: a second goroutine calls `runtime.GC()` every `IntervalMs` (default **200 ms**). With churn running, each call is a real STW pause of multiple ms.
  - **Visibility**: STW pauses surface in `process.runtime.go.gc.pause_ns` (already exported via `iruntime.Start` in `common/telemetry.go:87`) and as p99 jitter on `payment.charge` because handlers occasionally land inside a pause.
  - **Cleanup**: on clear, cancel both goroutines and `nil` the slice window. Memory is reclaimed within one or two natural GC cycles. Document: "RSS may take ~10 s to drop after clear."
  - **Don't mutate `GOGC`** — too invasive, lingers across knob clears.
- **Expected signals**: `runtime.go.gc.pause_ns` p99 climbs from sub‑ms to multi‑ms; `payment.charge` p99 jitters with the same period; heap_alloc oscillates ~100 MiB.
- **Params**: `IntervalMs` (int, 50–2000, default 200). Encoded in `LatencyMs`.
- **Detection**:
  - **Primary** (`detect_anomalies`): `query="histogram_quantile(0.99, sum by (le, service_name)(rate(process_runtime_go_gc_pause_ns_bucket[1m])))"` (full PromQL mode, since this is a derived expression), `direction="high"`, `compare_window="previous"`. Expected `p99_ratio` > 5×.
  - **Secondary** (`detect_outliers`): `metric_name="process.runtime.go.gc.pause_ns"`, `focus_tags=["service_name"]`, `direction="high"`. Expected: payment‑service stands out as the outlier service vs other Griffin services that share the same Go runtime baseline. **Requires** ≥3 services in the dataset for z‑score to engage — Griffin has 6, so we're fine.
  - **Tertiary** (`detect_anomalies`): `metric_name="griffin.payment.charge.duration_ms"`, `aggregation="p99"`, `direction="high"`. The user‑facing impact: charge p99 climbs because requests occasionally land in a STW pause.

### 7. `images.slow` — visible browser slowness

- **Service / kind**: `images` / `slow`
- **Trigger**: `/static/*` and `/api/images/product/{id}` sleep `LatencyMs`.
- **Mechanism**: wrap both handlers in `services/images/main.go:68, 80` with the slow middleware.
- **Fidelity notes**:
  - **Browser cache will mask this.** Static images are cached aggressively. Two mitigations (apply both):
    1. The `images` service already computes a per‑file hash (`imageHashes` in `services/images/main.go:38`). The frontend can use `?h={hash}` query for static URLs (`ProductImage.svelte` already fetches `/api/images/product/{id}` and gets the hash). Verify this round‑trips so cache‑busting works when image content doesn't change.
    2. While the knob is active, the slow middleware sets `Cache-Control: no-store` so reloads always hit the slow path.
  - **Browser concurrent request limit** (~6 per origin) means many slow images compound — that's fine, it's the visible demo.
- **Expected signals**: image span p99 climbs; frontend "Time to Interactive" climbs; cart/checkout unaffected (separates "asset latency" from "API latency" in the trace view).
- **Params**: `LatencyMs` (int, 200–5000, default 1500).
- **Detection**:
  - **Primary** (`detect_outliers`): `metric_name="http.server.duration"`, `focus_tags=["service_name"]`, `direction="high"`. Expected: `image-service` is the outlier service while other Griffin services hold steady — clean cross‑service peer comparison.
  - **Secondary** (`detect_anomalies`): `metric_name="http.server.duration"`, `filters={"service_name":"image-service"}`, `aggregation="p99"`, `direction="high"`, `compare_window="previous"`.

### 8. `recs.memleak` — unbounded heap growth

- **Service / kind**: `recs` / `memleak`
- **Trigger**: while active, a goroutine appends padded copies of `productCache` to a never‑freed slice every 100 ms.
- **Mechanism**: knob transition starts goroutine; clear stops it and (for demo iteration UX) sets the slice to nil.
- **Fidelity notes — critical**:
  - **The current `productCache` is too small to leak visibly.** 6 products × ~200 B = ~1 KiB. Even appending 10 000 copies = 10 MiB, easy for GC to ignore.
  - **Padding strategy**: each "leaked" snapshot is `[]common.Product` plus a per‑snapshot `[]byte` blob of **1 MiB** of random data, retained in a package‑level `var leaked []leakedSnapshot`. At 100 ms cadence → ~10 MiB/s growth. After 60 s → ~600 MiB. RSS climbs visibly; `runtime.go.mem.heap_alloc` and `process.resident_memory_bytes` (host metrics) both move.
  - **Why a goroutine, not the existing 5‑min refresh path?** 5 minutes is too slow for a demo. The leak goroutine generates pressure independent of refresh cadence.
  - **GC pressure secondary signal**: as heap grows, GC frequency increases (live set rising), so `runtime.go.gc.pause_ns` also climbs over time — a richer signal than a single metric.
  - **Cleanup**: `leaked = nil; runtime.GC()` on knob clear so demos can iterate. Document: "Memleak knob is reversible by design for demo purposes; a real leak wouldn't free."
- **Expected signals**: `process.resident_memory_bytes{service=recs}` linear ramp; `runtime.go.mem.heap_alloc{service=recs}` ramp with sawtooth from GC; eventual GC pause growth.
- **Params**: none. (Or `LatencyMs` reused as growth interval; default 100.)
- **Detection**:
  - **Primary** (`detect_outliers`): `metric_name="process.resident_memory_bytes"`, `focus_tags=["service_name"]`, `direction="high"`. Expected: `recommendations-service` stands out as the outlier service. The peer comparison is unambiguous because the other 5 Griffin services share a near‑identical baseline RSS.
  - **Secondary** (`detect_anomalies`): `metric_name="process.resident_memory_bytes"`, `filters={"service_name":"recommendations-service"}`, `direction="high"`, `compare_window="previous"`, `start="now-2m"`. Best when called ≥1 minute into the leak so ramp is visible.
  - **Tertiary** (`detect_anomalies`): `metric_name="process.runtime.go.gc.pause_ns"`, `filters={"service_name":"recommendations-service"}`, `aggregation="p99"`, `direction="high"`. Shows the secondary effect: heap growth → GC pressure rises.

### 9. `global.cpu-burn-traffic` — per‑request CPU burn (latency cascade)

- **Service / kind**: `global` / `cpuburn`
- **Trigger**: every inbound HTTP request runs a tight CPU loop for `LatencyMs` (default 50) before serving.
- **Mechanism**: `faults.Middleware` global branch.
- **Fidelity notes**:
  - **Spin loop must do un‑optimizable work.** `for { _ = math.Sqrt(rand.Float64()) }` driven by a deadline: `for time.Now().Before(deadline) { _ = math.Sqrt(rand.Float64()) }`. Compiler can't elide it.
  - **Per‑request burn cascades latency** but only saturates CPU under load. Pair with locust traffic. Recommend default `LatencyMs=50` so a single request is barely visible but 50 RPS at 50 ms = 2.5 cores busy.
- **Expected signals**: `process.cpu.utilization` rises proportional to RPS; latency rises by `LatencyMs` baseline plus contention overhead.
- **Params**: `LatencyMs` (int, 5–500, default 50).
- **Detection**:
  - **Primary** (`detect_anomalies`): `metric_name="http.server.duration"`, `aggregation="p99"`, `direction="high"`, `compare_window="previous"`. Expected `p99_ratio` ≈ `(baseline + LatencyMs) / baseline`.
  - **Secondary** (`detect_anomalies`): `metric_name="process.cpu.utilization"`, `direction="high"`, `compare_window="previous"`.
  - **Why no `detect_outliers`**: the global knob saturates **every** service equally, so no single peer stands out. This is intentional — a "global congestion" demo where outlier detection correctly returns nothing and anomaly detection catches it.

### 10. `global.cpu-burn-bg` — background CPU saturation (independent of traffic)

- **Service / kind**: `global` / `cpuburn`
- **Trigger**: while active, every service spawns `runtime.NumCPU()` background goroutines running an un‑optimizable spin loop.
- **Mechanism**: knob transition starts goroutines under context; clear cancels.
- **Fidelity notes**:
  - **Uses `LatencyMs` field as a stand‑in for per‑goroutine duty cycle.** With `LatencyMs=0` (default), goroutines spin 100 % — CPU pegged. With `LatencyMs=100` they spin 100 ms then sleep 100 ms — 50 % duty cycle. Lets users choose between "soft pressure" and "smoking ruin."
  - **Why per‑service?** Want the demo to show that one knob saturates *all* services equally — useful for "noisy neighbor / shared host" stories. If we wanted to saturate only one, we'd add a `Target=catalog` field. Out of v1 scope.
- **Expected signals**: `process.cpu.utilization` flat near 1.0; latency rises across every service due to scheduler contention; gc pause grows because GC threads can't get CPU.
- **Params**: `LatencyMs` (int, 0–500, default 0 = full burn).
- **Detection**:
  - **Primary** (`detect_anomalies`): `metric_name="process.cpu.utilization"`, `direction="high"`, `compare_window="previous"`.
  - **Secondary** (`detect_outliers`): `metric_name="http.server.duration"`, `focus_tags=["service_name"]`, `direction="high"`. With full burn this is degenerate (every service is equally bad → no outlier); with partial duty cycle (`LatencyMs > 0`) the demo also caps `Target=catalog` (out of v1 scope) to produce a single‑service outlier — document as future work.

### 11. `shipping.fail` — failure spike on a target carrier

- **Service / kind**: `shipping` / `error`
- **Trigger**: shipping carrier's runtime failure rate is overridden to `Probability` for the carrier matching `Target`.
- **Mechanism**: in `services/shipping/carriers.go:123`, replace `carrier.FailureRate` lookup with `effectiveFailureRate(fc, carrierID, carrier)`.
- **Fidelity notes**: 3 carriers (`ponyexpress`, `avianair`, `catcarrier`) — perfect peer set for outlier drilldown. Without `Target`, the override applies to all carriers and we lose the cohort signal; require `Target`.
- **Expected signals**: shipment `status="failed"` rate climbs for one carrier label, baseline for others.
- **Params**: `Target` (string, required, one of carrier IDs), `Probability` (float, 0–1, default 0.8).
- **Detection**:
  - **Primary** (`detect_outliers`): `metric_name="griffin.shipping.shipments_total"`, `focus_tags=["carrier","status"]`, `direction="high"`. Expected cohort: `{carrier: <Target>, status: "failed"}` with peer comparison vs the other two carriers.
  - **Secondary** (`detect_anomalies`): same metric, `filters={"carrier":<Target>, "status":"failed"}`, `direction="high"`, `compare_window="previous"`.

### 12. `cart.poison-product` — opaque 500 with the cause only in logs

This knob exists specifically to demonstrate the trace → side-drawer → logs UX. The trace alone shows a 500; the response body is intentionally vague; **only the structured error log (filtered by `trace_id`) reveals what actually happened**.

- **Service / kind**: `cart` / `error`
- **Trigger**: any `GET /api/cart/{id}` or `POST /api/cart/{id}/add` whose cart contains the product `Target` returns HTTP 500 with body `{"error":{"code":"INTERNAL_ERROR","message":"The service encountered an unexpected error"}}` — a deliberately uninformative response.
- **Mechanism**: hook in `services/cart/handlers.go:69, 87`. Inside the cart-fetch path, after `GetCart` returns, scan `cart.Items` for `Target`. On match:
  ```go
  span.AddEvent("griffin.fault.fired", trace.WithAttributes(
      attribute.String("griffin.fault", "cart.poison-product"),
      attribute.String("product_id", target),
      attribute.String("cart_id", cartID),
  ))
  slog.ErrorContext(r.Context(), "cart contains tainted item: poison product detected",
      "griffin.fault", "cart.poison-product",
      "product_id", target,
      "cart_id", cartID,
      "tenant", "demo",
      "cause", "data integrity check failed: corrupted item record")
  // generic 500
  ```
- **Fidelity notes**:
  - **Causal narrative in logs only.** The response carries no clue — that's the point. The UX story is: "frontend says generic error → user clicks the failed span → side drawer opens → logs tab shows the actual cause."
  - **Cart needs to actually contain the product** for the knob to fire, so the demo flow has the operator add `Target` to their cart first (the admin UI can include a "Add poison product to my cart" helper button next to this knob).
  - **One log line per failure, not many.** Verbose info logs would dilute the punchline. Keep the cart's other logging at info; the error log is the single high-signal line.
  - **Stable target.** Default `Target=PROD-003` (dog bed). Document.
- **Expected signals**:
  - Trace: cart span ends with `status_code=ERROR`, `http.status_code=500`, `griffin.fault=cart.poison-product` attribute, plus the `griffin.fault.fired` span event with `product_id`/`cart_id`.
  - Logs (filtered by trace_id): exactly one error-level log with the "tainted item" message and the structured attributes.
  - Metrics: `griffin.cart.operations_total{operation, http_status_code="500"}` ticks for affected ops.
- **Params**: `Target` (string, required, default `PROD-003`).
- **Detection**:
  - **Primary** (UX, not a tool): operator clicks the failed span in the trace UI → side drawer's logs tab queries `{trace_id="<id>"}` → renders the error log. **This is the demo punchline** for the new logs side-drawer feature.
  - **Secondary** (`detect_anomalies`, log mode): `log_selector='{service_name="cart-service", level="error"}'`, `log_metric="count_over_time"`, `direction="high"`, `compare_window="previous"`. Should flag a sharp rise in cart error logs vs the previous window. Not the headline detection — the log-correlation UX is.
  - **Tertiary** (`detect_outliers`): `metric_name="griffin.cart.operations_total"`, `focus_tags=["http_status_code","operation"]`. Confirms which cart operation is most affected (typically `add` or `get`).

### 13. `loadgen.flood` — traffic spike (Phase 4)

- **Service / kind**: `loadgen` / `flood`
- **Trigger**: locust scales up users when knob active; reverts on clear.
- **Mechanism**: a small Python poll loop in `loadgen/locustfile.py` calls control plane every 2 s; on `flood` active, switches to `FloodShape` (ramp 10 → 500 users over 30 s). On clear, returns to baseline shape.
- **Fidelity notes**:
  - **Locust environment variables only apply at startup** — we cannot use them for runtime change. Polling + `Environment.runner.start(user_count, spawn_rate)` is the documented runtime control path.
  - **Frontend is the bottleneck first** at high RPS (single Vite/Node process). Document expected progression: frontend saturates → backend services see latency rise → eventual error budget burn.
- **Expected signals**: RPS gauge climbs; latency p99 across all services climbs together; eventual 5xx burn.
- **Params**: `Probability` reused as target user count fraction (0–1 mapping to 0–500 users); `LatencyMs` reused as ramp duration ms.
- **Detection**:
  - **Primary** (`detect_anomalies`): `metric_name="http.server.duration"` count (i.e. RPS), `direction="high"`, `compare_window="previous"`, `detect_sparse=true`. Expected: large positive `count_ratio`.
  - **Secondary** (`detect_anomalies`): same metric `aggregation="p99"`, `direction="high"`. Confirms saturation latency rise alongside RPS.
  - **No outlier expected** at first — load is broad. As one service saturates first (frontend), `detect_outliers` on `http.server.duration` with `focus_tags=["service_name"]` may then flag the saturated service as the outlier; useful "watch the bottleneck propagate" demo step.

---

## Phased delivery

Five PRs, each independently shippable.

### Phase 0 — Logs↔traces correlation (prerequisite)

This must land **before** Phase 2 so every fault-injected error log carries `trace_id` from the moment knobs come online.

**New files**
- `common/logging/trace_handler.go` — custom `slog.Handler` wrapping the underlying text handler. In `Handle(ctx, r)`, extracts `trace.SpanContextFromContext(ctx)`; if valid, calls `r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))` before delegating. Lives in a new `common/logging` package to keep `common/telemetry.go` tidy.

**Edits**
- `common/telemetry.go:71` — wrap the `slog.NewTextHandler(...)` leg of the `Fanout` with the new `trace_handler`. The `otelslog.NewHandler` leg already attaches trace context to OTLP records once call sites pass ctx.
- Mechanical sweep across all 96 `slog.*` call sites per the audit checklist above. Request-scoped → `*Context` variant with `r.Context()`. Out-of-request → leave bare.
- `services/shipping/carriers.go` — convert all `log.Printf` to `slog.*Context`; thread `ctx` through `CreateShipment(ctx, orderID, carrierID)` and update `services/shipping/handlers.go:99` to pass `r.Context()`.

**Acceptance**
- Integration test (extends `integration/cart_catalog_test.go`): drives one cart-add request, captures stdout from cart and catalog, asserts every log line emitted between request entry and response carries non-empty `trace_id` and `span_id` matching the request's outgoing trace header.
- Same test asserts the OTLP log exporter sees records with non-empty `TraceId`/`SpanId` (use a stub OTLP collector; record protobuf and inspect).
- No behavioral changes to handlers — purely log shape.

### Phase 1 — Control plane + registry primitives

**New files**
- `cmd/controlplane.go` — subcommand wiring (mirror of `cmd/catalog.go`)
- `services/controlplane/main.go` — `Start()`, server on `:8086`, `active *Knob` behind `sync.Mutex`, 100‑entry ring‑buffer event log, SSE handler
- `services/controlplane/handlers.go` — REST endpoints
- `services/controlplane/catalog.go` — slice of `KnobDefinition` for `GET /admin/faults/catalog`
- `common/faults/knob.go` — `Knob` and `KnobDefinition` types
- `common/faults/client.go` — polling client; `atomic.Pointer[Knob]`; `Active() *Knob`; transition callbacks (`OnActivate`, `OnClear`) so services can spawn/stop background goroutines
- `common/faults/middleware.go` — generic middleware (per‑request slow / error / cpu‑burn‑traffic)
- `common/faults/metrics.go` — registers `griffin.faults.injected` counter, `griffin.faults.added_latency_ms` histogram; `Record(ctx, knob, kind)` helper sets `griffin.fault=<key>` span attribute

**Edits**
- `docker-compose.yml` — add `controlplane` service on `:8086`, add `CONTROLPLANE_URL=http://controlplane:8086` env to every existing service
- `Makefile` — add `controlplane` to build target list if not auto‑picked up

**Acceptance**
- `griffin controlplane` starts; `curl localhost:8086/admin/faults` returns `{"active":null,...}`
- PUT then GET round‑trips a knob; DELETE clears
- SSE stream emits `activate` and `clear` events
- No other service consumes faults yet (zero behavior change in existing services)

### Phase 2 — Service hook points

Each service's `Start()` adds:

```go
fc := faults.NewClient(faults.ClientOpts{
    URL:     os.Getenv("CONTROLPLANE_URL"),
    Service: "<service-name>",
    OnActivate: handleKnobActivate,
    OnClear:    handleKnobClear,
})
fc.Start(ctx)
r.Use(faults.Middleware(fc, "<service-name>")) // generic slow/error/cpuburn-traffic
```

**Service‑specific changes**:

| File:line | Change |
|---|---|
| `services/catalog/handlers.go:47` (`GetProductHandler`) | At entry: if `active.Key=="catalog.error" && active.Target==id`, write `StatusCode` and return. Always emit `griffin.catalog.product.requests_total{product_id, http_status_code}` and `griffin.catalog.product.duration_ms{product_id}` so outlier drilldown by `product_id` works. |
| `services/cart/handlers.go:69, 87, 130` | At entry: `faults.MaybeFail(fc, "cart.error")` and `faults.MaybeOutlier(fc, "cart.outlier")`. After response: emit `griffin.cart.operations_total{operation, http_status_code}` and `griffin.cart.operation.duration_ms{operation}` with `operation` ∈ {`get`,`add`,`remove`,`checkout`}. |
| `services/payment/processor.go:90` | Replace `processor.FailureRate` with `effectiveFailureRate(fc, processor)` (respects `Target`). Emit `griffin.payment.charges_total{processor, status}` and `griffin.payment.charge.duration_ms{processor}`. |
| `services/payment/processor.go` (new) | `gcStorm` controller — context + 2 goroutines (churn + GC trigger); started/stopped from `OnActivate`/`OnClear`. |
| `services/shipping/carriers.go:123` | Replace `carrier.FailureRate` with `effectiveFailureRate(fc, carrierID, carrier)` (respects `Target`). Emit `griffin.shipping.shipments_total{carrier, status}` after `CreateShipment` returns. |
| `services/images/main.go:68, 80` | Wrap both handlers (API + static fileserver) in `faults.SlowMiddleware(fc, "images.slow")` (separate from generic middleware so static files are covered). When knob active, set `Cache-Control: no-store` on the response. |
| `services/recommendations/main.go:60` | Make refresh interval env‑configurable (`RECS_REFRESH_INTERVAL`, default `30s`); wire `POST /admin/recs/refresh` admin route used by the control plane on every transition involving `catalog.error`. |
| `services/recommendations/main.go` (new) | `memLeak` controller goroutine; activated via `OnActivate("recs.memleak")`; appends `leakedSnapshot{products, blob: 1MiB random}` every 100 ms; clear nils slice + `runtime.GC()`. |
| `common/faults/metrics.go` (new) | Define and register the 7 custom Counter/Histogram instruments listed in "Custom metrics required for outlier drilldown" so each service has access to the same instrument handles. |
| All fault fire sites (above) | Whenever a knob causes a request to fail or be slowed, emit a `slog.ErrorContext(r.Context(), …, "griffin.fault", knob.Key, …site-specific context…)` and `span.AddEvent("griffin.fault.fired", …)`. This is what makes the trace → side-drawer → logs UX surface a usable cause. |

**Custom metric emission contract** — every site emits unconditionally (whether or not a knob is active) so the previous window has real data for `detect_anomalies` to compare against. The fault is detectable as a *change* against that baseline.

**Trace-correlated log contract** — every error/slow knob's hook site emits one structured error log carrying `trace_id`, `span_id`, `griffin.fault`, and the site's identifying labels (`product_id` / `processor` / `carrier` / `cart_id` / route / etc.). The poison-product knob (#12) is the canonical demo of this; the others get it by symmetry so the logs side-drawer is useful for any failed span, not just one knob.

**Acceptance**

For each of the 12 v1 knobs, in turn, against a running stack with at least 5 minutes of warm‑up traffic:
- PUT knob → symptom appears in metrics within 2 s; `griffin.fault=<key>` span attribute present.
- After 60 s of fault traffic: invoke the **primary detection** documented for the knob (offline replay against the metrics endpoint is fine for tests; live mcp-gateway call for the demo). Verify `IsAnomalous=true` or a non‑empty `Cohorts[]` whose `Tags` match the documented expected output.
- DELETE knob → symptom disappears within 2 s (memleak: RSS reclaim within ~10 s).
- `griffin.faults.injected{key,kind,service}` counter increments while active and stops while cleared.

### Phase 3 — Frontend admin UI

**New files**
- `frontend/src/lib/faults.ts` — typed REST + SSE client
- `frontend/src/routes/admin/faults/+page.svelte` — main admin page
- `frontend/src/lib/components/ActiveFaultBanner.svelte` — persistent banner

**Edits**
- `frontend/vite.config.ts` — add proxy for `/admin/faults` → `http://localhost:8086`, **set `proxyTimeout: 60000`** on that entry **and on the `/api/cart` entry** (for the 30 s outlier)
- `frontend/vite.config.prod.ts` — same
- `frontend/src/routes/+layout.svelte` (or App root) — mount `<ActiveFaultBanner />`

**Page behavior**
- Top: active card. If active, shows `Key`, `Service`, `Kind`, `StartedAt`, current parameters, and a **Clear** button. If null, shows "No fault active."
- Body: a card per `KnobDefinition` from `GET /admin/faults/catalog`. Each card renders param inputs (sliders for ranges, text for `Target`) and an **Activate** button. Clicking Activate sends `PUT /admin/faults` — control plane atomically replaces.
- Side panel: scrolling event log fed by SSE.
- Page is gated by `import.meta.env.VITE_ENABLE_FAULT_UI === 'true'`.

**Banner behavior**
- Mounted once in root layout. Subscribes to SSE; falls back to 2 s polling if SSE drops.
- Visible on every page (catalog, cart, checkout) so the user can always see what fault is in effect.

**Acceptance**
- From admin page: activate `catalog.error` with `Target=PROD-001`, observe in another browser tab that adding PROD‑001 to cart fails while other products work; click Clear, observe recovery.
- Banner appears on the homepage within 2 s of activation.

### Phase 4 — Loadgen + k8s overlay + docs

- `loadgen/locustfile.py` — add `FloodShape`; add control‑plane poll loop (separate thread, 2 s); on `loadgen.flood` active, call `environment.runner.start(target_users, spawn_rate)`; on clear, return to baseline.
- Catalog entry for `loadgen.flood` added to control plane.
- `k8s/overlays/chaos/` — kustomize overlay:
  - `controlplane.yaml` Deployment (`replicas: 1`, `strategy: Recreate`) + Service
  - patches setting `GRIFFIN_ADMIN_ENABLED=true`, `CONTROLPLANE_URL=http://controlplane:8086`, `VITE_ENABLE_FAULT_UI=true` on relevant workloads
  - kustomization listing the patches
- `README.md` — short "Chaos demo" section: how to bring up the chaos overlay, link to `/admin/faults`, table of knobs with expected signals.
- `docs/demo/conductor-detection.md` — for each of the 12 knobs, a copy‑pasteable Maestro/mcp-gateway tool call (the "primary detection" from the knob's Detection block) plus the expected output shape. This is the script anyone giving the demo follows.
- `k8s/overlays/chaos/` env: set `OTEL_METRIC_EXPORT_INTERVAL=10s` on every workload so window comparisons have ≥30 samples per 5‑minute window.

**Acceptance**
- `kubectl apply -k k8s/overlays/chaos` brings up a fully chaos‑capable stack pointing at the same lakerunner that mcp-gateway queries.
- `loadgen.flood` ramps then reverts cleanly when toggled.
- For each of the 12 knobs, running its primary detection tool call after 60 s of fault traffic returns the expected `IsAnomalous=true` / cohort match documented under that knob.

---

## Demo flow

Each scenario follows the same shape, calibrated so Conductor's tools have enough data to fire:

1. **Warm‑up baseline (≥ 5 min)** — Locust at the default 10 users keeps every metric series populated. Without baseline, `detect_anomalies` has nothing to compare to and `detect_outliers` has too few peer samples for z‑score.
2. **Activate the knob** via `/admin/faults` UI (or `PUT /admin/faults`).
3. **Observation window (60–120 s)** — fault traffic accumulates. Active‑fault banner makes it obvious.
4. **Invoke detection** — call the primary tool documented under the knob via Maestro / mcp-gateway. The tool's output is the demo punchline: it names the bad product / processor / carrier / service in plain language.
5. **Clear knob** — `DELETE /admin/faults`. Confirm with the same tool call after a ~30 s settle that `IsAnomalous=false` (compared against the now‑recovered baseline).

A single demo session typically chains 4 knobs, one per detection style:
- An outlier story (`catalog.error` → "PROD‑001 is the bad product" via `detect_outliers`),
- An anomaly story (`payment.gc-storm` → "payment p99 is 5× baseline" via `detect_anomalies`),
- A cascade story (`catalog.slow` → propagation visible across cart, recs, frontend traces),
- A **logs-explain-the-failure** story (`cart.poison-product` → trace shows generic 500 → side drawer logs tab reveals "cart contains tainted item: product_id=PROD‑003").

Phase 4 docs include shell snippets that drive this flow end‑to‑end so it's reproducible without manual mcp-gateway calls.

---

## Cross‑cutting details

### Polling jitter

`faults.Client` uses `1000ms + rand.IntN(400)-200` between polls so 6 services don't synchronize their requests against the control plane.

### Atomic state swap

`atomic.Pointer[Knob]` for read‑side; `Active() *Knob` is lock‑free. Transition callbacks (start/stop background goroutines) run inside a per‑client `sync.Mutex` so we don't double‑spawn or leak goroutines on rapid PUT/PUT/DELETE sequences.

### Clean transitions

`faults.Client` diffs old vs new on each poll. Cases:

| old | new | action |
|---|---|---|
| nil | nil | nothing |
| nil | K | swap pointer; call `OnActivate(K)` |
| K | nil | swap pointer; call `OnClear(K)` |
| K1 | K2 (same key) | swap pointer; call `OnReconfigure(K2)` if knob has a reconfigure handler, else no‑op |
| K1 | K2 (different key) | call `OnClear(K1)`; swap; call `OnActivate(K2)` |

### OTel emission

- New counter `griffin.faults.injected{key, kind, service}` — incremented every time a knob fires (per request for traffic knobs, once per tick for background knobs).
- New histogram `griffin.faults.added_latency_ms{key}` — recorded by every slow/outlier/cpu‑burn fire site.
- Span attribute `griffin.fault=<key>` on the active span at fire time. Lets a Tempo/Jaeger query distinguish injected vs natural failures with one filter.
- Span event `griffin.fault.fired` on the active span at fire time, carrying the same site-specific attributes as the corresponding error log. Visible directly in trace UIs without log correlation.
- Existing runtime + host metrics cover GC pause, heap alloc, RSS, CPU. No additions needed for those.

### Logs ↔ traces correlation

- Every request-scoped log line carries `trace_id` and `span_id` after Phase 0 (custom slog handler on stdout + `otelslog` for OTLP). LogQL `{trace_id="..."}` returns all logs for one trace.
- Logs emitted outside a request (startup, shutdown, ticker callbacks) have no `trace_id`/`span_id` — by design.
- Lakerunner ingest must promote `TraceId` and `SpanId` from OTLP `LogRecord` to indexed labels for `{trace_id="..."}` to work as a stream selector. Out-of-repo, but documented as a precondition.
- **Volume budget**: target ≤ 5 log lines per trace under normal flow (request entry + handler + response), plus one error line if a fault fired. The poison-product knob's punchline depends on signal-to-noise — keep info logs tight.

### Browser cache

While `images.slow` is active, the slow middleware sets `Cache-Control: no-store` on every image response so reloads hit the slow path. When inactive, normal cache headers apply.

### Vite proxy timeouts

Set `proxyTimeout: 60000` on `/api/cart` and `/admin/faults` proxy entries. Cart: needed for 30 s outlier knob. Admin: SSE long‑lived stream needs to not be cut.

### Recs refresh interval

Make `services/recommendations/main.go:60` configurable via `RECS_REFRESH_INTERVAL` (default `30s` in demo, `5m` historical). Also add `POST /admin/recs/refresh` (gated by `GRIFFIN_ADMIN_ENABLED`) so the control plane can force a refresh on knob transitions, making the catalog→recs cascade visible without waiting for the next tick.

### Cart HTTP client timeout

Document: cart calls catalog with a 5 s timeout. `catalog.slow` with `LatencyMs >= 4500` will surface as **timeout errors at cart**, not slow requests. UI catalog clamps `LatencyMs` to `[100, 4000]` for the slow knob to keep the demo predictable. (A separate `catalog.timeout` knob is out of v1 scope.)

### Single‑replica control plane

Required for state coherence. K8s overlay enforces:

```yaml
spec:
  replicas: 1
  strategy:
    type: Recreate
```

If we ever scale, we'd need to externalize state (Redis, etc.) — out of scope.

### Trace sampling

Currently the SDK uses default sampling (always‑on root, parent‑based). 100 % capture is required to reliably observe `cart.outlier` traces. Document this constraint; if tail/head sampling is added later, outlier capture must be revisited.

### Load‑bearing tests

Per‑service Go unit tests for `faults.Client` polling, transition diff, and `Middleware` injection logic. Integration test (`integration/`): start control plane + cart + catalog, PUT a `cart.error` knob, verify cart returns 500 within 2 s of PUT. Frontend gets a single Playwright test for the admin page round‑trip if time permits — otherwise manual acceptance.

---

## Open questions for execution

None blocking. Decisions locked above. Items to revisit *after* v1 ships:
- Whether to add `payment.unreachable` as a k8s‑network‑policy knob.
- Whether to extract the control‑plane state into Redis to allow horizontal scale (out of demo scope).
- Whether to integrate flagd / OpenFeature instead of the bespoke client (deferred until we know if there's an audience reason to use it).
