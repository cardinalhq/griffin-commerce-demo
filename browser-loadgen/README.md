# browser-loadgen

Headless Chromium (Playwright) driving the Griffin frontend continuously so
the RUM demo has a live signal without a human at a keyboard. Each worker
opens a fresh browser context per iteration — that resets `session.id` in
the OTel-web SDK, so the collector sees a stream of distinct sessions
rather than one long-lived one.

## Environment

| Variable                   | Default                  | Purpose                                                   |
|----------------------------|--------------------------|-----------------------------------------------------------|
| `TARGET_URL`               | `http://frontend:5173`   | Frontend to drive. In-cluster Service name in k8s.        |
| `CONCURRENCY`              | `2`                      | Parallel browser contexts (1–32).                         |
| `PAUSE_MIN_MS`             | `1000`                   | Lower bound on per-step jitter and inter-journey pause.   |
| `PAUSE_MAX_MS`             | `5000`                   | Upper bound on the same.                                  |
| `JOURNEY_TIMEOUT_MS`       | `60000`                  | Hard cap on a single journey; on timeout we bail cleanly. |
| `NAV_TIMEOUT_MS`           | `30000`                  | Per-navigation timeout.                                   |
| `CHAOS_ROUTE_PROBABILITY`  | `0.05`                   | Chance a journey visits `/chaos` (admin UI).              |
| `HEADLESS`                 | `true`                   | Set `false` to see the browser (docker-compose only).     |
| `USER_AGENT_SUFFIX`        | `GriffinBrowserLoadgen`  | Appended to the UA so RUM sessions are self-identifying.  |

## Run locally

Standalone (assumes the frontend is already reachable at
`http://localhost:5173`):

```bash
cd browser-loadgen
npm install
TARGET_URL=http://localhost:5173 CONCURRENCY=1 HEADLESS=false node run.js
```

Or via docker-compose from the repo root:

```bash
docker-compose up browser-loadgen
```

## How this produces RUM

1. Playwright launches Chromium.
2. It loads the Griffin frontend's index.html.
3. The frontend loads `/rum-config.js` (rendered by nginx from the
   environment at container start) and imports `@opentelemetry/sdk-trace-web`.
4. Auto-instrumentations attach: document-load, fetch, XHR, and click /
   submit user-interactions. web-vitals fires LCP / CLS / INP / FCP / TTFB
   as short standalone spans.
5. The SDK OTLP-batches over HTTP to `/v1/traces` and `/v1/logs`, which
   nginx proxies to the collector configured by `OTEL_COLLECTOR_HOST`.

There is no separate egress from the loadgen container itself — the point
is to force the browser to do the work.
