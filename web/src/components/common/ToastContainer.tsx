import clsx from 'clsx';
import { useEffect } from 'react';
import { useAppStore } from '../../store/useAppStore';

export function ToastContainer() {
  const toasts = useAppStore((s) => s.toasts);
  const dismissToast = useAppStore((s) => s.dismissToast);

  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map((t) =>
      setTimeout(() => dismissToast(t.id), 4000)
    );
    return () => timers.forEach(clearTimeout);
  }, [toasts, dismissToast]);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 space-y-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={clsx(
            'px-4 py-2.5 rounded-xl border text-xs font-medium shadow-lg flex items-center gap-2 min-w-[200px] max-w-[400px]',
            toast.type === 'error' && 'bg-rose-900/90 border-rose-500/40 text-rose-200',
            toast.type === 'warning' && 'bg-amber-900/90 border-amber-500/40 text-amber-200',
            toast.type === 'success' && 'bg-emerald-900/90 border-emerald-500/40 text-emerald-200',
            toast.type === 'info' && 'bg-slate-900/90 border-slate-700 text-slate-200'
          )}
        >
          <i className={clsx(
            'fa-solid text-[10px]',
            toast.type === 'error' && 'fa-circle-exclamation',
            toast.type === 'warning' && 'fa-triangle-exclamation',
            toast.type === 'success' && 'fa-circle-check',
            toast.type === 'info' && 'fa-circle-info'
          )}></i>
          <span className="flex-1">{toast.message}</span>
          <button onClick={() => dismissToast(toast.id)} className="text-slate-400 hover:text-slate-200">
            <i className="fa-solid fa-xmark text-[10px]"></i>
          </button>
        </div>
      ))}
    </div>
  );
}
