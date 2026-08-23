import clsx from 'clsx';
import { useEffect, useRef, useState } from 'react';
import type { Thread } from '../../types';
import { getBreedColor } from '../../lib/breed-colors';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';
import { useThreads } from '../../hooks/useThreads';
import { useBreeds } from '../../hooks/useBreeds';
import { useI18n } from '../../store/useI18n';

interface ThreadItemProps {
  thread: Thread;
}

export function ThreadItem({ thread }: ThreadItemProps) {
  const { t } = useI18n();
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const switchThread = useAppStore((s) => s.switchThread);
  const { deleteThread, renameThread } = useThreads();
  const { dogs } = useBreeds();
  // G4: unresolved CVO escalation lives in the chat store (WS-driven), not on
  // the REST thread payload.
  const hasEscalation = useChatStore((s) => !!s.escalations[thread.id]);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [editing, setEditing] = useState(false);
  const [draftTitle, setDraftTitle] = useState(thread.title);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

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

  const startEdit = (e: React.MouseEvent) => {
    e.stopPropagation();
    setDraftTitle(thread.title);
    setEditing(true);
  };

  const commitRename = async () => {
    const title = draftTitle.trim();
    setEditing(false);
    if (!title || title === thread.title) return;
    try {
      await renameThread(thread.id, title);
    } catch {
      // error handled in hook; keep old title
      setDraftTitle(thread.title);
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
          <i className={clsx('fa-regular text-xs shrink-0', hasEscalation ? 'fa-triangle-exclamation text-rose-400 animate-pulse' : 'fa-comments text-indigo-400')}></i>
          {editing ? (
            <input
              ref={inputRef}
              value={draftTitle}
              onChange={(e) => setDraftTitle(e.target.value)}
              onClick={(e) => e.stopPropagation()}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void commitRename();
                if (e.key === 'Escape') setEditing(false);
              }}
              onBlur={() => void commitRename()}
              maxLength={200}
              className="flex-1 min-w-0 bg-slate-900 border border-indigo-500/60 rounded px-1.5 py-0.5 text-xs text-slate-100 outline-none"
            />
          ) : (
            <span className="text-xs font-medium text-slate-200 truncate">{thread.title}</span>
          )}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          {hasEscalation && (
            <span className="text-[9px] font-mono px-1.5 py-0.5 rounded bg-rose-500/20 border border-rose-500/40 text-rose-300">CVO</span>
          )}
          {!editing && (
            <button
              onClick={startEdit}
              className="p-0.5 rounded transition text-slate-600 hover:text-indigo-400 opacity-0 group-hover:opacity-100"
              title={t('threads.renameTitle')}
            >
              <i className="fa-regular fa-pen-to-square text-[10px]"></i>
            </button>
          )}
          <button
            onClick={handleDelete}
            className={clsx(
              'p-0.5 rounded transition',
              confirmingDelete
                ? 'bg-rose-600 text-white'
                : 'text-slate-600 hover:text-rose-400 opacity-0 group-hover:opacity-100'
            )}
            title={confirmingDelete ? t('threads.threaditem.s1') : t('threads.threaditem.s2')}
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
