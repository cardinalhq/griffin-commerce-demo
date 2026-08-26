// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

import { initTelemetry } from './lib/telemetry'
import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'

// Init before mount so document-load spans and the earliest fetch spans
// see the tracer. No-op unless /rum-config.js set window.__RUM_CONFIG__.
initTelemetry()

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
