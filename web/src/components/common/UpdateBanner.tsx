import { useEffect, useState } from 'react';
import { fetchUpgradeInfo } from '../../services/update';

const POLL_INTERVAL_MS = 60_000;

/**
 * Polls the backend version and shows a banner once the running server is
 * newer than the page — i.e. a redeploy happened while this tab was open.
 * The banner offers a manual refresh; stale-tab chunk failures are already
 * self-healed by the ErrorBoundary, this just refreshes before breakage.
 */
export function UpdateBanner() {
  const [newVersion, setNewVersion] = useState<string | null>(null);

  useEffect(() => {
    let baseline: string | null = null;
    let cancelled = false;

    const check = async () => {
      try {
        const info = await fetchUpgradeInfo();
        if (cancelled || !info.version) return;
        if (baseline === null) {
          baseline = info.version;
        } else if (info.version !== baseline) {
          setNewVersion(info.version);
        }
      } catch {
        // Server unreachable — ConnectionStatusBar already surfaces that.
      }
      // Also nudge the service worker update check: the browser only
      // re-fetches sw.js on navigation by default, so a long-lived tab would
      // keep serving the old precache until its next reload. When the new SW
      // installs (skipWaiting + claim), the controllerchange listener in
      // main.tsx reloads the page onto the new build.
      if ('serviceWorker' in navigator) {
        void navigator.serviceWorker
          .getRegistration()
          .then((reg) => reg?.update())
          .catch(() => {
            // Best-effort; navigation still triggers the check.
          });
      }
    };

    const onVisibility = () => {
      if (document.visibilityState === 'visible') void check();
    };

    void check();
    const timer = window.setInterval(() => void check(), POLL_INTERVAL_MS);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, []);

  if (!newVersion) return null;

  return (
    <div
      role="status"
      style={{
        position: 'fixed',
        bottom: '1rem',
        right: '1rem',
        zIndex: 9999,
        display: 'flex',
        alignItems: 'center',
        gap: '0.75rem',
        padding: '0.75rem 1rem',
        borderRadius: '0.5rem',
        border: '1px solid rgb(99 102 241 / 0.5)',
        background: 'rgb(15 23 42 / 0.95)',
        color: '#e2e8f0',
        fontSize: '0.875rem',
        boxShadow: '0 4px 12px rgb(0 0 0 / 0.4)',
      }}
    >
      <span>
        新版本已部署 <strong style={{ color: '#a5b4fc' }}>{newVersion}</strong>
        ，建议刷新页面
      </span>
      <button
        onClick={() => window.location.reload()}
        style={{
          padding: '0.25rem 0.75rem',
          cursor: 'pointer',
          border: '1px solid rgb(99 102 241 / 0.6)',
          borderRadius: '0.375rem',
          background: 'rgb(67 56 202 / 0.6)',
          color: '#e2e8f0',
        }}
      >
        立即刷新
      </button>
    </div>
  );
}
