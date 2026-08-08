import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useBreeds } from '../../hooks/useBreeds';

const severityConfig = {
  info: { icon: 'fa-solid fa-circle-info', color: 'text-cyan-400', bg: 'bg-cyan-500/10', border: 'border-cyan-500/20' },
  warning: { icon: 'fa-solid fa-triangle-exclamation', color: 'text-amber-400', bg: 'bg-amber-500/10', border: 'border-amber-500/20' },
  error: { icon: 'fa-solid fa-circle-exclamation', color: 'text-rose-400', bg: 'bg-rose-500/10', border: 'border-rose-500/20' },
};

const PREF_KEYS = ['reply', 'permission', 'mention', 'schedule', 'signal'] as const;
const PREF_LABELS: Record<string, { label: string; icon: string; desc: string }> = {
  reply: { label: '犬种消息', icon: 'fa-solid fa-dog', desc: '犬种回复完成通知' },
  permission: { label: '权限请求', icon: 'fa-solid fa-key', desc: '命令审批请求通知' },
  mention: { label: '@提及', icon: 'fa-solid fa-at', desc: '被提及时的通知' },
  schedule: { label: '定时任务', icon: 'fa-solid fa-clock', desc: '定时任务执行通知' },
  signal: { label: '信号更新', icon: 'fa-solid fa-signal', desc: '系统信号变更通知' },
};

type PrefKey = typeof PREF_KEYS[number];

