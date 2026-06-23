# Adani Solar Farm Simulator

**Service:** `services/solar` (cobra subcommand `griffin solar`)
**Deploy:** `k8s/overlays/adani-prod/`
**Source plant:** Adani Khavda Renewable Energy Park, Kutch, Gujarat

## Purpose

The Adani solar farm simulator is a single-pod synthetic telemetry source
for the Adani Renewable Ops demo. It emits OTLP metrics and logs that
look like a real fleet of grid-tied PV blocks in a giga-scale solar park
— inverters, MV step-up transformers, trackers, met stations, and the
substation feeding the grid — with PPAs (Power Purchase Agreements) to
SECI, GUVNL, and Adani Electricity Mumbai standing in as the
tenant-style SLO surface (commanded MW vs actual MW dispatch).

A demo operator activates one of four failure profiles via a
port-forwarded HTTP endpoint to walk Cardinal through the
PPA-deviation → block-PR → inverter-derate → MV-transformer story.

The simulator targets the lakerunner prod cluster
(`aws-prod-us-east-2-global`) by default, sending OTLP to the
node-local OTel collector DaemonSet (`http://$(HOST_IP):4318`) — the
same wiring `services/dbaas` (Airtel) uses. No Cardinal API key is
provisioned at the pod; the collector handles upstream routing.

The four failure profiles, four dashboards, and the canonical story arc
are designed end-to-end around the `correlate_dashboards` MCP tool:
every per-entity metric carries the join keys (`offtaker_id`, `block_id`,
`inverter_station_id`, `mv_transformer_id`, `mv_compound_id`,
`inverter_id`) needed to chain symptom → cause across the four
dashboards without a human re-typing filters.

## Entity catalog (§4)

One site (`khavda-1`), three offtakers (SECI Phase III / GUVNL / Adani
Electricity Mumbai), one substation, four MV transformers, six blocks,
twenty-four inverter stations, ninety-six inverters, twelve met
stations, and six trackers (one per block).

| Entity | Count | Notes |
| ------ | ----: | ----- |
| Site | 1 | `khavda-1` |
| Offtakers | 3 | `seci_phase_iii`, `guvnl_state`, `adani_mumbai` |
| Substation | 1 | 220/400 kV pooling substation `gss-khavda-01` |
| MV transformers | 4 | `T-04-A`, `T-04-B`, `T-08`, `T-10` — see compound mapping below |
| MV compounds | 3 | `mvc-04`, `mvc-08`, `mvc-10` — shared cooling-shed grouping |
| Blocks | 6 | `block-04`, `block-06`, `block-08`, `block-10`, `block-12`, `block-14` |
| Inverter stations (PCS) | 24 | 4 per block |
| Inverters | 96 | 4 per station (3.125 MW each → ~12.5 MW per station → 300 MW total) |
| Trackers | 6 | one aggregated controller per block |
| Met stations | 12 | two per block |

### MV transformer ↔ block ↔ compound mapping

The blast-radius story (`mv_transformer_winding_overheat`) hinges on
`T-04-A` and `T-04-B` sharing `mvc-04` — the same outdoor cooling shed,
same radiator bank ambient.

| Block | MV transformer | MV compound | Offtaker |
| ----- | -------------- | ----------- | -------- |
| block-04 | T-04-A | mvc-04 | seci_phase_iii |
| block-06 | T-04-B | mvc-04 | guvnl_state |
| block-08 | T-08 | mvc-08 | guvnl_state |
| block-10 | T-10 | mvc-10 | adani_mumbai |
| block-12 | T-04-A | mvc-04 | seci_phase_iii |
| block-14 | T-04-B | mvc-04 | guvnl_state |

The `catalog_test.go` invariants enforce this so renames cannot silently
break the demo.

## Metric inventory

All instrument names are stable strings (`adani_*` / `inverter_*` /
`mv_transformer_*` / `block_*` / `ppa_*` / `met_*` / `substation_*`) so
dashboards and the `correlate_dashboards` tool can grep them.

| Family | Metrics | Cardinality at idle |
| ------ | ------- | ------------------- |
| PPA / offtaker | 5 | ~15 |
| Block | 4 | ~24 |
| Inverter station | 4 | ~96 |
| Inverter | 13 | ~1250 |
| String (per-inverter aggregate) | 2 | ~190 |
| MV transformer | 8 | ~32 |
| Tracker | 4 | ~24 |
| Met station | 7 | ~84 |
| Site / substation | 5 | ~6 |
| Alert | 1 | 0–3 during incident |

