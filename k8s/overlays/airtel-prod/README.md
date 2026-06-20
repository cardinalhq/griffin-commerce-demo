# Airtel telemetry simulator — prod deploy

This overlay deploys the Airtel telemetry simulator (`services/dbaas`) as
a single pod in the `airtel-demo` namespace, sending OTLP metrics and logs
to a Lakerunner ingest endpoint. The pod exposes a small HTTP API on
port `9999` so the demo operator can flip failure profiles via
`kubectl port-forward`.

## Prerequisites

1. A built image of `griffin-commerce-demo` containing the Airtel telemetry
   simulator changes — tag it and update `kustomization.yaml` `newTag` if
   you don't want `latest`.
2. The Lakerunner prod OTLP endpoint URL (HTTPS).
3. A Cardinal API key scoped to the demo org you want the synthetic
   telemetry to land in.

## Operator runbook

1. Create the namespace (kustomize will also do this, but doing it first
   lets you create the Secret before applying):

   ```bash
   kubectl create namespace airtel-demo
   ```

2. Create the Cardinal API key Secret:

   ```bash
   kubectl -n airtel-demo create secret generic cardinal-api-key \
     --from-literal=CARDINAL_API_KEY=<YOUR_KEY_HERE>
   ```

3. Create the OTLP endpoint ConfigMap:

   ```bash
   kubectl -n airtel-demo create configmap dbaas-otlp-config \
     --from-literal=OTEL_EXPORTER_OTLP_ENDPOINT=https://otelhttp.cardinalhq.io
   ```

   Replace the URL with whatever Lakerunner prod OTLP/HTTP endpoint your
   demo org points at. The simulator will append `/v1/metrics` and
   `/v1/logs` itself.

4. Apply the overlay:

   ```bash
   kubectl apply -k k8s/overlays/airtel-prod
   ```

5. Verify the pod is healthy:

   ```bash
   kubectl -n airtel-demo get pods -l app=dbaas
   kubectl -n airtel-demo logs -l app=dbaas --tail=50
   ```

   You should see `Airtel telemetry simulator running. Waiting for shutdown
   signal.` once startup completes.

6. At baseline you should immediately see metrics in Cardinal under the
   demo org with `scenario_id=airtel_postgres_on_vmware_shared_vm_infra`:

   - `tenant_slo_burn_rate` — 6 tenants, all values < 1
   - `pg_query_latency_p95_ms` — 4 PG instances at 35–90 ms
   - `vmware_datastore_write_latency_ms` — 3 datastores at 2–12 ms

## Triggering a failure

The pod listens on port 9999 inside the cluster only (Service is
ClusterIP). Port-forward to drive it locally:

```bash
kubectl -n airtel-demo port-forward svc/dbaas-faults 9999:9999
```

In another shell:

```bash
# List available profiles
curl -s localhost:9999/faults/profiles | jq

# Activate the canonical demo scenario (datastore-level shared-infra story)
curl -sX POST 'localhost:9999/faults/activate?profile=datastore_latency_shared_infra' | jq

# Check current state
curl -s localhost:9999/faults/status | jq

# Clear when done
curl -sX POST localhost:9999/faults/clear | jq
```

The four failure profiles per spec §29:

| Profile ID                          | Story                                                          |
| ----------------------------------- | -------------------------------------------------------------- |
| `vm_local_disk_saturation`          | PG slow because the VM's local disk is saturated               |
| `vm_cpu_ready_contention`           | PG slow because the host is over-committed (noisy neighbor)    |
| `vm_memory_pressure_swap`           | PG slow because the VM is swapping                             |
| `datastore_latency_shared_infra`    | PG slow because a shared datastore is degraded (blast radius)  |

Each profile runs for 35 minutes with a 2-minute ramp-up and 5-minute
ramp-down. Within the first 2–3 minutes infrastructure-layer metrics
degrade; PostgreSQL symptoms appear 4–7 minutes after activation;
tenant SLO breach appears 9–12 minutes after activation. The
`datastore_latency_shared_infra` profile also degrades the at-risk tenant
(`tenant_indigo_ops`) starting ~12 minutes in, at a lower amplitude per
spec §22.4.

## Tear-down

```bash
kubectl delete -k k8s/overlays/airtel-prod
kubectl delete namespace airtel-demo
```

## Environment knobs

| Env var                         | Default | Purpose                                               |
| ------------------------------- | ------- | ----------------------------------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT`   | n/a     | Lakerunner OTLP/HTTP URL (from ConfigMap)             |
| `CARDINAL_API_KEY`              | n/a     | Cardinal demo-org API key (from Secret)               |
| `DBAAS_FAULT_PORT`              | 9999    | Port the local fault HTTP server binds                |
| `DBAAS_LOG_INTERVAL_SEC`        | 5       | Seconds between log-emission ticks                    |
| `DBAAS_EMIT_PG_HIST`            | true    | Set `false` to drop the 264-series PG latency buckets |
