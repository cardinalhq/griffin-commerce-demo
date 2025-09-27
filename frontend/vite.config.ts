import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  server: {
    proxy: {
      // Proxy catalog service
      '/api/products': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      // Proxy payment service
      '/api/payments': {
        target: 'http://localhost:8081',
        changeOrigin: true
      },
      // Proxy cart service
      '/api/cart': {
        target: 'http://localhost:8082',
        changeOrigin: true
      },
      // Proxy images service - both API and static files
      '/api/images': {
        target: 'http://localhost:8083',
        changeOrigin: true
      },
      '/static': {
        target: 'http://localhost:8083',
        changeOrigin: true
      },
      // Proxy shipping service
      '/api/shipping': {
        target: 'http://localhost:8084',
        changeOrigin: true
      },
      // Proxy recommendations service
      '/api/recommendations': {
        target: 'http://localhost:8085',
        changeOrigin: true
      }
    }
  }
})
