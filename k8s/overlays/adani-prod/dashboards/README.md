# Adani demo dashboards

`author.py` (Python 3, stdlib only) generates the SQL to create the four
demo dashboards in the Cardinal HQ org of the prod cluster and emits it
on stdout. The SQL is wrapped in a transaction and uses
`DELETE`-then-`INSERT` keyed on `(org_id, name)` so it's idempotent:
rerun to update.

The four dashboards mirror spec §24 of the source telemetry contract:

| Dashboard                                | Purpose                                                              |
| ---------------------------------------- | -------------------------------------------------------------------- |
| Adani — Plant Health Overview            | Offtaker PPA snapshot, fleet PR / dispatch, irradiance, alerts       |
| Adani — Block Detail                     | One block deep — stations / inverters / strings / trackers / met     |
| Adani — Electrical Infrastructure        | MV transformers, compound ambient (shared bay), substation export    |
| Adani — Correlation & Blast Radius       | Cause-to-symptom chain on one axis + same-compound sibling exposure  |

## One-shot deploy

```bash
# port-forward the prod cnpg-cardinal writer
kubectl -n cnpg-cardinal port-forward svc/cnpg-cardinal-rw 15432:5432 &

# generate SQL and apply
python3 author.py > /tmp/adani-dashboards.sql

PASS=$(kubectl -n maestro get secret pg-credentials -o jsonpath='{.data.MAESTRO_DB_PASSWORD}' | base64 -d)
PGPASSWORD="$PASS" psql -h localhost -p 15432 -U maestro -d maestro -f /tmp/adani-dashboards.sql
```

The dashboards land in the **Cardinal HQ** org
(`c4375e34-dfcf-498a-8ba3-a02d119baf82`). That's where the prod
node-local OTel collector daemonset routes telemetry; until the
collector is wired to a dedicated Adani org, dashboards must live there
to match the data. Override `ORG_ID` in `author.py` to retarget.

## Why direct SQL, not the maestro REST API

Same reason as Airtel: `POST /api/orgs/:orgId/dashboards` writes
`created_by` to the authenticated user id, and the system MCP API key
synthesizes the user id as `system:mcp-api-key`, which fails the UUID
cast. `created_by`/`updated_by` are nullable, so direct insert with
NULL sidesteps the issue.

## How the dashboards thread through `correlate_dashboards`

Every dashboard's panels group-by or filter on labels that exist on the
simulator's emitted series:

- **Plant Health Overview** — `offtaker_id`, `block_id`, `met_station_id`,
  `inverter_station_id`, `mv_transformer_id`, `mv_compound_id`
- **Block Detail** — `block_id`, `inverter_station_id`, `inverter_id`,
  `tracker_id`, `met_station_id`
- **Electrical Infrastructure** — `mv_transformer_id`, `mv_compound_id`,
  `substation_id`
- **Correlation & Blast Radius** — every key above, plus the
  cause-to-symptom chain plotted on a single axis

So when the investigator seeds with e.g.
`correlate_dashboards(metric_name="ppa_dispatch_deviation_pct",
filters={"offtaker_id": "seci_phase_iii"})`, the Plant Health Overview
and Correlation dashboards surface. Drilling into
`block_performance_ratio` filtered by `block_id` brings Block Detail.
Drilling further into `mv_transformer_winding_temp_c` filtered by
`mv_transformer_id` brings Electrical Infrastructure — and the chain
terminates at the root cause without the human re-typing filters.

## Editing

Edit `author.py`. The schema mirrors maestro's v2 dashboard spec
(`packages/ui-pages/src/dashboards/v2/types.ts`): 24-column grid,
`panels` keyed by id, `sections[].cells` with `x/y/w/h/i`. Panel kinds:
`label` (single-stat) and `timeseries` (variant: `line`, `stacked-bar`,
`stacked-area`).
