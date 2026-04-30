# Kubernetes Manifests

Deploy the demo to any conformant Kubernetes cluster (kind, k3s, EKS, GKE,
AKS, etc.) with `kubectl` + `kustomize`.

## Prerequisites

- `kubectl` v1.21+ (built-in `kustomize` is fine; standalone `kustomize` v4+
  also works).
- An ingress controller installed in the cluster (nginx, Traefik, HAProxy,
  cloud LB, ...). The base manifests don't pin a class so the cluster's
  default controller picks the Ingress up.
- Outbound network access to `ghcr.io` from the cluster's nodes.

## Quick start

```sh
kubectl apply -k k8s/base
```

This creates the `griffin-commerce` namespace and deploys the seven services
(frontend + six backends) plus an Ingress that routes `/` to the frontend.
All images are pinned to a public GHCR release tag, so the apply is
reproducible.

Watch the rollout:

```sh
kubectl -n griffin-commerce rollout status deploy/frontend
kubectl -n griffin-commerce get pods
```

## Reaching the frontend

The base Ingress accepts any hostname and uses the cluster's default ingress
class. To restrict it to a specific host, edit `k8s/base/ingress.yaml` and
uncomment / set:

```yaml
spec:
  ingressClassName: nginx           # or your controller's class
  rules:
  - host: griffin.example.com
    http: ...
```

Then point that hostname at the cluster's ingress LB / node IP.

## Switching versions

`k8s/base/kustomization.yaml` pins the image tag in one place:

```yaml
images:
  - name: ghcr.io/cardinalhq/griffin-commerce-demo
    newTag: v0.1.9
```

Bump `newTag` and re-apply to roll forward. The two example overlays show the
common cases:

- `k8s/overlays/dev` — tracks `latest`. Re-apply or `kubectl rollout restart`
  to pull the newest build.
- `k8s/overlays/prod` — pinned to a specific release tag for reproducibility.

```sh
kubectl apply -k k8s/overlays/dev
kubectl apply -k k8s/overlays/prod
```

## Enabling OpenTelemetry export

Telemetry is **off by default**: with no OTLP env vars set, the services log
to stdout only and emit no exporter traffic. To export traces, metrics, and
logs to a collector, use the `with-otlp` overlay:

```sh
kubectl apply -k k8s/overlays/with-otlp
```

The overlay assumes a per-node collector reachable at `:4318/HTTP` on each
Node IP (the typical DaemonSet pattern). If your collector is somewhere else
— for example a cluster Service — edit `OTEL_EXPORTER_OTLP_ENDPOINT` in
`k8s/overlays/with-otlp/kustomization.yaml` to point at it (e.g.
`http://otel-collector.observability:4318`).

## Notes

- Every Deployment runs `replicas: 1`. This is a demo, not an HA reference;
  scale up if you actually want resilience.
- The namespace enforces the `restricted` Pod Security Standard. The
  manifests already comply (non-root UID 65532, dropped capabilities, no
  privilege escalation, RuntimeDefault seccomp profile).
- Backend services expose `/health` and have liveness + readiness probes
  wired to it.
