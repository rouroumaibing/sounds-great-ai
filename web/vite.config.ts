import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Sounds Great AI',
        short_name: 'SGAI',
        description: 'Multi-Agent Command Deck',
        theme_color: '#0f172a',
        background_color: '#0f172a',
        display: 'standalone',
        start_url: '/',
        icons: [
          {
            src: '/favicon.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'any',
          },
          {
            src: '/icons.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,jpg,jpeg,svg,woff,woff2}'],
        runtimeCaching: [
          {
            // API calls: NetworkOnly (no caching, always fresh)
            urlPattern: /\/api\/.*/i,
            handler: 'NetworkOnly',
            method: 'GET',
          },
          {
            // Static assets: CacheFirst (60 entries, 30 days)
            urlPattern: /\.(?:png|jpg|jpeg|svg|ico|woff|woff2)$/i,
            handler: 'CacheFirst',
            options: {
              cacheName: 'static-assets',
              expiration: {
                maxEntries: 60,
                maxAgeSeconds: 60 * 60 * 24 * 30, // 30 days
              },
            },
          },
        ],
        // Exclude WebSocket from caching (NetworkOnly by default, but explicit exclude)
        navigateFallbackDenylist: [/^\/ws/],
      },
      // PWA only active in production build (vite-plugin-pwa is disabled in dev by default)
      injectRegister: 'auto',
    }),
  ],
  server: {
    proxy: {
      '/ws': { target: 'ws://localhost:8080', ws: true },
      '/api': 'http://localhost:8080',
    },
  },
  optimizeDeps: {
    // Pre-bundle fontawesome to speed up dev server cold start
    include: ['@fortawesome/fontawesome-free'],
  },
  build: {
    chunkSizeWarningLimit: 1000,
    cssCodeSplit: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('react-dom') || id.includes('react/')) return 'react-vendor'
          if (id.includes('react-markdown') || id.includes('rehype-sanitize') || id.includes('remark-gfm')) return 'markdown'
          if (id.includes('@fortawesome')) return 'icons'
        },
      },
    },
  },
})
