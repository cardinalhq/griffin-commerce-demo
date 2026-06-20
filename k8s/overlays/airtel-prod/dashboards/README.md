# Airtel demo dashboards

`author.py` (Python 3, stdlib only) generates the SQL to create the four
demo dashboards in the Airtel maestro org and emits it on stdout. The
SQL is wrapped in a transaction and uses `DELETE`-then-`INSERT` keyed on
`(org_id, name)` so it's idempotent: rerun to update.

The four dashboards mirror spec §24 of the source telemetry contract:

| Dashboard                              | Purpose                                                            |
| -------------------------------------- | ------------------------------------------------------------------ |
| Airtel — Tenant Health Overview        | Tenant SLO Snapshot, fleet burn / compliance, app vs infra rollup  |
| Airtel — Tenant PostgreSQL Detail      | PG p95 / wait events / checkpoint / cache + VM correlation row     |
| Airtel — VMware Infrastructure         | Datastore + ESXi host + VM + guest layers, blast-radius helper     |
| Airtel — Correlation & Blast Radius    | The cause-to-symptom chain + IndiGo at-risk + alerts table         |

## One-shot deploy

```bash
# port-forward the prod cnpg-cardinal writer
kubectl -n cnpg-cardinal port-forward svc/cnpg-cardinal-rw 15432:5432 &

# generate SQL and apply
python3 author.py > /tmp/airtel-dashboards.sql

PASS=$(kubectl -n maestro get secret pg-credentials -o jsonpath='{.data.MAESTRO_DB_PASSWORD}' | base64 -d)
PGPASSWORD="$PASS" psql -h localhost -p 15432 -U maestro -d maestro -f /tmp/airtel-dashboards.sql
```

The dashboards land in the **Cardinal HQ** org
(`c4375e34-dfcf-498a-8ba3-a02d119baf82`). That's where the prod
node-local OTel collector daemonset routes telemetry; the Airtel org
(`3aa7b421-…`) is the natural-sounding target but the collector isn't
wired to it. Override by editing `ORG_ID` in `author.py` if you point
the collector at a different org.

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
`stacked-bar`, `stacked-area`). Variables drive `$tenant_id` /
`$pg_instance` / `$vm_name` / `$datastore_id` interpolation in queries.
