// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.
//
// Browser RUM: OTel-web tracer + logger + web-vitals, wired to whatever
// OTLP-HTTP endpoint the runtime template stamped into window.__RUM_CONFIG__.
// The nginx sidecar renders /rum-config.js at container start; a build with
// no runtime config (npm run dev, `serve` on the dist) becomes a no-op.

import { trace, diag, DiagConsoleLogger, DiagLogLevel } from '@opentelemetry/api'
import { logs, SeverityNumber } from '@opentelemetry/api-logs'
import { getWebAutoInstrumentations } from '@opentelemetry/auto-instrumentations-web'
import { ZoneContextManager } from '@opentelemetry/context-zone'
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-http'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http'
import { registerInstrumentations } from '@opentelemetry/instrumentation'
import { Resource } from '@opentelemetry/resources'
import { BatchLogRecordProcessor, LoggerProvider } from '@opentelemetry/sdk-logs'
import { BatchSpanProcessor, WebTracerProvider } from '@opentelemetry/sdk-trace-web'
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from '@opentelemetry/semantic-conventions'
import { onCLS, onFCP, onINP, onLCP, onTTFB, type Metric } from 'web-vitals'

export interface RumConfig {
  enabled: boolean
  // Base path for OTLP-HTTP. The exporter appends /traces and /logs.
  // Same-origin ('/v1') avoids CORS; nginx proxies to the collector.
  endpoint: string
  serviceName: string
  serviceNamespace?: string
  serviceVersion?: string
  // Extra resource attributes stamped on every signal. Mirror the backend
  // OTEL_RESOURCE_ATTRIBUTES so RUM correlates cleanly with API traces.
  resourceAttributes?: Record<string, string>
  // Regex sources (strings) — URLs whose fetch/XHR gets a traceparent
  // header. Same-origin requests always get one; use this for cross-origin
  // APIs that also accept W3C trace-context.
  propagateHeaderCorsUrls?: string[]
  // 'diag' console noise level; leave off in prod.
  debug?: boolean
}

declare global {
  interface Window {
    __RUM_CONFIG__?: RumConfig | null
  }
}

let initialized = false

export function initTelemetry(): void {
  if (initialized) return
  const cfg = window.__RUM_CONFIG__
  if (!cfg || !cfg.enabled) return
  initialized = true

  if (cfg.debug) {
    diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.DEBUG)
  }

  const sessionId = crypto.randomUUID()
  const baseAttrs: Record<string, string | number | boolean> = {
    [ATTR_SERVICE_NAME]: cfg.serviceName,
    'session.id': sessionId,
    'browser.language': navigator.language,
    'browser.user_agent': navigator.userAgent,
    'page.url': window.location.href,
  }
  if (cfg.serviceNamespace) baseAttrs['service.namespace'] = cfg.serviceNamespace
  if (cfg.serviceVersion) baseAttrs[ATTR_SERVICE_VERSION] = cfg.serviceVersion
  for (const [k, v] of Object.entries(cfg.resourceAttributes ?? {})) {
    baseAttrs[k] = v
  }

  const resource = new Resource(baseAttrs)

  const traceUrl = joinUrl(cfg.endpoint, 'traces')
  const logsUrl = joinUrl(cfg.endpoint, 'logs')

  const traceProvider = new WebTracerProvider({
    resource,
    spanProcessors: [
      new BatchSpanProcessor(new OTLPTraceExporter({ url: traceUrl }), {
        maxExportBatchSize: 20,
        scheduledDelayMillis: 5_000,
      }),
    ],
  })
  traceProvider.register({ contextManager: new ZoneContextManager() })

  const propagateUrls = (cfg.propagateHeaderCorsUrls ?? []).map(
    (source) => new RegExp(source),
  )

  registerInstrumentations({
    instrumentations: getWebAutoInstrumentations({
      '@opentelemetry/instrumentation-fetch': {
        propagateTraceHeaderCorsUrls: propagateUrls,
        clearTimingResources: true,
      },
      '@opentelemetry/instrumentation-xml-http-request': {
        propagateTraceHeaderCorsUrls: propagateUrls,
      },
      '@opentelemetry/instrumentation-user-interaction': {
        eventNames: ['click', 'submit'],
      },
      '@opentelemetry/instrumentation-document-load': {},
    }),
  })

  const loggerProvider = new LoggerProvider({ resource })
  loggerProvider.addLogRecordProcessor(
    new BatchLogRecordProcessor(new OTLPLogExporter({ url: logsUrl }), {
      maxExportBatchSize: 20,
      scheduledDelayMillis: 5_000,
    }),
  )
  logs.setGlobalLoggerProvider(loggerProvider)
  const logger = logs.getLogger('griffin.frontend')

  const emitLog = (
    severity: SeverityNumber,
    severityText: string,
    body: string,
    attrs: Record<string, string | number | boolean> = {},
  ) => {
    try {
      logger.emit({
        severityNumber: severity,
        severityText,
        body,
        attributes: attrs,
      })
    } catch {
      // never let telemetry break the app
    }
  }

  // console.error → OTel log. Preserve original so devtools still show it.
  const origError = console.error.bind(console)
  console.error = (...args: unknown[]) => {
    emitLog(SeverityNumber.ERROR, 'ERROR', args.map(fmt).join(' '))
    origError(...args)
  }

  window.addEventListener('error', (ev) => {
    emitLog(SeverityNumber.ERROR, 'ERROR', ev.message, {
      'exception.type': ev.error?.name ?? 'Error',
      'exception.stacktrace': ev.error?.stack ?? '',
      'source': ev.filename ?? '',
      'line': ev.lineno ?? 0,
      'column': ev.colno ?? 0,
    })
  })
  window.addEventListener('unhandledrejection', (ev) => {
    emitLog(SeverityNumber.ERROR, 'ERROR', `unhandled rejection: ${fmt(ev.reason)}`)
  })

  // Web vitals — LCP/CLS/INP/FCP/TTFB as short standalone spans so they
  // show up in the trace stream without a metrics pipeline. Rating (good /
  // needs-improvement / poor) stamps as an attribute for dashboarding.
  const vitalsTracer = trace.getTracer('web-vitals')
  const emitVital = (name: string) => (metric: Metric) => {
    const span = vitalsTracer.startSpan(`web-vital.${name}`)
    span.setAttribute('metric.name', name)
    span.setAttribute('metric.value', metric.value)
    span.setAttribute('metric.rating', metric.rating)
    span.setAttribute('metric.id', metric.id)
    span.setAttribute('page.url', window.location.href)
    span.end()
  }
  onLCP(emitVital('LCP'))
  onCLS(emitVital('CLS'))
  onINP(emitVital('INP'))
  onFCP(emitVital('FCP'))
  onTTFB(emitVital('TTFB'))

  // Best-effort flush on unload. sendBeacon is the transport the exporter
  // uses in these cases when available.
  window.addEventListener('pagehide', () => {
    void traceProvider.forceFlush()
    void loggerProvider.forceFlush()
  })
}

function joinUrl(base: string, path: string): string {
  const trimmed = base.endsWith('/') ? base.slice(0, -1) : base
  return `${trimmed}/${path}`
}

function fmt(v: unknown): string {
  if (typeof v === 'string') return v
  if (v instanceof Error) return v.stack ?? v.message
  try {
    return JSON.stringify(v)
  } catch {
    return String(v)
  }
}