Total: ~1750 series at idle, well within the source-spec target. The
heaviest contributor is per-inverter (96 inverters × 13 metrics). To
trim cardinality below 1 k, set `SOLAR_EMIT_PER_INVERTER=false` and the
per-inverter family rolls up to per-inverter-station only.

### Standard tag set

Every metric emits a subset of these resource + per-metric attributes
(only the ones meaningful to that metric):

```
site_id, plant_name, region, state, country,
offtaker_id, ppa_id, ppa_tier,
block_id, mv_transformer_id, mv_compound_id,
inverter_station_id, inverter_id, inverter_model, inverter_vendor,
string_id, tracker_id, met_station_id,
substation_id, scenario_id, scenario_version
```

The join keys are: `offtaker_id` → `block_id` → `inverter_station_id`
→ `inverter_id`, and orthogonally `block_id` → `mv_transformer_id` →
`mv_compound_id`. The blast-radius narrative is reachable through the
`mv_compound_id` join.

## Ramp model

Each failure profile sets a 35-minute duration with a 2-minute ramp-up
and 5-minute ramp-down. The trapezoid factor for a metric at time `t`
is identical to the Airtel simulator (`services/dbaas/scenario.go`):

```
elapsed = (t - profile.started) - metric.onsetOffset
factor = 0                                   if elapsed ≤ 0 or ≥ duration
factor = elapsed / 2m                        during ramp-up
factor = 1                                   during plateau
factor = (duration - elapsed) / 5m           during ramp-down
```

`RampedValue` linearly interpolates between the metric's baseline range
sample and its incident range sample. Both ranges are sampled with a
deterministic seed derived from the entity, so retries within a scrape
return the same value.

## Failure profiles (§29)

| Profile ID | Root layer | Primary entity | Secondary impact |
| ---------- | ---------- | -------------- | ---------------- |
| `mv_transformer_winding_overheat` | mv_transformer | T-04-A | sibling T-04-B in mvc-04 (at-risk) |
| `inverter_cooling_fault` | inverter | INV-08-02-03 | none — local |
| `tracker_stow_misalignment` | tracker | TRK-12 | block-12 PR drop, dust-storm coupled |
| `string_pid_degradation` | string | INV-10-03-01 | chronic — strings 1–12 only |

### Canonical profile — `mv_transformer_winding_overheat`

Cooling oil flow on **T-04-A** drops over the course of two minutes
(degraded MOV / clogged radiator). Winding insulation temp climbs.
Bay-level temperature monitor throttles the transformer load → all 4
inverter stations on T-04-A derate → Block-04 + Block-12 AC power
drops → SECI Phase III dispatch shortfall → PPA burn rate climbs.

`T-04-B` shares the `mvc-04` outdoor compound and radiator bank ambient
with T-04-A. When T-04-A's cooling fans run at max, the compound ambient
temp rises 6–10 °C, which slowly bumps T-04-B's winding temp into the
warning band (but not the trip band). Block-06 and Block-14 see mild PR
drops via the GUVNL PPA dispatch.

Temporal order (onset offsets from activation):

```
+0m   T-04-A oil_flow_lpm drops, winding_temp + oil_temp climb
+2m   T-04-A radiator_temp climbs; load_kva throttles
+4m   IS-04-01..04 + IS-12-01..04 ac_voltage dips, derate state = 1
+5m   per-inverter igbt_temp climbs, inverter_derate_state = 1
+7m   block-04 + block-12 ac_power_mw drops, performance_ratio drops
+9m   substation export_power_mw delta visible (T-04-A bay contribution)
+11m  seci_phase_iii ppa_dispatch_deviation_pct crosses 5%, burn_rate climbs
+13m  T-04-B winding_temp + oil_temp creep up (ambient-shared from mvc-04)
+15m  block-06 + block-14 performance_ratio + ac_power_mw drop mildly
+16m  guvnl_state ppa_dispatch_deviation_pct nudges up at at-risk amplitude
```

## Logs

`logs.go` runs a 5-second ticker (`SOLAR_LOG_INTERVAL_SEC`). Per tick,
for every (entity, event_type) pair it Poisson-samples a count using a
baseline or incident frequency, then emits that many records via the
global OTel logger. Every record carries the full correlation field set
(`site_id`, `block_id`, `inverter_station_id`, `inverter_id`,
`mv_transformer_id`, `mv_compound_id`, `offtaker_id`).

