# NVCF GPU-inference simulator — prod deploy

This overlay deploys the NVCF telemetry simulator (`services/nvcf`) as a
single pod in the `nvcf-demo` namespace of the lakerunner prod cluster
(`aws-prod-us-east-2-global`). Telemetry flows **directly to the Cardinal
SaaS intake** (`https://otelhttp.intake.us-east-2.aws.cardinalhq.io`)
authenticated with the **"Cardinal HQ - Demo" org's "Demo Apps" ingest
key**, so segments land in `6d69ff5f-d386-491e-a715-306a8f172b53` — the
org the NVCF dashboards live in.

This is **different** from the `airtel-demo` / `adani-demo` overlays,
which route via the node-local agent DaemonSet. That path tags segments
to the parent Cardinal HQ org (`c4375e34-...`), not Cardinal HQ - Demo,
which is why the first version of this overlay produced data nothing in
the demo org could see.

The pod exposes an HTTP API on port `9998` so the demo operator can
activate any of the 11 chaos knobs via `kubectl port-forward`.

See `docs/specs/nvcf.md` for the full scenario design — knobs, dashboards,
and Cardinal-detection playbook.

## Prerequisites

- A built image of `griffin-commerce-demo` containing the NVCF simulator
  changes (the `services/nvcf` package) — pin its tag in
  `kustomization.yaml` (`images.newTag`). At time of writing the M1
  scaffold targets `v0.5.1`.
- A Cardinal HQ - Demo "Demo Apps" ingest key. Plaintext lives in
  `maestro.maestro_ingest_api_keys` for
  `org_id=6d69ff5f-... name='Demo Apps'`. Pull it with:

  ```bash
  PASS=$(kubectl -n maestro get secret pg-credentials -o jsonpath='{.data.MAESTRO_DB_PASSWORD}' | base64 -d)
  kubectl -n cnpg-cardinal port-forward svc/cnpg-cardinal-rw 15432:5432 &
  PGPASSWORD="$PASS" psql -h localhost -p 15432 -U maestro -d maestro -tA -c \
    "SELECT api_key FROM maestro_ingest_api_keys
     WHERE org_id='6d69ff5f-d386-491e-a715-306a8f172b53' AND name='Demo Apps' AND revoked_at IS NULL;"
  ```

## Operator runbook

1. Create the `nvcf-demo` namespace and the ingest-key Secret:

   ```bash
   kubectl create namespace nvcf-demo
   kubectl -n nvcf-demo create secret generic nvcf-cardinal-ingest \
     --from-literal=OTEL_EXPORTER_OTLP_HEADERS="x-cardinalhq-api-key=<demo-apps-key>"
   ```

   The Secret is **not** kustomize-managed: it carries an org credential
   and shouldn't sit in git. Apply once per cluster; rotate by recreating.

2. Apply the overlay:

   ```bash
   kubectl apply -k k8s/overlays/nvcf-prod
   ```

   This creates (or no-ops if present) the `nvcf-demo` namespace, the
   `nvcf` Deployment, and the `nvcf-faults` Service (ClusterIP,
   port 9998). The Deployment pulls the OTLP header from the
   `nvcf-cardinal-ingest` Secret you just created.

3. Verify the pod is healthy:

   ```bash
   kubectl -n nvcf-demo get pods -l app=nvcf
   kubectl -n nvcf-demo logs -l app=nvcf --tail=50
   ```

   You should see `NVCF simulator running. Waiting for shutdown signal.`
   once startup completes, with a one-line catalog summary
   (`functions=4 versions=6 accounts=4 clusters=2 instances=24
   inference_servers=120`).

4. At baseline you should immediately see metrics with
   `scenario_id=nvcf_gpu_inference_simulator` in the **Cardinal HQ -
   Demo** org. M1 emits:

   - `function_request_total` — cumulative requests per
     `(function_id, function_version_id, account_name)`
   - `function_request_latency` — p95 latency per function version
   - `stargate_client_request_time_to_first_token_seconds` — TTFT per
     inference server
   - `stargate_client_model_output_tps`,
     `stargate_client_model_kv_cache_used_tokens`,
     `stargate_client_model_kv_cache_capacity_tokens`,
     `stargate_client_requests_inflight` — per-model stargate signals
   - `nvcf_function_queue_depth` — per
     `(account_name, function_id, function_version_id)`
   - `nvcf_autoscaler.scaling.current_instances`,
     `nvcf_autoscaler.scaling.desired_instances` — per function version
   - `nvca_container_crash_total` — per cluster
   - `DCGM_FI_DEV_GPU_UTIL` — per GPU device

   All metric names and label vocabulary are verbatim NVCF native — the
   same dashboard queries work against a real NVCF cluster.

## Activating a chaos knob

Port-forward the fault HTTP server:

```bash
kubectl -n nvcf-demo port-forward svc/nvcf-faults 9998:9998 &
```

List available knobs:

```bash
curl -sS http://localhost:9998/faults/profiles
```

Activate the `function.ttft-regression` knob targeting `summarize-doc`
(the only knob with a full M1 impact spec; the remaining 10 activate
cleanly but don't yet bend metrics):

```bash
curl -sS -X POST \
  "http://localhost:9998/faults/activate?profile=function.ttft-regression&function=summarize-doc"
```

Check status:

```bash
curl -sS http://localhost:9998/faults/status
```

Clear:

```bash
curl -sS -X POST http://localhost:9998/faults/clear
```

Knobs auto-clear after 5 minutes (the `defaultDuration` in
`services/nvcf/scenario.go`).
