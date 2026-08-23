import { apiGet } from './http';

const AUTO_RELOAD_KEY = 'sg-auto-reload-at';
const AUTO_RELOAD_THROTTLE_MS = 30_000;

/**
 * Reload the page at most once per throttle window. Shared by the
 * ErrorBoundary (stale chunk after a redeploy) and the service-worker
 * controllerchange handler, so a persistently broken deploy cannot turn
 * recovery into a reload loop.
 */
export function tryAutoReload(): boolean {
  let last = 0;
  try {
    last = Number(sessionStorage.getItem(AUTO_RELOAD_KEY) ?? 0);
  } catch {
    // Private mode / storage disabled — fall through and reload.
  }
  if (Date.now() - last < AUTO_RELOAD_THROTTLE_MS) return false;
  try {
    sessionStorage.setItem(AUTO_RELOAD_KEY, String(Date.now()));
  } catch {
    // Swallow: worst case an unhealthy deploy reload-loops until the user
    // closes the tab, which is still better than a permanently dead tab.
  }
  window.location.reload();
  return true;
}

/**
 * True when the error is a lazy-route chunk that vanished after a redeploy:
 * the running page still references the old content hash, which no longer
 * exists on the server. A fresh page load always recovers.
 */
export function isStaleChunkError(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err);
  return (
    msg.includes('Failed to fetch dynamically imported module') ||
    msg.includes('Importing a module script failed') ||
    msg.includes('error loading dynamically imported module') ||
    // Chrome refuses module scripts served as text/html (the old SPA
    // fallback used to answer missing assets with index.html).
    (msg.includes('MIME type') && msg.includes('text/html'))
  );
}

export interface UpgradeInfo {
  mode: string;
  version: string;
  repo: string;
}

export function fetchUpgradeInfo(): Promise<UpgradeInfo> {
  return apiGet<UpgradeInfo>('/api/upgrade/info');
}
