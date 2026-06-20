# Airtel Telemetry Simulator

**Service:** `services/dbaas` (cobra subcommand `griffin dbaas`)
**Deploy:** `k8s/overlays/airtel-prod/`
**Source spec:** Airtel demo spec v0.2 (Postgres on VMware shared VMs)

## Purpose

The Airtel telemetry simulator is a single-pod synthetic telemetry source
for the Airtel Managed Services demo. It emits OTLP metrics and logs that
look like a real fleet of PostgreSQL instances running on VMware VMs under
Airtel Cloud, sharing ESXi hosts and datastores. A demo operator activates
one of four failure profiles via a port-forwarded HTTP endpoint to walk
Cardinal through the tenant-SLO → application → infrastructure
investigation flow.

The simulator targets the lakerunner prod cluster
(`aws-prod-us-east-2-global`) by default, sending OTLP to a Lakerunner
ingest endpoint with a Cardinal API key supplied at deploy time.

## Entity catalog (spec §4)

Six tenants — Bajaj Finance, IndiGo Operations, Apollo Health, ACME Retail,
Mahindra Auto, HDFC Life. One vCenter (`vcenter-delhi-01`), two clusters
(`cluster-delhi-prod-a/b`), four ESXi hosts, three datastores, six VMs
(five tenant VMs + one noisy-neighbor), four PG instances.

The shared-infra blast-radius story (§22.4 of the source spec) hinges on
`vm-bajaj-pg-01` and `vm-indigo-pg-01` both running on
`host-1017` / `datastore-202`. The `catalog_test.go` invariants enforce
this so renames cannot silently break the demo.

## Metric inventory

All instrument names match the source spec literally so Cardinal can search
for `pg_query_latency_p95_ms`, `vmware_datastore_write_latency_ms`, etc.

| Family       | Metrics | Source spec § | Cardinality at idle |
| ------------ | ------- | ------------- | ------------------- |
| Tenant SLO   | 3       | §7            | ~18                 |
| Postgres     | 16      | §8            | ~190 (gauges+counters), +312 with histogram |
| Linux VM     | 16      | §9            | ~80                 |
| VMware VM    | 10      | §10           | ~55                 |
| ESXi host    | 7       | §11           | ~30                 |
| Datastore    | 7       | §12           | ~21                 |
| Probe        | 2       | §13           | ~12                 |
| Alert        | 1       | §14           | 0–2 during incident |

Total: ~620 series without the PG latency histogram, ~930 with it. Within
the source spec §3 target of 800–1500. The histogram is gated on
`DBAAS_EMIT_PG_HIST` (default true).

## Ramp model

Each failure profile sets a 35-minute duration with a 2-minute ramp-up and
5-minute ramp-down. The trapezoid factor for a metric at time `t` is:

```
elapsed = (t - profile.started) - metric.onsetOffset
factor = 0                                   if elapsed ≤ 0 or ≥ duration
factor = elapsed / 2m                        during ramp-up
factor = 1                                   during plateau
factor = (duration - elapsed) / 5m           during ramp-down
```

`RampedValue` linearly interpolates between the metric's baseline range
sample and its incident range sample using the factor. Both ranges are
sampled with a deterministic seed derived from the entity (so retries
within a scrape return the same value).

## Failure profiles (spec §29)

| Profile ID                          | Root layer | Primary VM        | Secondary impact                            |
| ----------------------------------- | ---------- | ----------------- | ------------------------------------------- |
| `vm_local_disk_saturation`          | linux_vm   | vm-bajaj-pg-01    | none                                        |
| `vm_cpu_ready_contention`           | vmware_vm  | vm-bajaj-pg-01    | noisy neighbor on host-1017                 |
| `vm_memory_pressure_swap`           | linux_vm   | vm-bajaj-pg-01    | none                                        |
| `datastore_latency_shared_infra`    | datastore  | vm-bajaj-pg-01    | at-risk vm-indigo-pg-01 (shared host + ds)  |

