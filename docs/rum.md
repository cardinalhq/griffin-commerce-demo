# Browser RUM

Real-User Monitoring for the Griffin frontend, plus a headless-browser
loadgen that keeps the signal alive so the demo doesn't need a human at a
keyboard.

## Architecture

```
                    ┌────────────────────────────────────────────┐
                    │  browser (Chromium via Playwright OR user) │
                    │                                            │
                    │   @opentelemetry/sdk-trace-web             │
                    │   @opentelemetry/api-logs                  │
                    │   web-vitals                               │
                    └──────────────────┬─────────────────────────┘
                                       │ OTLP-HTTP protobuf, same-origin
                                       │ POST /v1/traces, /v1/logs
                                       ▼
       ┌────────────────────────────────────────────────────────┐
       │  frontend Pod (nginx + Svelte SPA)                     │
       │                                                        │
       │   nginx.conf.template locations /v1/*  ─────►          │
       │   /rum-config.js  ←── rendered at boot by entrypoint   │
       └──────────────────┬─────────────────────────────────────┘
                          │ HTTP
                          ▼
       ┌────────────────────────────────────────────────────────┐
       │  OTLP collector (per-cluster; whatever                 │
       │  OTEL_COLLECTOR_HOST resolves inside the pod)          │
       └──────────────────┬─────────────────────────────────────┘
                          ▼
                  Cardinal intake (via cluster gateway)
```

The browser and the Go backends share the trace context — a click
initiates a fetch, the fetch instrumentation stamps `traceparent`, the
Go handler continues the same trace. RUM sessions correlate end-to-end.

## What's emitted

| Signal            | Source                                                 |
|-------------------|--------------------------------------------------------|
| document-load     | `@opentelemetry/instrumentation-document-load`         |
| fetch spans       | `@opentelemetry/instrumentation-fetch`                 |
| XHR spans         | `@opentelemetry/instrumentation-xml-http-request`      |
| user-interactions | `@opentelemetry/instrumentation-user-interaction` (click, submit) |
| web vitals        | `web-vitals` → short standalone spans (`web-vital.LCP`, `.CLS`, `.INP`, `.FCP`, `.TTFB`) |
| console.error     | patched to also emit an OTel log record               |
| window.onerror    | emits OTel log record with exception attributes       |
| unhandledrejection| emits OTel log record                                  |

Every signal is stamped with:

- `service.name` (default `griffin-frontend`)
- `service.namespace` (mirrors the backends)
- `session.id` (fresh UUID per page load)
- `browser.language`, `browser.user_agent`, `page.url`
- Everything in `RUM_RESOURCE_ATTRIBUTES_JSON`

## Runtime configuration

`entrypoint.sh` renders `/rum-config.js` at container start from these
environment variables. Change one and restart the pod — no rebuild.

| Variable                            | Default              | Purpose                                                                        |
|-------------------------------------|----------------------|--------------------------------------------------------------------------------|
| `RUM_ENABLED`                       | `false`              | Master switch. Off → SDK is a no-op.                                            |
| `OTEL_COLLECTOR_HOST`               | `127.0.0.1`          | nginx `proxy_pass` target for `/v1/*`. In-cluster Service DNS or `$(HOST_IP)`. |
| `OTEL_COLLECTOR_PORT`               | `4318`               | OTLP-HTTP port on the collector.                                                |
| `RUM_OTLP_PATH`                     | `/v1`                | Same-origin base. Exporter appends `/traces` and `/logs`.                       |
| `RUM_SERVICE_NAME`                  | `griffin-frontend`   | resource `service.name`.                                                        |
| `RUM_SERVICE_NAMESPACE`             | ``                   | resource `service.namespace` — set to whatever backends set.                    |
| `RUM_SERVICE_VERSION`               | ``                   | resource `service.version`.                                                     |
| `RUM_RESOURCE_ATTRIBUTES_JSON`      | `{}`                 | JSON object of extra resource attrs. Verbatim inline in `rum-config.js`.        |
| `RUM_PROPAGATE_HEADER_CORS_URLS_JSON`| `[]`                | JSON array of regex sources. Adds `traceparent` to cross-origin URLs.           |
| `RUM_DEBUG`                         | `false`              | Turns on `@opentelemetry/api` diag console logging.                             |

## Enabling in a k8s deployment

Layer the `with-rum` overlay on top of `base`:

```yaml
# k8s/overlays/<yours>/kustomization.yaml
resources:
  - ../../k8s/overlays/with-rum

patches:
  - target: { kind: Deployment, name: frontend }
    patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/env/3   # OTEL_COLLECTOR_HOST
        value: { name: OTEL_COLLECTOR_HOST, value: "otel-agent.observability.svc.cluster.local" }
```

Or write directly into your cluster kustomization the way the backend
overlays (`with-otlp-nodelocal`, `with-otlp-url`) already do. If your
backends export via the per-node agent DaemonSet (`$(HOST_IP):4318`), do
the same for the frontend by adding a `HOST_IP` downward-API env and
setting `OTEL_COLLECTOR_HOST` to `$(HOST_IP)`.

## Enabling locally

```bash
docker-compose up otel-collector browser-loadgen frontend catalog cart payment images shipping recommendations controlplane
```

The `frontend` service already sets `RUM_ENABLED=true` and points at the
local collector. Tail `docker-compose logs -f otel-collector` — browser
spans and web-vitals arrive within a few seconds of each browser-loadgen
journey. Open http://localhost:5173/ yourself to add manual traffic.

## Loadgen

See [`browser-loadgen/README.md`](../browser-loadgen/README.md). Each
container runs N Chromium contexts through short randomised journeys —
homepage → product detail → add-to-cart → occasional /chaos. Every
journey ends by closing the browser context so `session.id` rotates and
the collector sees a stream of distinct sessions.

## Bundle cost

The SDK adds ~200KB unminified / ~55KB gzipped over the Svelte bundle.
