import clsx from 'clsx';
import type { DragEvent as ReactDragEvent } from 'react';
import type { SettingsMember } from '../../types';
import { useI18n } from '../../store/useI18n';

const RUNTIME_LABELS: Record<string, string> = {
  claude: 'Claude',
  codex: 'Codex',
  gemini: 'Gemini',
  kimi: 'Kimi',
  opencode: 'OpenCode',
};

const OAUTH_REFS = new Set(['claude', 'codex', 'gemini', 'kimi', 'opencode']);

export function runtimeLabel(clientId?: string): string {
  if (!clientId) return '';
  return RUNTIME_LABELS[clientId] ?? clientId;
}

export function accountSummary(m: SettingsMember): string {
  const ref = m.accountRef?.trim() ?? '';
  if (!ref) return '';
  if (OAUTH_REFS.has(ref.toLowerCase())) return 'CLI（OAuth）';
  return useI18n.getState().t('settings.membercard.s1').replace('{ref}', ref);
}

export function formatMentionPreview(patterns?: string[], max = 3): string {
  if (!patterns || patterns.length === 0) return '';
  const visible = patterns.slice(0, max);
  const rest = patterns.length - visible.length;
  return rest > 0 ? `${visible.join(' ')}  +${rest}` : visible.join(' ');
}

interface MemberCardProps {
  member: SettingsMember;
  onEdit: (m: SettingsMember) => void;
  onToggle: (m: SettingsMember) => void;
  onDelete: (m: SettingsMember) => void;
  toggling?: boolean;
  draggable?: boolean;
  onDragStart?: (m: SettingsMember, e: ReactDragEvent<HTMLElement>) => void;
  onDragOver?: (m: SettingsMember, e: ReactDragEvent<HTMLElement>) => void;
  onDrop?: (m: SettingsMember, e: ReactDragEvent<HTMLElement>) => void;
  onDragEnd?: (m: SettingsMember, e: ReactDragEvent<HTMLElement>) => void;
  isDragging?: boolean;
}

export function MemberCard({
  member: m,
  onEdit,
  onToggle,
  onDelete,
  toggling = false,
  draggable = false,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  isDragging = false,
}: MemberCardProps) {
  const { t } = useI18n();
  const rtLabel = runtimeLabel(m.clientId);
  const acctSummary = accountSummary(m);
  const mentionPreview = formatMentionPreview(m.mentionPatterns);

  return (
    <div
      className={clsx(
        'p-4 rounded-2xl bg-slate-900/80 border transition flex items-center justify-between group shadow-sm cursor-pointer',
        isDragging ? 'border-amber-500/60 opacity-50' : 'border-slate-800 hover:border-slate-700',
      )}
      draggable={draggable}
      onDragStart={draggable ? (e) => onDragStart?.(m, e) : undefined}
      onDragOver={draggable ? (e) => onDragOver?.(m, e) : undefined}
      onDrop={draggable ? (e) => onDrop?.(m, e) : undefined}
      onDragEnd={draggable ? (e) => onDragEnd?.(m, e) : undefined}
      onClick={() => onEdit(m)}
    >
      <div className="flex items-center space-x-3 min-w-0">
        {draggable && (
          <span aria-hidden="true" title={t('members.dragSort')} className="select-none leading-none text-lg text-slate-500 hover:text-slate-300 cursor-grab shrink-0">
            ⠿
          </span>
        )}
        <div className="w-9 h-9 rounded-xl flex items-center justify-center text-white text-xs font-bold shrink-0 shadow" style={{ backgroundColor: m.colorPrimary ?? m.color }}>
          {m.avatar && m.avatar.length <= 2 ? m.avatar : <i className={m.icon}></i>}
        </div>
        <div className="space-y-1 truncate">
          <div className="flex items-center space-x-2 flex-wrap">
            <span className="text-xs font-bold text-slate-100">{m.name}</span>
            {rtLabel && (
              <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-slate-800 border border-slate-700 text-slate-300 font-semibold">
                {rtLabel}
              </span>
            )}
            {m.sessionChain && (
              <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-emerald-500/15 border border-emerald-500/30 text-emerald-300 font-semibold">
                Session Chain
              </span>
            )}
          </div>
          <div className="flex items-center space-x-2 text-[11px] font-mono text-slate-400 truncate">
            <span className="text-slate-500">{m.id}</span>
            <span>·</span>
            <span>{m.defaultModel ?? m.model}</span>
            {acctSummary && (
              <>
                <span>·</span>
                <span className="text-slate-500">{acctSummary}</span>
              </>
            )}
          </div>
          {mentionPreview && (
            <div className="text-[10px] font-mono text-purple-400 font-medium truncate">
              {mentionPreview}
            </div>
          )}
        </div>
      </div>
      <div className="flex items-center space-x-2 shrink-0 pl-3 border-l border-slate-800/80">
        <span className={clsx('text-[11px] font-mono px-2 py-0.5 rounded-lg border font-semibold', m.enabled ? 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30' : 'bg-slate-800 text-slate-500 border-slate-700')}>
          {m.enabled ? t('members.enabled') : t('members.disabled')}
        </span>
        <button
          onClick={(e) => { e.stopPropagation(); onToggle(m); }}
          disabled={toggling}
          className={clsx('w-11 h-6 rounded-full p-0.5 transition-colors relative focus:outline-none', m.enabled ? 'bg-amber-600' : 'bg-slate-800', toggling && 'opacity-50 cursor-wait')}
          title={m.enabled ? t('members.disableMember') : t('members.enableMember')}
        >
          {toggling ? (
            <i className="fa-solid fa-spinner fa-spin text-[10px] text-slate-300 absolute top-1 left-2.5"></i>
          ) : (
            <div className={clsx('w-5 h-5 rounded-full bg-white shadow-md transform transition-transform', m.enabled ? 'translate-x-5' : 'translate-x-0')}></div>
          )}
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onDelete(m); }}
          className="p-1.5 text-slate-500 hover:text-rose-400 transition"
          title={t('members.deleteMember')}
        >
          <i className="fa-regular fa-trash-can text-xs"></i>
        </button>
      </div>
    </div>
  );
}
