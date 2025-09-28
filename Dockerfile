# Dockerfile for GoReleaser
# GoReleaser will provide the pre-built binaries
FROM alpine:3.19
ARG TARGETPLATFORM

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    bash

RUN env

COPY $TARGETPLATFORM/griffin /app/bin/

# Copy static assets for image service
COPY services/images/static /app/services/images/static

# Copy entrypoint
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/bin/* /app/entrypoint.sh

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:${PORT:-5173}/health || exit 1

EXPOSE 5173

ENTRYPOINT ["/app/entrypoint.sh"]
