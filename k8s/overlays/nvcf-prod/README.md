# NVCF GPU-inference simulator — prod deploy

This overlay deploys the NVCF telemetry simulator (`services/nvcf`) as a
single pod in the `nvcf-demo` namespace of the lakerunner prod cluster
(`aws-prod-us-east-2-global`). Telemetry flows via the node-local OTel
collector DaemonSet (`collector/aws-prod-us-east-2-global-agent`) at
`http://$(HOST_IP):4318` — same pattern the existing `airtel-demo` and
`adani-demo` namespaces use. No Cardinal API key is provisioned at the
pod; the collector handles upstream routing to the Cardinal demo org.

The pod exposes an HTTP API on port `9998` so the demo operator can
activate any of the 11 chaos knobs via `kubectl port-forward`.

See `docs/specs/nvcf.md` for the full scenario design — knobs, dashboards,
and Cardinal-detection playbook.

## Prerequisites

- A built image of `griffin-commerce-demo` containing the NVCF simulator
  changes (the `services/nvcf` package) — pin its tag in
  `kustomization.yaml` (`images.newTag`). At time of writing the M1
  scaffold targets `v0.5.1`.
- The node-local OTel collector DaemonSet
  (`collector/aws-prod-us-east-2-global-agent`) already running, which
  it is in `aws-prod-us-east-2-global`. Nothing to install here.

## Operator runbook

1. Apply the overlay:

   ```bash
   kubectl apply -k k8s/overlays/nvcf-prod
   ```

   This creates the `nvcf-demo` namespace, the `nvcf` Deployment, and
   the `nvcf-faults` Service (ClusterIP, port 9998).

2. Verify the pod is healthy:

   ```bash
   kubectl -n nvcf-demo get pods -l app=nvcf
   kubectl -n nvcf-demo logs -l app=nvcf --tail=50
   ```

   You should see `NVCF simulator running. Waiting for shutdown signal.`
   once startup completes, with a one-line catalog summary
   (`functions=4 versions=6 accounts=4 clusters=2 instances=24
   inference_servers=120`).

3. At baseline you should immediately see metrics with
   `scenario_id=nvcf_gpu_inference_simulator` in the Cardinal demo org
   (same org as `airtel-demo` and `adani-demo`). M1 emits:

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
