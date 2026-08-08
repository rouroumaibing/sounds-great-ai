import { useEffect, useState } from 'react';
import type { LeaderConfig } from '../../hooks/useLeaderConfig';
import { TagEditor } from './TagEditor';

interface HubLeaderEditorProps {
  leader: LeaderConfig;
  onSave: (cfg: LeaderConfig) => Promise<boolean>;
  onClose: () => void;
}

function normalizeMentionTag(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '';
  return trimmed.startsWith('@') ? trimmed : `@${trimmed}`;
}

function uniqueTags(tags: string[]): string[] {
  return Array.from(new Set(tags.filter(Boolean)));
}

const inputCls = 'w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-slate-200 focus:outline-none focus:border-amber-500';

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-slate-400 mb-1">{label}</label>
      {children}
    </div>
  );
}

export function HubLeaderEditor({ leader, onSave, onClose }: HubLeaderEditorProps) {
  const [name, setName] = useState(leader.name);
  const [avatar, setAvatar] = useState(leader.avatar ?? '');
  const [colorPrimary, setColorPrimary] = useState(leader.colorPrimary ?? '#6366f1');
  const [colorSecondary, setColorSecondary] = useState(leader.colorSecondary ?? '#818cf8');
  const [timeZone, setTimeZone] = useState(leader.timeZone ?? 'Asia/Shanghai');
  const [aliases, setAliases] = useState<string[]>(leader.aliases ?? []);
  const [mentionPatterns, setMentionPatterns] = useState<string[]>(leader.mentionPatterns ?? ['@leader']);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setName(leader.name);
    setAvatar(leader.avatar ?? '');
    setColorPrimary(leader.colorPrimary ?? '#6366f1');
    setColorSecondary(leader.colorSecondary ?? '#818cf8');
    setTimeZone(leader.timeZone ?? 'Asia/Shanghai');
    setAliases(leader.aliases ?? []);
    setMentionPatterns(leader.mentionPatterns ?? ['@leader']);
    setError(null);
  }, [leader]);

  const handleSave = async () => {
    const cleanedName = name.trim();
    const cleanedMentions = uniqueTags(mentionPatterns.map(normalizeMentionTag));
    if (!cleanedName) {
      setError('Leader 名称不能为空');
      return;
    }
    if (cleanedMentions.length === 0) {
      setError('至少保留一个 @ 标签');
      return;
    }

    setSaving(true);
    setError(null);
    const ok = await onSave({
      name: cleanedName,
      aliases: uniqueTags(aliases.map((a) => a.trim())),
      mentionPatterns: cleanedMentions,
      timeZone: timeZone.trim() || 'Asia/Shanghai',
      avatar: avatar.trim() || undefined,
      colorPrimary,
      colorSecondary,
    });
    setSaving(false);
    if (ok) {
      onClose();
    }
  };

  const aliasesDisplay = aliases.length > 0 ? aliases.join(' · ') : 'You · Owner';

  return (
    <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl p-6 space-y-4 shadow-2xl max-h-[90vh] overflow-y-auto" role="dialog" aria-modal="true">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <div>
            <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
              <i className="fa-solid fa-user-tie text-indigo-400"></i>
              <span>Leader 设置</span>
            </h3>
            <p className="text-[11px] text-slate-500 mt-0.5">Leader（用户/Owner）</p>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>

        {/* 身份信息 */}
        <div className="space-y-3">
          <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider">身份信息</h4>
          <div className="grid grid-cols-2 gap-3 text-xs">
            <Field label="名称 (必填)">
              <input value={name} onChange={(e) => setName(e.target.value)} className={inputCls} placeholder="Leader 显示名称" />
            </Field>
            <Field label="Avatar (URL/emoji)">
              <input value={avatar} onChange={(e) => setAvatar(e.target.value)} className={inputCls} placeholder="URL 或 emoji" />
            </Field>
            <Field label="Color Primary">
              <input type="color" value={colorPrimary} onChange={(e) => setColorPrimary(e.target.value)} className="h-10 w-full rounded-xl border border-slate-800 bg-transparent cursor-pointer" />
            </Field>
            <Field label="Color Secondary">
              <input type="color" value={colorSecondary} onChange={(e) => setColorSecondary(e.target.value)} className="h-10 w-full rounded-xl border border-slate-800 bg-transparent cursor-pointer" />
            </Field>
            <Field label="TimeZone">
              <input value={timeZone} onChange={(e) => setTimeZone(e.target.value)} className={inputCls} placeholder="Asia/Shanghai" />
            </Field>
          </div>
        </div>

        {/* 别名与 @ 路由 */}
        <div className="space-y-3">
          <h4 className="text-xs font-bold text-slate-300 uppercase tracking-wider">别名与 @ 路由</h4>
          <div className="space-y-3 text-xs">
            {/* 只读别名信息 */}
            <div className="rounded-xl bg-slate-950/60 border border-slate-800 px-3 py-2">
              <div className="flex items-center gap-2 mb-1">
                <span className="text-slate-400">别名</span>
                <span className="text-amber-400/80 text-[10px]">只能编辑，不能新增或删除</span>
              </div>
              <div className="text-slate-200 font-medium">{aliasesDisplay}</div>
            </div>
            <Field label="@ 标签 (至少 1 个)">
              <TagEditor
                tags={mentionPatterns}
                onChange={(next) => setMentionPatterns(next.map(normalizeMentionTag).filter(Boolean))}
                placeholder="@leader"
              />
            </Field>
          </div>
        </div>

        {error && (
          <p className="rounded-xl bg-rose-500/10 border border-rose-500/30 px-4 py-2 text-xs text-rose-400">{error}</p>
        )}

        <div className="flex justify-end space-x-2 pt-2 border-t border-slate-800">
          <button onClick={onClose} className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium">取消</button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-lg shadow-indigo-600/20 disabled:opacity-50"
          >
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}
