# Adani Khavda solar farm simulator — prod deploy

This overlay deploys the Adani solar farm telemetry simulator
(`services/solar`) as a single pod in the `adani-demo` namespace of the
lakerunner prod cluster (`aws-prod-us-east-2-global`). Telemetry flows
via the node-local OTel collector DaemonSet
(`collector/aws-prod-us-east-2-global-agent`) at `http://$(HOST_IP):4318`
— same pattern the existing `griffin-demo` and `airtel-demo` namespaces
use. No Cardinal API key is provisioned at the pod; the collector
handles upstream routing.

The pod exposes an HTTP API on port `9999` so the demo operator can flip
failure profiles via `kubectl port-forward`.

## Prerequisites

- A built image of `griffin-commerce-demo` containing the solar
  simulator changes — pin its tag in `kustomization.yaml`
  (`images.newTag`).
- The node-local OTel collector DaemonSet
  (`collector/aws-prod-us-east-2-global-agent`) already running, which
  it is in `aws-prod-us-east-2-global`. Nothing to install here.

## Operator runbook

1. Apply the overlay:

   ```bash
   kubectl apply -k k8s/overlays/adani-prod
   ```

   This creates the `adani-demo` namespace, the `solar` Deployment, and
   the `solar-faults` Service (ClusterIP, port 9999).

2. Verify the pod is healthy:

   ```bash
   kubectl -n adani-demo get pods -l app=solar
   kubectl -n adani-demo logs -l app=solar --tail=50
   ```

   You should see `Adani solar telemetry simulator running. Waiting for
   shutdown signal.` once startup completes.

3. At baseline you should immediately see metrics with
   `scenario_id=adani_khavda_solar_park` in whichever Cardinal org the
   node-local collector routes to:

   - `ppa_dispatch_deviation_pct` — 3 offtakers, values < 2%
   - `block_performance_ratio` — 6 blocks at 0.94–0.985
   - `mv_transformer_winding_temp_c` — 4 transformers at 62–76°C
   - `inverter_ac_power_kw` — 96 inverters at ~2.8–3.0 MW
   - `met_irradiance_poa_w_m2` — 12 met stations at 820–940 W/m²

## Triggering a failure

The pod listens on port 9999 inside the cluster only (Service is
ClusterIP). Port-forward to drive it locally:

```bash
kubectl -n adani-demo port-forward svc/solar-faults 9999:9999
```

In another shell:

```bash
# List available profiles
curl -s localhost:9999/faults/profiles | jq

# Activate the canonical demo scenario (MV transformer shared-infra blast radius)
curl -sX POST 'localhost:9999/faults/activate?profile=mv_transformer_winding_overheat' | jq

# Check current state
curl -s localhost:9999/faults/status | jq

# Clear when done
curl -sX POST localhost:9999/faults/clear | jq
```

The four failure profiles (spec §29):

| Profile ID                          | Story                                                                       |
| ----------------------------------- | --------------------------------------------------------------------------- |
| `mv_transformer_winding_overheat`   | Shared-infra blast radius — T-04-A winding overheats, sibling T-04-B at-risk |
| `inverter_cooling_fault`            | Single inverter (INV-08-02-03) cooling fan failure → derate → trip          |
| `tracker_stow_misalignment`         | Tracker TRK-12 stuck during dust event → block-12 PR drop                    |
| `string_pid_degradation`            | Strings 1–12 on INV-10-03-01 lose current → MPPT chase                      |

Each profile runs for 35 minutes with a 2-minute ramp-up and 5-minute
ramp-down. Within the first 2–3 minutes infrastructure-layer metrics
degrade; block-level symptoms appear 5–7 minutes after activation;
PPA dispatch deviation crosses threshold ~11 minutes in. The
`mv_transformer_winding_overheat` profile also bumps the at-risk
sibling transformer + offtaker (`guvnl_state`) starting ~13–16 minutes
in, at a lower amplitude per spec §22.

## Tear-down

```bash
kubectl delete -k k8s/overlays/adani-prod
```

## Environment knobs

| Env var                         | Default                  | Purpose                                                 |
| ------------------------------- | ------------------------ | ------------------------------------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT`   | `http://$(HOST_IP):4318` | Node-local OTel collector (set automatically)           |
| `OTEL_INSECURE`                 | `true`                   | Don't TLS-verify the loopback hop to the node agent     |
| `SOLAR_FAULT_PORT`              | `9999`                   | Port the local fault HTTP server binds                  |
| `SOLAR_LOG_INTERVAL_SEC`        | `5`                      | Seconds between log-emission ticks                      |
| `SOLAR_EMIT_PER_INVERTER`       | `true`                   | Set `false` to drop the per-inverter family (96×13 series) |
