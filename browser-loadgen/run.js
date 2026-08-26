// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.
//
// Playwright-driven RUM traffic generator. Spawns N concurrent "user"
// loops against a Griffin frontend. Each loop opens a fresh browser
// context (a fresh RUM session.id), runs a short randomised journey, and
// closes; then pauses and loops again. The whole point is to keep real
// browser telemetry (document loads, fetch spans, user-interaction
// spans, web-vitals) flowing continuously so the RUM demo has a live
// signal without a human at a keyboard.

import { chromium } from 'playwright'

const TARGET_URL = process.env.TARGET_URL ?? 'http://frontend:5173'
const CONCURRENCY = clampInt(process.env.CONCURRENCY, 1, 32, 2)
const PAUSE_MIN_MS = clampInt(process.env.PAUSE_MIN_MS, 100, 60_000, 1_000)
const PAUSE_MAX_MS = clampInt(process.env.PAUSE_MAX_MS, PAUSE_MIN_MS, 300_000, 5_000)
const JOURNEY_TIMEOUT_MS = clampInt(process.env.JOURNEY_TIMEOUT_MS, 5_000, 300_000, 60_000)
const NAV_TIMEOUT_MS = clampInt(process.env.NAV_TIMEOUT_MS, 5_000, 120_000, 30_000)
const HEADLESS = (process.env.HEADLESS ?? 'true').toLowerCase() !== 'false'
const USER_AGENT_SUFFIX = process.env.USER_AGENT_SUFFIX ?? 'GriffinBrowserLoadgen'
const CHAOS_ROUTE_PROBABILITY = clampFloat(process.env.CHAOS_ROUTE_PROBABILITY, 0, 1, 0.05)

function clampInt(raw, min, max, fallback) {
  const n = raw == null ? NaN : Number.parseInt(raw, 10)
  if (!Number.isFinite(n)) return fallback
  return Math.max(min, Math.min(max, n))
}
function clampFloat(raw, min, max, fallback) {
  const n = raw == null ? NaN : Number.parseFloat(raw)
  if (!Number.isFinite(n)) return fallback
  return Math.max(min, Math.min(max, n))
}
function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms))
}
function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)]
}
function jitter() {
  return PAUSE_MIN_MS + Math.random() * (PAUSE_MAX_MS - PAUSE_MIN_MS)
}
function ts() {
  return new Date().toISOString()
}
function log(msg) {
  process.stdout.write(`${ts()} ${msg}\n`)
}

// Journey primitives. Each returns a promise so we can chain them with
// per-step timeouts and never wedge on a broken selector.
async function browseHome(page) {
  await page.goto(TARGET_URL + '/', { waitUntil: 'domcontentloaded', timeout: NAV_TIMEOUT_MS })
  // Give the SPA a beat to fire off its /api/products fetch and render.
  await page.waitForLoadState('networkidle', { timeout: NAV_TIMEOUT_MS }).catch(() => {})
  await sleep(jitter())
}
async function scrollAround(page) {
  const height = await page.evaluate(() => document.body.scrollHeight).catch(() => 0)
  if (!height) return
  const steps = 2 + Math.floor(Math.random() * 3)
  for (let i = 0; i < steps; i++) {
    const y = Math.floor((Math.random() * height) / 1.2)
    await page.evaluate((y) => window.scrollTo({ top: y, behavior: 'smooth' }), y)
    await sleep(300 + Math.random() * 700)
  }
}
async function openRandomProduct(page) {
  const buttons = await page.getByRole('button', { name: /view details|details/i }).all().catch(() => [])
  const cards = buttons.length ? buttons : await page.locator('[data-product-id], .product-card, article').all().catch(() => [])
  if (!cards.length) return
  const target = pick(cards)
  await target.click({ timeout: 5_000 }).catch(() => {})
  await sleep(jitter())
}
async function addSomethingToCart(page) {
  const btn = page.getByRole('button', { name: /add to cart/i }).first()
  await btn.click({ timeout: 5_000 }).catch(() => {})
  await sleep(jitter())
}
async function visitChaos(page) {
  await page.goto(TARGET_URL + '/chaos', { waitUntil: 'domcontentloaded', timeout: NAV_TIMEOUT_MS }).catch(() => {})
  await sleep(jitter())
}

async function runJourney(browser, workerId, iteration) {
  const context = await browser.newContext({
    userAgent: `Mozilla/5.0 (compatible; ${USER_AGENT_SUFFIX}/1.0; worker-${workerId})`,
    viewport: { width: 1280, height: 800 },
  })
  const page = await context.newPage()
  page.on('pageerror', (err) => log(`w${workerId} pageerror: ${err.message}`))
  page.on('console', (msg) => {
    if (msg.type() === 'error') log(`w${workerId} console.error: ${msg.text()}`)
  })
  const start = Date.now()
  try {
    await Promise.race([
      (async () => {
        await browseHome(page)
        await scrollAround(page)
        await openRandomProduct(page)
        await addSomethingToCart(page)
        if (Math.random() < CHAOS_ROUTE_PROBABILITY) await visitChaos(page)
        // Sit on the page long enough for the SDK to batch-flush.
        await sleep(6_000 + Math.random() * 4_000)
      })(),
      sleep(JOURNEY_TIMEOUT_MS).then(() => {
        throw new Error(`journey timeout after ${JOURNEY_TIMEOUT_MS}ms`)
      }),
    ])
    log(`w${workerId} i${iteration} ok (${Date.now() - start}ms)`)
  } catch (err) {
    log(`w${workerId} i${iteration} fail: ${(err && err.message) || err}`)
  } finally {
    await context.close().catch(() => {})
  }
}

async function workerLoop(browser, workerId) {
  let iteration = 0
  // Stagger workers so they don't hammer the frontend in lockstep.
  await sleep(Math.random() * PAUSE_MAX_MS)
  while (true) {
    iteration++
    await runJourney(browser, workerId, iteration)
    await sleep(jitter())
  }
}

async function main() {
  log(`target=${TARGET_URL} concurrency=${CONCURRENCY} pause=${PAUSE_MIN_MS}-${PAUSE_MAX_MS}ms headless=${HEADLESS}`)
  const browser = await chromium.launch({
    headless: HEADLESS,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })

  const shutdown = async (sig) => {
    log(`shutdown on ${sig}`)
    await browser.close().catch(() => {})
    process.exit(0)
  }
  process.on('SIGINT', () => shutdown('SIGINT'))
  process.on('SIGTERM', () => shutdown('SIGTERM'))

  const workers = Array.from({ length: CONCURRENCY }, (_, i) => workerLoop(browser, i + 1))
  await Promise.allSettled(workers)
}

main().catch((err) => {
  log(`fatal: ${err && err.stack ? err.stack : err}`)
  process.exit(1)
})
