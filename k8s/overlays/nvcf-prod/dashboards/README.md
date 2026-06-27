# NVCF demo dashboards

`author.py` (Python 3, stdlib only) generates the SQL to create the five
NVCF M1 demo dashboards in the Cardinal demo org and emits it on stdout.
The SQL is wrapped in a transaction and uses `DELETE`-then-`INSERT` keyed
on `(org_id, name)` so it's idempotent: rerun to update.

The five dashboards mirror the registry in `docs/specs/nvcf.md §Dashboards`:

| Dashboard                                | Purpose                                                                 |
| ---------------------------------------- | ----------------------------------------------------------------------- |
| NVCF — Executive Overview                | Fleet snapshot, traffic + GPU, latency + throughput, tenant + KV cache  |
| NVCF — Per-function Deep Dive            | One `$function_id`: traffic, latency (e2e + TTFT), throughput, capacity |
| NVCF — GPU & Compute Fleet               | One `$nvca_cluster_name`: GPU util by host/device/model, crashes        |
| NVCF — Tenant Fairness                   | One `$account_name`: per-account QD + invocations, fleet share          |
| NVCF — Deployment Regression             | `$function_id` × `$function_version_id`: the **ttft-regression** demo   |

Out of scope until M2 (in the spec, but no metric landing yet):
cold-start init seconds, thermal divergence, router metrics, gateway
connection metrics, registry/cache failures, rate-limit events,
allocatable-vs-capacity.

The M1 emitter publishes TTFT / output_tps / function_latency as
**gauges** rather than histograms, so the spec's `histogram_quantile(p95,
... _bucket)` panels render here as gauge averages. Histogram conversion
is a parallel M2 work item.

## One-shot deploy

```bash
# port-forward the prod cnpg-cardinal writer
kubectl -n cnpg-cardinal port-forward svc/cnpg-cardinal-rw 15432:5432 &

# generate SQL and apply
python3 author.py > /tmp/nvcf-dashboards.sql

PASS=$(kubectl -n maestro get secret pg-credentials -o jsonpath='{.data.MAESTRO_DB_PASSWORD}' | base64 -d)
PGPASSWORD="$PASS" psql -h localhost -p 15432 -U maestro -d maestro -f /tmp/nvcf-dashboards.sql
```

The dashboards land in the Cardinal demo org
(`3aa7b421-0ecb-48a8-bf3a-b7397814862a`, named "Airtel" historically —
same org the airtel/adani demo pods send telemetry to via the
node-local OTel agent DaemonSet). Open them in the Cardinal UI under
`Dashboards` once the nvcf-prod pod is producing data.

## Why direct SQL, not the maestro REST API

Maestro's `POST /api/orgs/:orgId/dashboards` writes `created_by` to the
authenticated user id. When called with the `X-CardinalHQ-API-Key`
system key, the synthesized context's user id is the literal string
`system:mcp-api-key`, which fails the UUID cast on the column.
`created_by`/`updated_by` are nullable, so direct insert with NULL
sidesteps the issue without a maestro-side patch.

## Editing

Edit `author.py` — it's the single source. The schema mirrors maestro's
v2 dashboard spec (`packages/ui-pages/src/dashboards/v2/types.ts`):
24-column grid, `panels` keyed by id, `sections[].cells` with `x/y/w/h/i`.
Panel kinds: `label` (single-stat) and `timeseries` (variant: `line`,
`stacked-bar`, `stacked-area`). Variables drive `$function_id` /
`$function_version_id` / `$nvca_cluster_name` / `$account_name`
interpolation in queries.

## Acceptance check (for the ttft-regression knob)

After applying the dashboards and activating the knob:

```bash
kubectl -n nvcf-demo port-forward svc/nvcf-faults 9998:9998
curl -X POST 'http://localhost:9998/faults/activate?profile=function.ttft-regression'
```

Open **NVCF — Deployment Regression**, pick a function whose v2 is the
ttft-regression target, and watch panel `rg_ttft_overlay`: v2's curve
elevates 2-3x within ~90 seconds while v1 stays at baseline.
