import clsx from 'clsx';
import { useEffect, useRef, useState } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { useFocusTrap } from './useFocusTrap';
import type { Notification } from '../../types';

// NotificationCenter is the bell surface for the notification store: live
// SYSTEM_NOTICE events and mirrored backend notices land here. The list is
// read+unread aware; clicking an item marks it read.
export function NotificationCenter() {
  const { t } = useI18n();
  const notifications = useAppStore((s) => s.notifications);
  const markNotificationRead = useAppStore((s) => s.markNotificationRead);
  const markAllNotificationsRead = useAppStore((s) => s.markAllNotificationsRead);
  const clearNotifications = useAppStore((s) => s.clearNotifications);
  const [open, setOpen] = useState(false);

  const panelRef = useFocusTrap<HTMLDivElement>({
    isActive: open,
    onClose: () => setOpen(false),
  });
  const bellRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleMouseDown = (e: MouseEvent) => {
      const panel = panelRef.current;
      const bell = bellRef.current;
      if (panel && !panel.contains(e.target as Node) && bell && !bell.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleMouseDown);
    return () => document.removeEventListener('mousedown', handleMouseDown);
  }, [open, panelRef]);

  const unread = notifications.filter((n) => !n.read).length;
  const sorted = [...notifications].reverse(); // newest last pushed = newest first

  const severityIcon = (sev: Notification['severity']) =>
    sev === 'error' ? 'fa-circle-exclamation'
    : sev === 'warning' ? 'fa-triangle-exclamation'
    : 'fa-circle-info';
  const severityClass = (sev: Notification['severity']) =>
    sev === 'error' ? 'text-rose-400'
    : sev === 'warning' ? 'text-amber-400'
    : 'text-indigo-400';

  return (
    <div className="relative">
      <button
        ref={bellRef}
        onClick={() => setOpen(!open)}
        className="relative p-2 rounded-lg border border-slate-800 text-slate-400 hover:text-slate-200 hover:border-slate-700 transition text-xs"
        title={t('notif.title')}
        aria-label={t('notif.title')}
      >
        <i className="fa-solid fa-bell"></i>
        {unread > 0 && (
          <span className="absolute -top-1 -right-1 min-w-[16px] h-4 px-1 rounded-full bg-rose-500 text-white text-[9px] font-bold flex items-center justify-center ring-2 ring-slate-900">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div
          ref={panelRef}
          className="absolute right-0 top-full mt-2 z-40 w-96 rounded-xl bg-slate-900 border border-slate-800 shadow-2xl flex flex-col overflow-hidden"
          role="dialog"
          aria-label={t('notif.title')}
        >
          <div className="flex items-center justify-between px-3 py-2 border-b border-slate-800">
            <div className="flex items-center gap-2">
              <i className="fa-solid fa-bell text-indigo-400 text-xs"></i>
              <span className="text-xs font-bold text-slate-100">{t('notif.title')}</span>
              {unread > 0 && (
                <span className="text-[10px] font-mono text-rose-300">
                  {t('notif.unread').replace('{count}', String(unread))}
                </span>
              )}
            </div>
            <div className="flex items-center gap-1">
              <button
                onClick={markAllNotificationsRead}
                disabled={unread === 0}
                className="px-2 py-0.5 rounded text-[10px] text-slate-400 hover:text-slate-200 hover:bg-slate-800 disabled:opacity-40 transition"
              >
                {t('notif.markAllRead')}
              </button>
              <button
                onClick={() => clearNotifications()}
                disabled={notifications.length === 0}
                className="px-2 py-0.5 rounded text-[10px] text-slate-400 hover:text-rose-300 hover:bg-slate-800 disabled:opacity-40 transition"
              >
                {t('notif.clear')}
              </button>
            </div>
          </div>

          <div className="max-h-80 overflow-y-auto divide-y divide-slate-800/60">
            {sorted.length === 0 ? (
              <p className="px-3 py-6 text-center text-xs text-slate-500">{t('notif.empty')}</p>
            ) : (
              sorted.map((n) => (
                <button
                  key={n.id}
                  onClick={() => markNotificationRead(n.id)}
                  className={clsx(
                    'w-full text-left px-3 py-2.5 flex gap-2.5 transition hover:bg-slate-800/50',
                    !n.read && 'bg-indigo-500/5'
                  )}
                >
                  <i className={clsx('fa-solid text-xs mt-0.5', severityIcon(n.severity), severityClass(n.severity))}></i>
                  <span className="flex-1 min-w-0">
                    <span className="flex items-center justify-between gap-2">
                      <span className={clsx('text-xs truncate', n.read ? 'text-slate-400' : 'font-semibold text-slate-100')}>
                        {n.title}
                      </span>
                      <span className="text-[9px] font-mono text-slate-600 shrink-0">{n.timestamp}</span>
                    </span>
                    <span className={clsx('block text-[11px] mt-0.5 leading-snug', n.read ? 'text-slate-500' : 'text-slate-300')}>
                      {n.message}
                    </span>
                    {!n.read && <span className="inline-block mt-1 w-1.5 h-1.5 rounded-full bg-indigo-400"></span>}
                  </span>
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
