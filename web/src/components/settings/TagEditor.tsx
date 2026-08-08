import { useState } from 'react';

interface TagEditorProps {
  tags: string[];
  onChange: (tags: string[]) => void;
  placeholder?: string;
}

export function TagEditor({ tags, onChange, placeholder = '添加...' }: TagEditorProps) {
  const [input, setInput] = useState('');

  const addTag = () => {
    const v = input.trim();
    if (v && !tags.includes(v)) {
      onChange([...tags, v]);
    }
    setInput('');
  };

  const removeTag = (idx: number) => {
    onChange(tags.filter((_, i) => i !== idx));
  };

  return (
    <div className="flex flex-wrap gap-1.5 items-center">
      {tags.map((t, i) => (
        <span key={i} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg bg-slate-800 border border-slate-700 text-[11px] text-slate-200 font-mono">
          {t}
          <button onClick={() => removeTag(i)} className="text-slate-500 hover:text-rose-400">
            <i className="fa-solid fa-xmark text-[9px]"></i>
          </button>
        </span>
      ))}
      <input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addTag(); } }}
        onBlur={addTag}
        type="text"
        placeholder={placeholder}
        className="flex-1 min-w-[80px] bg-slate-950 border border-slate-800 rounded-lg px-2 py-0.5 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500 font-mono"
      />
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
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addEntry(); } }}
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
