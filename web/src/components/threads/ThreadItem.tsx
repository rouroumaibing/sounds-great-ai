import clsx from 'clsx';
import { useState } from 'react';
import type { Thread } from '../../types';
import { getBreedColor } from '../../lib/breed-colors';
import { useAppStore } from '../../store/useAppStore';
import { useThreads } from '../../hooks/useThreads';
import { useBreeds } from '../../hooks/useBreeds';

interface ThreadItemProps {
  thread: Thread;
}

export function ThreadItem({ thread }: ThreadItemProps) {
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const switchThread = useAppStore((s) => s.switchThread);
  const { deleteThread } = useThreads();
  const { dogs } = useBreeds();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const handleDelete = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirmingDelete) {
      setConfirmingDelete(true);
      setTimeout(() => setConfirmingDelete(false), 3000);
      return;
    }
    try {
      await deleteThread(thread.id);
    } catch {
      // error handled in hook
    }
  };

  return (
    <div
      onClick={() => switchThread(thread.id)}
      className={clsx(
        'p-2.5 rounded-xl border transition cursor-pointer flex flex-col gap-1.5 relative group',
        activeThreadId === thread.id
          ? 'bg-indigo-950/40 border-indigo-500/50 shadow-sm'
          : 'bg-slate-950/40 border-slate-800/60 hover:bg-slate-900/60'
      )}
    >
      <div className="flex items-start justify-between gap-1">
        <div className="flex items-center space-x-2 min-w-0">
          <i className={clsx('fa-regular text-xs shrink-0', thread.hasEscalation ? 'fa-triangle-exclamation text-rose-400 animate-pulse' : 'fa-comments text-indigo-400')}></i>
          <span className="text-xs font-medium text-slate-200 truncate">{thread.title}</span>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          {thread.hasEscalation && (
            <span className="text-[9px] font-mono px-1.5 py-0.5 rounded bg-rose-500/20 border border-rose-500/40 text-rose-300">CVO</span>
          )}
          <button
            onClick={handleDelete}
            className={clsx(
              'p-0.5 rounded transition',
              confirmingDelete
                ? 'bg-rose-600 text-white'
                : 'text-slate-600 hover:text-rose-400 opacity-0 group-hover:opacity-100'
            )}
            title={confirmingDelete ? '再次点击确认删除' : '删除线程'}
          >
            <i className={clsx('fa-regular text-[10px]', confirmingDelete ? 'fa-trash-can' : 'fa-trash-can')}></i>
          </button>
        </div>
      </div>

      <div className="flex items-center justify-between text-[10px] text-slate-500 font-mono">
        <div className="flex items-center -space-x-1">
          {(thread.agents ?? []).map((bId) => {
            const dog = dogs.find((d) => d.id === bId);
            return (
            <div
              key={bId}
              className="w-4 h-4 rounded-full flex items-center justify-center text-[8px] text-white border border-slate-900"
              style={{ backgroundColor: dog?.color ?? getBreedColor(bId).primary }}
              title={dog?.name ?? bId}
            >
              <i className={dog?.icon ?? ''}></i>
            </div>
            );
          })}
        </div>
        <span>{thread.updatedAt ?? ''}</span>
      </div>
    </div>
  );
}
