# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 CardinalHQ, Inc.
#
# RUM traffic generator. Wraps a headless Chromium (installed by
# Playwright at build time into /ms-playwright) around a tiny Node.js
# loop that navigates the Griffin frontend continuously.
# The point of shipping *real browser* traffic — vs. Locust hitting REST
# endpoints — is that OTel-web only fires when a real browser loads the
# SPA and executes its script.
#
# Based on the official Node image rather than mcr.microsoft.com/playwright
# so the Node version is pinned here and picks up security releases
# without waiting for a new Playwright image.

FROM node:24.22.1-bookworm-slim

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

# The base image ships a non-root `node` user (UID 1000) so Chromium can
# run under runAsNonRoot=true. Our k8s base kustomization stamps that
# pod-level securityContext onto every Deployment.
#
# WORKDIR creates its target dir as root:root by default — a subsequent
# `USER 1000:1000` then can't `npm ci` because it can't create
# node_modules under a root-owned parent. Pre-create + chown fixes it.
RUN mkdir -p /home/node/app && chown node:node /home/node/app
WORKDIR /home/node/app
USER 1000:1000

COPY --chown=node:node package.json package-lock.json ./
RUN npm ci --omit=dev --no-fund --no-audit \
  && npm cache clean --force

# Chromium plus its system libraries, at the build the locked playwright
# version expects. --with-deps uses apt, so this step runs as root.
USER root
RUN ./node_modules/.bin/playwright install --with-deps chromium \
  && rm -rf /var/lib/apt/lists/*
USER 1000:1000

COPY --chown=node:node run.js ./

# Sensible demo defaults; overridden per-environment via compose / k8s env.
ENV TARGET_URL=http://frontend:5173
ENV CONCURRENCY=2
ENV PAUSE_MIN_MS=1000
ENV PAUSE_MAX_MS=5000
ENV HEADLESS=true

CMD ["node", "run.js"]
