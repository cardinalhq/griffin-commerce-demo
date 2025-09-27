import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// Production config that avoids esbuild for cross-platform builds
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  build: {
    // Disable minification to avoid esbuild issues with QEMU
    minify: false,
    // Disable CSS minification as well
    cssMinify: false,
    // Use Rollup for all transformations
    commonjsOptions: {
      transformMixedEsModules: true
    }
  },
  // Disable esbuild entirely
  esbuild: false
})