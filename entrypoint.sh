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
    envsubst '${PORT}' < /app/nginx.conf.template > /tmp/nginx.conf
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
  loadgen)
    # Continuous low-rate traffic against the cart service so the
    # customer-persona side of the Airtel demo has live traces to
    # investigate. No HTTP server — outbound client only.
    /app/bin/griffin loadgen
    ;;
  *)
    echo "Unknown service: $SERVICE_NAME"
    echo "Valid services: frontend, catalog, payment, cart, images, shipping, recommendations, controlplane, dbaas"
    exit 1
    ;;
esac