import { useState } from 'react';
import { useI18n } from '../../store/useI18n';

function normalizeTag(value: string): string {
  return value.trim();
}

function mergeTags(tags: string[], nextTag: string): string[] {
  return Array.from(new Set([...tags, nextTag]));
}

function pillClass(tone: 'purple' | 'green' | 'orange'): string {
  if (tone === 'green') return 'bg-emerald-900/40 text-emerald-300 border border-emerald-700/40';
  if (tone === 'orange') return 'bg-amber-900/30 text-amber-300 border border-amber-700/40';
  return 'bg-slate-800 text-slate-200 border border-slate-700';
}

interface TagEditorProps {
  tags: string[];
  onChange: (tags: string[]) => void;
  /** 点击后展开输入框的「添加」按钮文案；不传则保持 SG 原有内联输入框。 */
  addLabel?: string;
  placeholder?: string;
  /** 无标签时显示的提示文案。 */
  emptyLabel?: string;
  lockedTags?: string[];
  tone?: 'purple' | 'green' | 'orange';
  normalize?: (value: string) => string;
  /** 校验：返回错误信息则阻止添加，返回 null 表示通过。 */
  validate?: (tag: string) => string | null;
  minCount?: number;
}

/**
 * 标签编辑器：
 *  - 提供 addLabel / emptyLabel / validate / minCount / lockedTags / tone 能力（别名唯一性校验、最少数量保护）
 *  - 当不传 addLabel 时降级为 SG 原有的「始终可见内联输入框」行为，保证 AccountAuthModal 等既有调用向后兼容。
 */
export function TagEditor({
  tags,
  onChange,
  addLabel,
  placeholder: _placeholder,
  emptyLabel,
  lockedTags = [],
  tone = 'purple',
  normalize = normalizeTag,
  validate,
  minCount = 0,
}: TagEditorProps) {
  const { t } = useI18n();
  const placeholder = _placeholder ?? t('tagEditor.placeholder');
  const [adding, setAdding] = useState(false);
  const [input, setInput] = useState('');
  const [error, setError] = useState<string | null>(null);

  const removableCount = tags.filter((tag) => !lockedTags.includes(tag)).length;

  const addTag = () => {
    const nextTag = normalize(input);
    if (!nextTag) {
      setAdding(false);
      setInput('');
      setError(null);
      return;
    }
    const err = validate?.(nextTag) ?? null;
    if (err) {
      setError(err);
      return;
    }
    onChange(mergeTags(tags, nextTag));
    setAdding(false);
    setInput('');
    setError(null);
  };

  const removeTag = (tag: string) => {
    if (removableCount > minCount) onChange(tags.filter((item) => item !== tag));
  };

  // ---- 「+ 添加」按钮（传入 addLabel 时启用）----
  if (addLabel) {
    return (
      <div className="space-y-2">
        <div className="flex flex-wrap gap-1.5 items-center">
          {tags.length === 0 && emptyLabel ? (
            <span className="text-[11px] italic text-slate-500">{emptyLabel}</span>
          ) : null}
          {tags.map((tag) => (
            <span
              key={tag}
              className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-lg text-[11px] font-mono ${pillClass(tone)}`}
            >
              <span>{tag}</span>
              {!lockedTags.includes(tag) && removableCount > minCount ? (
                <button
                  type="button"
                  aria-label={`移除 ${tag}`}
                  onClick={() => removeTag(tag)}
                  className="text-slate-400 hover:text-rose-400"
                >
                  ×
                </button>
              ) : null}
            </span>
          ))}
          <button
            type="button"
            onClick={() => setAdding((value) => !value)}
            className={`rounded-full px-2.5 py-0.5 text-[11px] font-medium ${pillClass(tone)}`}
          >
            {addLabel}
          </button>
        </div>
        {adding ? (
          <div className="flex flex-wrap items-center gap-2">
            <input
              autoFocus
              value={input}
              onChange={(event) => {
                setInput(event.target.value);
                if (error) setError(null);
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  addTag();
                }
              }}
              placeholder={placeholder}
              className="min-w-[180px] flex-1 bg-slate-950 border border-slate-800 rounded-lg px-2 py-1 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500 font-mono"
            />
            <button
              type="button"
              onClick={addTag}
              className="rounded-full bg-slate-800 px-3 py-1 text-[11px] font-medium text-amber-400"
            >
              添加
            </button>
            {error && <span className="w-full text-[11px] text-rose-400">{error}</span>}
          </div>
        ) : null}
      </div>
    );
  }

  // ---- SG 原有内联输入框（向后兼容）----
  return (
    <div className="flex flex-wrap gap-1.5 items-center">
      {tags.map((tag) => (
        <span
          key={tag}
          className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-lg bg-slate-800 border border-slate-700 text-[11px] text-slate-200 font-mono ${pillClass(tone)}`}
        >
          {tag}
          {!lockedTags.includes(tag) && removableCount > minCount ? (
            <button
              onClick={() => removeTag(tag)}
              className="text-slate-500 hover:text-rose-400"
              aria-label={`移除 ${tag}`}
            >
              ×
            </button>
          ) : null}
        </span>
      ))}
      <input
        value={input}
        onChange={(e) => {
          setInput(e.target.value);
          if (error) setError(null);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            addTag();
          }
        }}
        onBlur={addTag}
        type="text"
        placeholder={placeholder}
        className="flex-1 min-w-[80px] bg-slate-950 border border-slate-800 rounded-lg px-2 py-0.5 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500 font-mono"
      />
      {error && <span className="w-full text-[11px] text-rose-400">{error}</span>}
    </div>
  );
}

interface KeyValueEditorProps {
  entries: Record<string, string>;
  onChange: (entries: Record<string, string>) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}

export function KeyValueEditor({ entries, onChange, keyPlaceholder = 'key', valuePlaceholder = 'value' }: KeyValueEditorProps) {
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  const addEntry = () => {
    const k = newKey.trim();
    if (k) {
      onChange({ ...entries, [k]: newValue.trim() });
    }
    setNewKey('');
    setNewValue('');
  };

  const removeEntry = (key: string) => {
    const next = { ...entries };
    delete next[key];
    onChange(next);
  };

  return (
    <div className="space-y-1.5">
      {Object.entries(entries).map(([k, v]) => (
        <div key={k} className="flex items-center gap-2">
          <span className="text-[11px] font-mono text-slate-300 w-1/3 truncate">{k}</span>
          <span className="text-[11px] font-mono text-slate-400 flex-1 truncate">{v}</span>
          <button onClick={() => removeEntry(k)} className="text-slate-500 hover:text-rose-400 p-0.5">
            <i className="fa-solid fa-xmark text-[9px]"></i>
          </button>
        </div>
      ))}
      <div className="flex items-center gap-2">
        <input
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          type="text"
          placeholder={keyPlaceholder}
          className="flex-1 bg-slate-950 border border-slate-800 rounded-lg px-2 py-1 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500 font-mono"
        />
        <input
          value={newValue}
          onChange={(e) => setNewValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              addEntry();
            }
          }}
          type="text"
          placeholder={valuePlaceholder}
          className="flex-1 bg-slate-950 border border-slate-800 rounded-lg px-2 py-1 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500 font-mono"
        />
        <button onClick={addEntry} className="px-2 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px]">
          <i className="fa-solid fa-plus"></i>
        </button>
      </div>
    </div>
  );
}
