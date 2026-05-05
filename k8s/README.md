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
logs, pick the overlay that matches how your collector is reachable.

### Per-node collector (DaemonSet + hostPort)

```sh
kubectl apply -k k8s/overlays/with-otlp-nodelocal
```

Each backend Pod resolves its endpoint to the Node IP it was scheduled on
(`status.hostIP`) on port `4318/HTTP`. This is the classic Kubernetes pattern
where a collector DaemonSet exposes a hostPort on every node and Pods short-
circuit to their local node, avoiding cross-node hops.

### Single endpoint URL

```sh
kubectl apply -k k8s/overlays/with-otlp-url
```

All backends export to the same URL (a Service, Route, Ingress, or external
endpoint). Edit `k8s/overlays/with-otlp-url/kustomization.yaml` and replace
the placeholder URL — the default is
`http://otel-collector.observability.svc.cluster.local:4318` — with whatever
your collector is reachable at. Drop `OTEL_INSECURE: "true"` from each patch
if the endpoint speaks HTTPS.

This is the recommended overlay on **OpenShift**, where SCCs typically
restrict hostPort and pod-to-Node-IP traffic, making the node-local pattern
awkward; routing everything to a Service or Route URL stays inside the
sanctioned pod network.

## Deploying on OpenShift

The base manifests are restricted-PSA-clean (non-root, dropped caps,
RuntimeDefault seccomp) and don't pin a specific UID, so OpenShift's
default `restricted-v2` SCC will inject a UID from the namespace's
auto-assigned range. No SCC rolebinding is required:

```sh
kubectl apply -k k8s/overlays/with-otlp-url   # or k8s/base, etc.
```

The image already runs cleanly as an arbitrary UID (nginx writes its
PID and temp files under `/tmp`, logs to stdout/stderr, and binds to
an unprivileged port).

### Browser access via a Route

The bundled `Ingress` is generic and won't auto-promote to a Route on
OpenShift (it has no `host:` and OCP's ingress-to-route translator logs
`IncompleteIngressToRouteRules`). Create a Route directly instead — fill
in the wildcard apps domain for your cluster (find it with `kubectl get
ingresscontroller -n openshift-ingress-operator default -o jsonpath='{.status.domain}'`):

```sh
cat <<EOF | kubectl apply -f -
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: griffin-commerce
  namespace: griffin-commerce
spec:
  host: griffin.apps.YOUR-CLUSTER-DOMAIN
  to:
    kind: Service
    name: frontend
    weight: 100
  port:
    targetPort: 5173
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
EOF
```

Then point a browser at `https://griffin.apps.YOUR-CLUSTER-DOMAIN/`.

## Notes

- Every Deployment runs `replicas: 1`. This is a demo, not an HA reference;
  scale up if you actually want resilience.
- The namespace enforces the `restricted` Pod Security Standard. The
  manifests already comply (non-root UID 65532, dropped capabilities, no
  privilege escalation, RuntimeDefault seccomp profile).
- Backend services expose `/health` and have liveness + readiness probes
  wired to it.
