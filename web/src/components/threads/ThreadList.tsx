import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useThreads } from '../../hooks/useThreads';
import { ThreadItem } from './ThreadItem';
import { VirtualList } from '../common/VirtualList';
import type { Thread } from '../../types';
import { useI18n } from '../../store/useI18n';

const THREAD_ITEM_HEIGHT = 70;
const VIRTUAL_THRESHOLD = 30;

export function ThreadList() {
  const { t } = useI18n();
  const threadSearchInput = useAppStore((s) => s.threadSearchInput);
  const setThreadSearchInput = useAppStore((s) => s.setThreadSearchInput);
  const threadFilter = useAppStore((s) => s.threadFilter);
  const setThreadFilter = useAppStore((s) => s.setThreadFilter);

  const { threads, loading, error, createThread } = useThreads();

  const filteredThreads = threads.filter((t) => {
    const matchesSearch =
      !threadSearchInput ||
      t.title.toLowerCase().includes(threadSearchInput.toLowerCase()) ||
      t.id.includes(threadSearchInput);
    if (!matchesSearch) return false;
    if (threadFilter === 'escalated') return t.hasEscalation;
    if (threadFilter === 'active') return !t.hasEscalation;
    return true;
  });

  const handleCreate = async () => {
    try {
      await createThread(t('threads.newThread'));
    } catch {
      // error handled in hook
    }
  };

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="p-3 border-b border-slate-800/80 flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
            <i className="fa-solid fa-layer-group text-indigo-400"></i>
            Active Threads
          </span>
          <span className="text-[10px] bg-indigo-500/20 text-indigo-300 px-1.5 py-0.5 rounded font-mono">{threads.length}</span>
        </div>
        <button
          onClick={handleCreate}
          title={t('threads.createTitle')}
          className="p-1 rounded-md bg-indigo-600/80 hover:bg-indigo-500 text-white text-xs transition"
        >
          <i className="fa-solid fa-plus"></i>
        </button>
      </div>

      <div className="p-2 border-b border-slate-800/60 space-y-2 bg-slate-950/40">
        <div className="relative">
          <i className="fa-solid fa-magnifying-glass absolute left-2.5 top-2.5 text-[10px] text-slate-500"></i>
          <input
            value={threadSearchInput}
            onChange={(e) => setThreadSearchInput(e.target.value)}
            type="text"
            placeholder={t('threads.searchPlaceholder')}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg pl-7 pr-2 py-1.5 text-[11px] text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500/60 transition"
          />
        </div>
        <div className="flex items-center space-x-1 text-[10px]">
          <button onClick={() => setThreadFilter('all')} className={clsx('px-2 py-0.5 rounded-full border transition', threadFilter === 'all' ? 'bg-indigo-500/20 border-indigo-500/40 text-indigo-300' : 'border-slate-800 text-slate-400 hover:text-slate-200')}>{t('common.all')}</button>
          <button onClick={() => setThreadFilter('escalated')} className={clsx('px-2 py-0.5 rounded-full border transition flex items-center gap-1', threadFilter === 'escalated' ? 'bg-rose-500/20 border-rose-500/40 text-rose-300' : 'border-slate-800 text-slate-400 hover:text-slate-200')}>
            <span className="w-1.5 h-1.5 rounded-full bg-rose-500"></span> {t('auto3.threadlist.1')}
          </button>
          <button onClick={() => setThreadFilter('active')} className={clsx('px-2 py-0.5 rounded-full border transition', threadFilter === 'active' ? 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300' : 'border-slate-800 text-slate-400 hover:text-slate-200')}>{t('auto.threadlist.2')}</button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
        {loading && <div className="text-center text-slate-500 text-xs py-4">{t('common.loading')}</div>}
        {error && <div className="text-center text-rose-400 text-xs py-4">{t('common.error')}</div>}
        {!loading && !error && filteredThreads.length === 0 && (
          <div className="text-center text-slate-500 text-xs py-4">{t('auto.threadlist.3')}</div>
        )}
        {!loading && !error && filteredThreads.length > 0 && filteredThreads.length <= VIRTUAL_THRESHOLD && (
          filteredThreads.map((thread) => (
            <ThreadItem key={thread.id} thread={thread} />
          ))
        )}
        {!loading && !error && filteredThreads.length > VIRTUAL_THRESHOLD && (
          <VirtualList
            items={filteredThreads}
            itemHeight={THREAD_ITEM_HEIGHT}
            renderItem={(thread: Thread) => (
              <div className="px-0.5">
                <ThreadItem key={thread.id} thread={thread} />
              </div>
            )}
          />
        )}
      </div>
    </div>
  );
}
