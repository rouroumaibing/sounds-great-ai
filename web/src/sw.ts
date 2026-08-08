/// <reference lib="webworker" />

/**
 * Service Worker for Sounds Great AI PWA.
 *
 * Handles:
 * - Push notifications with dedup registry
 * - Notification click → focus/open app
 *
 * The actual caching (static assets, API NetworkOnly) is handled by
 * vite-plugin-pwa's Workbox-generated service worker. This file provides
 * the custom push notification and notificationclick handlers.
 */

// Cast self to ServiceWorkerGlobalScope for proper typing in SW context.
const sw = self as unknown as ServiceWorkerGlobalScope

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
 * Check if a notification with the given tag was recently shown.
 * Uses Cache API for dedup within the dedup window.
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
 * Record that a notification was shown, for future dedup checks.
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
 * Parse push event data into a NotificationPayload.
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