Event types (spec):

- `inverter_derate` — `Inverter %s entered derate due to internal temp %.1f°C`
- `inverter_trip` — `Inverter %s tripped: ground_fault` / `over_temp` / `dc_overvoltage`
- `inverter_mppt_chase` — `Inverter %s MPPT search oscillating, dc_v=%.0f`
- `mv_transformer_winding_alarm` — `Transformer %s winding HV %.1f°C exceeds warning (90°C)`
- `mv_transformer_oil_low_flow` — `Transformer %s cooling oil flow %.1f LPM below threshold`
- `mv_transformer_buchholz` — `Transformer %s Buchholz relay gas accumulation alarm`
- `tracker_fault` — `Tracker %s motor over-current, fault_code=%s`
- `tracker_stow_failed` — `Tracker %s stow command failed, current angle %.1f° vs target %.1f°`
- `met_soiling_threshold` — `Met %s soiling loss crossed %.1f%%`
- `scada_disconnect` — `SCADA link to %s lost for %ds`
- `ppa_schedule_breach` — `PPA %s actual %.1f MW vs scheduled %.1f MW (deviation %.1f%%)`
- `correlation_discovered` — synthetic per-profile correlation marker

Severity mapping: WARN → `Warn`, ERROR → `Error`, INFO → `Info`,
HIGH → `Error2`, CRITICAL → `Fatal`. The original token is preserved in
attribute `severity_text`.

## HTTP fault endpoint

`httpserver.go` binds `:9999` and exposes:

| Method | Path                  | Behavior                                                |
| ------ | --------------------- | ------------------------------------------------------- |
| POST   | `/faults/activate`    | `?profile=<id>` — activate (replaces any active)        |
| POST   | `/faults/clear`       | Clear any active profile                                |
| GET    | `/faults/status`      | Return active profile id + elapsed seconds              |
| GET    | `/faults/profiles`    | List known profile ids                                  |
| GET    | `/healthz`            | 200 if the process is up                                |

Single-active semantics — activating profile B while profile A is
running replaces A immediately. The server only binds inside the cluster
(Service type ClusterIP); demo operators reach it via
`kubectl port-forward`.

## Deploy / smoke flow

See `k8s/overlays/adani-prod/README.md` for the full runbook. In short:

```bash
kubectl apply -k k8s/overlays/adani-prod
kubectl -n adani-demo port-forward svc/solar-faults 9999:9999
curl -sX POST 'localhost:9999/faults/activate?profile=mv_transformer_winding_overheat'
```

## Dashboards (§24)

Four dashboards generated by
`k8s/overlays/adani-prod/dashboards/author.py`:

1. **Adani — Plant Health Overview** — fleet posture, PPA SLO snapshot,
   block PR rollup, irradiance, alerts. The morning-coffee dashboard.
2. **Adani — Block Detail** — per-block inverter stations, inverter
   table, strings, trackers, met. Drives the variable `$block_id`.
3. **Adani — Electrical Infrastructure** — MV transformers (winding /
   oil / load / cooling), substation, feeder pooling. Drives
   `$mv_transformer_id`.
4. **Adani — Correlation & Blast Radius** — cause-to-symptom chain on
   one axis, at-risk siblings on `mv_compound_id`, alerts table.

Every dashboard's variables (`$offtaker_id`, `$block_id`,
`$mv_transformer_id`, `$mv_compound_id`, `$inverter_station_id`,
`$inverter_id`) are sourced from labels that exist on the simulator's
metrics, so `correlate_dashboards` reaches them in one hop.

## Tests

`go test ./services/solar/...` covers:

- `catalog_test.go` — entity counts, T-04-A and T-04-B share `mvc-04`,
  block-04 + block-12 wired to T-04-A, block-06 + block-14 wired to
  T-04-B, helper lookups (`InvertersInBlock`, `BlocksOnTransformer`,
  `TransformersInCompound`).
- `scenario_test.go` — trapezoid factor at every boundary, RampedValue
  baseline vs primary plateau, Activate replace semantics, unknown
  profile rejected, Clear no-op when nothing active, Range determinism.
- `httpserver_test.go` — activate (valid / unknown / missing param /
  wrong method), clear, status, profiles, replace semantics through
  the handler.
