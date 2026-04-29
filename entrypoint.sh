#!/bin/bash
set -e

# Default to frontend if no service specified
SERVICE_NAME=${SERVICE_NAME:-frontend}

echo "Starting service: $SERVICE_NAME"

case "$SERVICE_NAME" in
  frontend)
    export PORT=${PORT:-5173}
    envsubst '${PORT}' < /app/nginx.conf.template > /etc/nginx/nginx.conf
    exec nginx -g 'daemon off;'
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
  *)
    echo "Unknown service: $SERVICE_NAME"
    echo "Valid services: frontend, catalog, payment, cart, images, shipping, recommendations"
    exit 1
    ;;
esac