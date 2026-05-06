// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 CardinalHQ, Inc.

import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// Production configuration for containerized deployment
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  preview: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      // In production containers, services are accessible by their container names
      '/api/products': {
        target: 'http://catalog:8080',
        changeOrigin: true
      },
      '/api/payments': {
        target: 'http://payment:8081',
        changeOrigin: true
      },
      '/api/cart': {
        target: 'http://cart:8082',
        changeOrigin: true,
        proxyTimeout: 60_000,
        timeout: 60_000
      },
      '/api/images': {
        target: 'http://images:8083',
        changeOrigin: true
      },
      '/static': {
        target: 'http://images:8083',
        changeOrigin: true
      },
      '/api/shipping': {
        target: 'http://shipping:8084',
        changeOrigin: true
      },
      '/api/recommendations': {
        target: 'http://recommendations:8085',
        changeOrigin: true
      },
      '/admin/faults': {
        target: 'http://controlplane:8086',
        changeOrigin: true,
        proxyTimeout: 60_000,
        timeout: 60_000
      }
    }
  }
})