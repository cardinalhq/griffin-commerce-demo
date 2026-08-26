# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 CardinalHQ, Inc.
#
# RUM traffic generator. Wraps a headless Chromium (from the Playwright
# base image, which ships browsers preinstalled at /ms-playwright) around
# a tiny Node.js loop that navigates the Griffin frontend continuously.
# The point of shipping *real browser* traffic — vs. Locust hitting REST
# endpoints — is that OTel-web only fires when a real browser loads the
# SPA and executes its script.

FROM mcr.microsoft.com/playwright:v1.47.0-jammy

# The base image ships a non-root `pwuser` (UID 1000) precisely so
# Chromium can run under runAsNonRoot=true. Our k8s base kustomization
# stamps that pod-level securityContext onto every Deployment.
#
# WORKDIR creates its target dir as root:root by default — a subsequent
# `USER 1000:1000` then can't `npm install` because it can't create
# node_modules under a root-owned parent. Pre-create + chown fixes it.
RUN mkdir -p /home/pwuser/app && chown pwuser:pwuser /home/pwuser/app
WORKDIR /home/pwuser/app
USER 1000:1000

COPY --chown=pwuser:pwuser package.json ./
# npm ci needs a lockfile; the loadgen is one dep and rarely churns, so
# `npm install --omit=dev` on package.json is cheaper than committing one.
RUN npm install --omit=dev --no-fund --no-audit \
  && npm cache clean --force

COPY --chown=pwuser:pwuser run.js ./

# Match the base image's default browser install location.
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

# Sensible demo defaults; overridden per-environment via compose / k8s env.
ENV TARGET_URL=http://frontend:5173
ENV CONCURRENCY=2
ENV PAUSE_MIN_MS=1000
ENV PAUSE_MAX_MS=5000
ENV HEADLESS=true

CMD ["node", "run.js"]