For each profile, `scenario.go` holds a per-metric `OnsetOffsetMin` value
that pushes that metric's ramp-up later. Source-spec §28.2 dictates the
temporal order for `datastore_latency_shared_infra`:

```
+0m   datastore-202 write/read latency, queue depth, IOPS
+2m   host-1017 disk + CPU
+3m   vm-bajaj-pg-01 VMware disk + CPU ready
+4m   vm-bajaj-pg-01 Linux iowait / disk_io_now / CPU steal
+7m   pg-bajaj-01 query latency, checkpoint rates, cache hit, locks
+10m  tenant_bajaj_finance probe success + latency
+11m  tenant_bajaj_finance SLO burn + compliance + error budget
+12m  vm-indigo-pg-01 VMware/Linux at at-risk amplitudes
+14m  pg-indigo-01 query latency
+15m  tenant_indigo_ops probe + SLO at at-risk amplitudes
```

## Logs

`logs.go` runs a 5-second ticker (`DBAAS_LOG_INTERVAL_SEC`). Per tick, for
every (entity, event_type) pair it Poisson-samples a count using a
baseline or incident frequency from the source spec, then emits that many
records via the global OTel logger. Every record carries the full
correlation field set (`tenant_id`, `pg_instance`, `vm_uuid`,
`esxi_host_id`, `datastore_id`) so join queries work without joins.

Severity mapping: WARN → `Warn`, ERROR → `Error`, INFO → `Info`,
HIGH → `Error2`, DISASTER/CRITICAL → `Fatal`. The original token is
preserved in attribute `severity_text` for filters that expect the spec
verbatim.

## HTTP fault endpoint

`httpserver.go` binds `:9999` and exposes:

| Method | Path                  | Behavior                                                |
| ------ | --------------------- | ------------------------------------------------------- |
| POST   | `/faults/activate`    | `?profile=<id>` — activate (replaces any active)        |
| POST   | `/faults/clear`       | Clear any active profile                                |
| GET    | `/faults/status`      | Return active profile id + elapsed seconds              |
| GET    | `/faults/profiles`    | List known profile ids                                  |
| GET    | `/healthz`            | 200 if the process is up                                |

Single-active semantics — activating profile B while profile A is running
replaces A immediately. The server only binds inside the cluster (Service
type ClusterIP); demo operators reach it via `kubectl port-forward`.

## Deploy / smoke flow

See `k8s/overlays/airtel-prod/README.md` for the full runbook. In short:

```bash
kubectl create namespace airtel-demo
kubectl -n airtel-demo create secret generic cardinal-api-key --from-literal=CARDINAL_API_KEY=...
kubectl -n airtel-demo create configmap dbaas-otlp-config --from-literal=OTEL_EXPORTER_OTLP_ENDPOINT=https://...
kubectl apply -k k8s/overlays/airtel-prod
kubectl -n airtel-demo port-forward svc/dbaas-faults 9999:9999
curl -sX POST 'localhost:9999/faults/activate?profile=datastore_latency_shared_infra'
```

## Tests

`go test ./services/dbaas/...` covers:

- `scenario_test.go` — trapezoid factor at every boundary, RampedValue
  baseline vs primary plateau, Activate replace semantics, unknown profile
  rejected, Clear no-op when nothing active, Range determinism.
- `httpserver_test.go` — activate (valid / unknown / missing param / wrong
  method), clear, status, profiles, replace semantics through the handler.
- `catalog_test.go` — entity counts, every PG has a VM-host-datastore
  chain, primary and at-risk VMs share the degraded host and datastore,
  noisy neighbor shares the host with primary, helper lookups.

## Reference

- Source spec on operator workstation:
  `~/Desktop/airtel_postgres_vmware_telemetry_spec.md`
- Existing griffin commerce demo spec: `docs/demo/airtel-install.md`
