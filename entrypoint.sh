#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 CardinalHQ, Inc.

set -e

# Default to frontend if no service specified
SERVICE_NAME=${SERVICE_NAME:-frontend}

echo "Starting service: $SERVICE_NAME"

case "$SERVICE_NAME" in
  frontend)
    export PORT=${PORT:-5173}
    # OTLP-HTTP receiver the browser SDK proxies to via /v1/* nginx
    # locations. Defaults leave RUM disabled; the demo overlay flips
    # RUM_ENABLED=true and points OTEL_COLLECTOR_HOST at the per-node
    # agent (via HOST_IP downward API) or an in-cluster Service.
    export OTEL_COLLECTOR_HOST=${OTEL_COLLECTOR_HOST:-127.0.0.1}
    export OTEL_COLLECTOR_PORT=${OTEL_COLLECTOR_PORT:-4318}
    envsubst '${PORT} ${OTEL_COLLECTOR_HOST} ${OTEL_COLLECTOR_PORT}' \
      < /app/nginx.conf.template > /tmp/nginx.conf

    # Render browser RUM config at boot. The SDK reads window.__RUM_CONFIG__;
    # when RUM_ENABLED=false it treats the load as a no-op.
    export RUM_ENABLED=${RUM_ENABLED:-false}
    export RUM_OTLP_PATH=${RUM_OTLP_PATH:-/v1}
    export RUM_SERVICE_NAME=${RUM_SERVICE_NAME:-griffin-frontend}
    export RUM_SERVICE_NAMESPACE=${RUM_SERVICE_NAMESPACE:-}
    export RUM_SERVICE_VERSION=${RUM_SERVICE_VERSION:-}
    # JSON blobs so nginx can substitute them verbatim without shell
    # quoting hazards. Defaults: no extra attrs, no cross-origin URLs.
    export RUM_RESOURCE_ATTRIBUTES_JSON=${RUM_RESOURCE_ATTRIBUTES_JSON:-'{}'}
    export RUM_PROPAGATE_HEADER_CORS_URLS_JSON=${RUM_PROPAGATE_HEADER_CORS_URLS_JSON:-'[]'}
    export RUM_DEBUG=${RUM_DEBUG:-false}
    envsubst '${RUM_ENABLED} ${RUM_OTLP_PATH} ${RUM_SERVICE_NAME} ${RUM_SERVICE_NAMESPACE} ${RUM_SERVICE_VERSION} ${RUM_RESOURCE_ATTRIBUTES_JSON} ${RUM_PROPAGATE_HEADER_CORS_URLS_JSON} ${RUM_DEBUG}' \
      < /app/rum-config.js.template > /tmp/rum-config.js

    exec nginx -c /tmp/nginx.conf -g 'daemon off;'
    ;;
  catalog)
    PORT=${PORT:-8080} /app/bin/griffin catalog
    ;;
  payment)
    PORT=${PORT:-8081} /app/bin/griffin payment
    ;;
  cart)
    PORT=${PORT:-8082} /app/bin/griffin cart
    ;;
  images)
    PORT=${PORT:-8083} /app/bin/griffin images
    ;;
  shipping)
    PORT=${PORT:-8084} /app/bin/griffin shipping
    ;;
  recommendations)
    PORT=${PORT:-8085} /app/bin/griffin recommendations
    ;;
  controlplane)
    PORT=${PORT:-8086} /app/bin/griffin controlplane
    ;;
  dbaas)
    # DBaaS simulator for the Airtel demo. No HTTP server — it's a pure
    # OTLP metric emitter that polls the controlplane for fault knobs.
    /app/bin/griffin dbaas
    ;;
  nvcf)
    # NVIDIA Cloud Functions simulator. Pure OTLP metric emitter with a
    # local :9998 fault HTTP server reachable via kubectl port-forward.
    # See docs/specs/nvcf.md.
    /app/bin/griffin nvcf
    ;;
  loadgen)
    # Continuous low-rate traffic against the cart service so the
    # customer-persona side of the Airtel demo has live traces to
    # investigate. No HTTP server — outbound client only.
    /app/bin/griffin loadgen
    ;;
  *)
    echo "Unknown service: $SERVICE_NAME"
    echo "Valid services: frontend, catalog, payment, cart, images, shipping, recommendations, controlplane, dbaas, nvcf"
    exit 1
    ;;
esac