/// <reference lib="webworker" />

/**
 * Service Worker for Sounds Great AI PWA.
 *
 * vite-plugin-pwa compiles this file with the `injectManifest` strategy: the
 * workbox precache manifest is injected into self.__WB_MANIFEST at build time
 * (see vite.config.ts), while the caching routes below and the push
 * notification handlers are all defined here.
 */

import {
  precacheAndRoute,
  createHandlerBoundToURL,
  cleanupOutdatedCaches,
} from 'workbox-precaching'
import { registerRoute, NavigationRoute } from 'workbox-routing'
import { NetworkOnly, CacheFirst } from 'workbox-strategies'
import { ExpirationPlugin } from 'workbox-expiration'

// Cast self to ServiceWorkerGlobalScope for proper typing in SW context.
const sw = self as unknown as ServiceWorkerGlobalScope

// vite-plugin-pwa's injectManifest step scans the BUILT output for the
// literal token `self.__WB_MANIFEST` and replaces it with the precache
// manifest — reference it directly, never through an alias.
type PrecacheManifest = (string | { url: string; revision: string | null })[]

// --- Precache & routing ---
// The full build (every hashed chunk) is precached, which is what lets an
// offline or mid-deploy tab keep loading; on the next deploy the new manifest
// replaces it and autoUpdate + skipWaiting activate it immediately.
precacheAndRoute((self as unknown as { __WB_MANIFEST: PrecacheManifest }).__WB_MANIFEST)
cleanupOutdatedCaches()

// SPA navigation fallback: serve the app shell for document requests, except
// the WebSocket endpoint.
registerRoute(
  new NavigationRoute(createHandlerBoundToURL('index.html'), { denylist: [/^\/ws/] }),
)

// API calls: NetworkOnly — never cache.
registerRoute(/\/api\//i, new NetworkOnly(), 'GET')

// Static assets: CacheFirst (60 entries, 30 days).
registerRoute(
  /\.(?:png|jpg|jpeg|svg|ico|woff|woff2)$/i,
  new CacheFirst({
    cacheName: 'static-assets',
    plugins: [
      new ExpirationPlugin({
        maxEntries: 60,
        maxAgeSeconds: 60 * 60 * 24 * 30, // 30 days
      }),
    ],
  }),
  'GET',
)

// --- Notification dedup registry ---
// Prevents duplicate notifications for the same event within a time window.
const DEDUP_CACHE_PREFIX = 'notif-dedup:'
const DEDUP_WINDOW_MS = 5 * 60 * 1000 // 5 minutes

interface NotificationPayload {
  title: string
  body: string
  tag?: string
  data?: Record<string, unknown>
}

/**
 * Checks if a notification with the given tag was recently shown.
 * Uses the Cache API for dedup within the dedup window.
 */
async function isDuplicate(tag: string): Promise<boolean> {
  if (!tag) return false
  try {
    const cache = await caches.open('notif-dedup')
    const key = `${DEDUP_CACHE_PREFIX}${tag}`
    const cached = await cache.match(key)
    if (cached) {
      const timestamp = Number(await cached.text())
      if (Date.now() - timestamp < DEDUP_WINDOW_MS) {
        return true // duplicate within window
      }
    }
    return false
  } catch {
    return false // fail-open: show notification if cache check fails
  }
}

/**
 * Records that a notification with the given tag was shown.
 */
async function recordNotification(tag: string): Promise<void> {
  if (!tag) return
  try {
    const cache = await caches.open('notif-dedup')
    const key = `${DEDUP_CACHE_PREFIX}${tag}`
    const response = new Response(String(Date.now()))
    await cache.put(key, response)
  } catch {
    // fail silently — dedup is best-effort
  }
}

/**
 * Parses push event data into a NotificationPayload.
 */
function parsePushData(data: string | null): NotificationPayload {
  if (!data) {
    return { title: 'Sounds Great AI', body: 'New update' }
  }
  try {
    const parsed = JSON.parse(data)
    return {
      title: parsed.title ?? 'Sounds Great AI',
      body: parsed.body ?? '',
      tag: parsed.tag,
      data: parsed.data,
    }
  } catch {
    // If JSON parse fails, treat as plain text body
    return { title: 'Sounds Great AI', body: data }
  }
}

// --- Push notification handler ---
sw.addEventListener('push', (event: PushEvent) => {
  event.waitUntil(handlePush(event))
})

async function handlePush(event: PushEvent): Promise<void> {
  const payload = parsePushData(event.data?.text() ?? null)
  const tag = payload.tag ?? `notif-${Date.now()}`

  // Dedup check
  if (await isDuplicate(tag)) {
    return // skip duplicate
  }

  // Show notification
  await sw.registration.showNotification(payload.title, {
    body: payload.body,
    tag,
    data: payload.data,
    icon: '/favicon.svg',
    badge: '/favicon.svg',
  })

  // Record for dedup
  await recordNotification(tag)
}

// --- Notification click handler ---
sw.addEventListener('notificationclick', (event: NotificationEvent) => {
  event.waitUntil(handleNotificationClick(event))
})

async function handleNotificationClick(event: NotificationEvent): Promise<void> {
  event.notification.close()

  const data = event.notification.data as { url?: string } | undefined
  const targetUrl = data?.url ?? '/'

  // Check if app is already open in a tab
  const allClients = await sw.clients.matchAll({
    type: 'window',
    includeUncontrolled: true,
  })

  // Try to focus an existing tab
  for (const client of allClients) {
    if (client.url.includes(targetUrl)) {
      await client.focus()
      return
    }
  }

  // No existing tab — open a new one
  await sw.clients.openWindow(targetUrl)
}

// --- Service Worker lifecycle ---
sw.addEventListener('install', () => {
  // Skip waiting to activate immediately on update
  sw.skipWaiting()
})

sw.addEventListener('activate', (event: ExtendableEvent) => {
  // Claim all clients immediately on activation
  event.waitUntil(sw.clients.claim())
})

// --- Message handler (for manual update trigger) ---
sw.addEventListener('message', (event: ExtendableMessageEvent) => {
  if (event.data === 'SKIP_WAITING') {
    sw.skipWaiting()
  }
})

export {}
