import fs from 'node:fs'
import path from 'node:path'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// Write a build id (unix seconds) into the output dir. The Go server ranks
// the on-disk web/dist against the frontend embedded at compile time by this
// id (main.embeddedBuildID ldflag), so binary-only upgrades and dist-only
// rebuilds both resolve to the newer frontend.
function writeBuildId(): Plugin {
  let root = process.cwd()
  let outDir = 'dist'
  return {
    name: 'write-build-id',
    configResolved(config) {
      root = config.root
      outDir = config.build.outDir
    },
    closeBundle() {
      try {
        fs.writeFileSync(path.resolve(root, outDir, '.build-id'), `${Math.floor(Date.now() / 1000)}\n`)
      } catch {
        // Best-effort: SPAHandler falls back to index.html mtime ranking.
      }
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    writeBuildId(),
    VitePWA({
      registerType: 'autoUpdate',
      // Compile our own service worker (src/sw.ts: precache wiring + push
      // notification handlers). generateSW would ignore src/sw.ts entirely,
      // which is how the push handlers used to silently never ship. The
      // runtime-caching / navigation-fallback rules live in src/sw.ts too;
      // only the precache glob list stays here.
      strategies: 'injectManifest',
      srcDir: 'src',
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
      // NOTE: with strategies: 'injectManifest' the plugin reads glob options
      // from this `injectManifest` key (the `workbox` key is generateSW-only).
      injectManifest: {
        globPatterns: ['**/*.{js,css,html,ico,png,jpg,jpeg,svg,woff,woff2}'],
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