export function NotificationsPanel() {
  const notifications = useAppStore((s) => s.notifications);
  const markNotificationRead = useAppStore((s) => s.markNotificationRead);
  const markAllNotificationsRead = useAppStore((s) => s.markAllNotificationsRead);
  const clearNotifications = useAppStore((s) => s.clearNotifications);
  const showToast = useAppStore((s) => s.showToast);
  const { dogs } = useBreeds();

  const unreadCount = notifications.filter((n) => !n.read).length;

  // Push subscription state
  const [pushSupported, setPushSupported] = useState(false);
  const [pushPermission, setPushPermission] = useState<NotificationPermission>('default');
  const [pushSubscribed, setPushSubscribed] = useState(false);
  const [vapidKey, setVapidKey] = useState('');
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(false);

  // Preferences (localStorage)
  const [prefs, setPrefs] = useState<Record<PrefKey, boolean>>({
    reply: true, permission: true, mention: true, schedule: false, signal: true,
  });

  useEffect(() => {
    setPushSupported('serviceWorker' in navigator && 'PushManager' in window);
    setPushPermission(typeof Notification !== 'undefined' ? Notification.permission : 'denied');
    setPushSubscribed(localStorage.getItem('push_subscribed') === 'true');
    setVapidKey(localStorage.getItem('vapid_public_key') ?? '');

    const saved = localStorage.getItem('notification_prefs');
    if (saved) {
      try { setPrefs(JSON.parse(saved)); } catch { /* ignore */ }
    }
  }, []);

  const handlePrefToggle = (key: PrefKey) => {
    const newPrefs = { ...prefs, [key]: !prefs[key] };
    setPrefs(newPrefs);
    localStorage.setItem('notification_prefs', JSON.stringify(newPrefs));
  };

  const handlePushSubscribe = async () => {
    if (!pushSupported) {
      showToast({ message: '浏览器不支持推送通知', type: 'warning' });
      return;
    }
    try {
      const perm = await Notification.requestPermission();
      setPushPermission(perm);
      if (perm === 'granted') {
        setPushSubscribed(true);
        localStorage.setItem('push_subscribed', 'true');
        showToast({ message: '推送通知已订阅', type: 'success' });
      }
    } catch {
      showToast({ message: '订阅失败', type: 'error' });
    }
  };

  const handlePushUnsubscribe = () => {
    setPushSubscribed(false);
    localStorage.removeItem('push_subscribed');
    showToast({ message: '已取消推送订阅', type: 'info' });
  };

  const handleTestNotification = () => {
    if (pushPermission === 'granted') {
      new Notification('Sounds Great AI', { body: '这是一条测试通知 🐕' });
    }
    showToast({ message: '测试通知已发送', type: 'info' });
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5 flex items-start justify-between">
        <div>
          <h2 className="text-2xl font-bold text-slate-100">通知</h2>
          <p className="text-xs text-slate-400 mt-1">推送订阅、通知偏好与诊断 {unreadCount > 0 && <span className="text-cyan-400">· {unreadCount} 条未读</span>}</p>
        </div>
        <div className="flex items-center space-x-2">
          <button onClick={markAllNotificationsRead} className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-semibold transition">
            全部已读
          </button>
          <button onClick={clearNotifications} className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-rose-600 text-slate-300 hover:text-white text-[11px] font-semibold transition">
            清空
          </button>
        </div>
      </div>

      {/* 通知渠道 */}
      <div className="space-y-3">
        <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider">通知渠道</h3>

        {/* Browser Push */}
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className={clsx('w-10 h-10 rounded-xl flex items-center justify-center', pushSubscribed ? 'bg-emerald-500/20' : 'bg-slate-800')}>
                <i className={clsx('fa-solid fa-bell text-sm', pushSubscribed ? 'text-emerald-400' : 'text-slate-500')}></i>
              </div>
              <div>
                <div className="text-xs font-bold text-slate-200">浏览器推送 (Web Push API)</div>
                <div className="text-[11px] text-slate-500 mt-0.5">
                  {!pushSupported ? '浏览器不支持' : pushSubscribed ? '已订阅' : '未订阅'}
                  {vapidKey && ` · VAPID: ${vapidKey.slice(0, 8)}...`}
                </div>
              </div>
            </div>
            <div className="flex items-center space-x-2">
              <button onClick={handleTestNotification} className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-semibold transition">
                测试
              </button>
              {pushSubscribed ? (
                <button onClick={handlePushUnsubscribe} className="px-3 py-1.5 rounded-xl bg-rose-500/20 text-rose-300 border border-rose-500/30 text-[11px] font-semibold hover:bg-rose-500/30 transition">
                  取消订阅
                </button>
              ) : (
                <button onClick={handlePushSubscribe} className="px-3 py-1.5 rounded-xl bg-indigo-500 text-white text-[11px] font-semibold hover:bg-indigo-400 transition">
                  订阅
                </button>
              )}
            </div>
          </div>
        </div>

        {/* App notification (always on) */}
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="w-10 h-10 rounded-xl bg-cyan-500/20 flex items-center justify-center">
                <i className="fa-solid fa-app-indicator text-cyan-400 text-sm"></i>
              </div>
              <div>
                <div className="text-xs font-bold text-slate-200">应用内通知</div>
                <div className="text-[11px] text-slate-500 mt-0.5">始终开启</div>
              </div>
            </div>
            <span className="px-2 py-1 rounded bg-emerald-500/20 text-emerald-300 text-[10px] font-semibold">ON</span>
          </div>
        </div>
      </div>

      {/* 通知偏好 */}
      <div className="space-y-3">
        <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider">通知偏好</h3>
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
          <div className="divide-y divide-slate-800/40">
            {PREF_KEYS.map((key) => {
              const cfg = PREF_LABELS[key];
              return (
                <div key={key} className="px-4 py-3 flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <i className={clsx(cfg.icon, 'text-slate-400 text-xs w-4 text-center')}></i>
                    <div>
                      <div className="text-xs font-bold text-slate-200">{cfg.label}</div>
                      <div className="text-[11px] text-slate-500">{cfg.desc}</div>
                    </div>
                  </div>
                  <ToggleSwitch checked={prefs[key]} onChange={() => handlePrefToggle(key)} />
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* 诊断区 */}
      <div className="space-y-3">
        <button onClick={() => setDiagnosticsOpen(!diagnosticsOpen)} className="flex items-center space-x-2 text-xs font-bold text-slate-300 uppercase tracking-wider hover:text-slate-100 transition">
          <i className={clsx('fa-solid fa-chevron-right transition-transform', diagnosticsOpen && 'rotate-90')}></i>
          诊断
        </button>
        {diagnosticsOpen && (
          <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
            <DiagRow label="Notification 权限" value={pushPermission} status={pushPermission === 'granted' ? 'ok' : 'warn'} />
            <DiagRow label="Push 支持" value={pushSupported ? '支持' : '不支持'} status={pushSupported ? 'ok' : 'error'} />
            <DiagRow label="Push 订阅状态" value={pushSubscribed ? '已订阅' : '未订阅'} status={pushSubscribed ? 'ok' : 'warn'} />
            <DiagRow label="VAPID 公钥" value={vapidKey ? `${vapidKey.slice(0, 16)}...` : '未配置'} status={vapidKey ? 'ok' : 'warn'} />
            <DiagRow label="已发送通知" value={`${notifications.length} 条`} status="ok" />
            <DiagRow label="未读通知" value={`${unreadCount} 条`} status={unreadCount > 0 ? 'warn' : 'ok'} />
          </div>
        )}
      </div>

      {/* 通知列表 */}
      <div className="space-y-3">
        <h3 className="text-xs font-bold text-slate-300 uppercase tracking-wider">通知列表</h3>
        {notifications.length === 0 ? (
          <div className="p-8 rounded-2xl bg-slate-900/40 border border-slate-800 text-center">
            <i className="fa-solid fa-bell-slash text-2xl text-slate-600"></i>
            <p className="text-xs text-slate-500 mt-2">暂无通知</p>
          </div>
        ) : (
          <div className="space-y-2">
            {notifications.map((n) => {
              const sc = severityConfig[n.severity];
              return (
                <div
                  key={n.id}
                  onClick={() => !n.read && markNotificationRead(n.id)}
                  className={clsx('p-3 rounded-2xl border cursor-pointer transition', n.read ? 'bg-slate-900/30 border-slate-800/40' : `${sc.bg} ${sc.border} hover:bg-slate-800/40`)}
                >
                  <div className="flex items-start space-x-3">
                    <i className={clsx(sc.icon, sc.color, 'text-sm mt-0.5 shrink-0')}></i>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <span className={clsx('text-xs font-bold', n.read ? 'text-slate-300' : 'text-slate-100')}>{n.title}</span>
                        <div className="flex items-center space-x-2 shrink-0">
                          {!n.read && <span className="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>}
                          <span className="text-[10px] font-mono text-slate-500">{n.timestamp}</span>
                        </div>
                      </div>
                      <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">{n.message}</p>
                      <div className="flex items-center space-x-1 mt-1.5">
                        <span className="text-[10px] font-mono text-slate-500">来源:</span>
                        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
                          {dogs.find((d) => d.id === n.source)?.name ?? n.source}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function ToggleSwitch({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button
      onClick={onChange}
      className={clsx('relative w-10 h-5 rounded-full transition', checked ? 'bg-indigo-500' : 'bg-slate-700')}
    >
      <span className={clsx('absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform', checked ? 'translate-x-5' : 'translate-x-0.5')} />
    </button>
  );
}

function DiagRow({ label, value, status }: { label: string; value: string; status: 'ok' | 'warn' | 'error' }) {
  const colors = {
    ok: 'text-emerald-400',
    warn: 'text-amber-400',
    error: 'text-rose-400',
  };
  return (
    <div className="flex items-center justify-between text-[11px]">
      <span className="text-slate-500">{label}</span>
      <span className={clsx('font-mono', colors[status])}>{value}</span>
    </div>
  );
}
