// 添加成员弹窗的表单字段组件，适配 SG 深色主题。
// （slate 底 + amber 强调），去掉 --console-* / --cafe-* 浅色 CSS 变量。

import type { HTMLAttributes, ReactNode } from 'react';

function FieldShell({
  label,
  required = false,
  tone = 'neutral',
  children,
}: {
  label: string;
  required?: boolean;
  tone?: 'neutral' | 'success';
  children: ReactNode;
}) {
  const labelColor = tone === 'success' ? 'text-amber-300' : 'text-slate-300';
  return (
    <label className="flex flex-col gap-1.5 text-slate-300 sm:flex-row sm:items-center sm:gap-[14px]">
      <span className={`text-xs font-bold ${labelColor} sm:w-[150px] sm:shrink-0`}>
        {label}
        {required && <span className="ml-0.5 text-rose-400">*</span>}
      </span>
      <div className="min-w-0 flex-1">{children}</div>
    </label>
  );
}

export function SectionCard({
  title,
  description,
  tone = 'neutral',
  children,
  ...rest
}: {
  title: string;
  description?: string;
  tone?: 'neutral' | 'success' | 'error';
  children: ReactNode;
} & HTMLAttributes<HTMLElement>) {
  const toneClasses: Record<string, string> = {
    neutral: 'bg-slate-950/30 border border-slate-800',
    success: 'bg-slate-950/40 border border-amber-700/40',
    error: 'border border-rose-500/50 bg-rose-500/10 animate-[shake_0.3s_ease-in-out]',
  };
  const toneClass = toneClasses[tone] ?? toneClasses.neutral;
  return (
    <section className={`rounded-[18px] p-[18px] transition-colors ${toneClass}`} {...rest}>
      <div className="space-y-1">
        <h4
          className={`text-base font-extrabold ${tone === 'success' ? 'text-amber-300' : 'text-slate-100'}`}
        >
          {title}
        </h4>
        {description ? (
          <p
            className={`text-xs leading-5 ${tone === 'success' ? 'font-semibold text-amber-200/80' : 'text-slate-400'}`}
          >
            {description}
          </p>
        ) : null}
      </div>
      <div className="mt-3 space-y-2.5">{children}</div>
    </section>
  );
}

const inputCls =
  'w-full rounded-[10px] border border-slate-800 bg-slate-950 px-3 py-1.5 text-compact leading-5 text-slate-200 placeholder:text-slate-500 outline-none transition focus:ring-1 focus:border-amber-500 focus:ring-amber-500/30';

export function TextField({
  label,
  ariaLabel,
  value,
  onChange,
  inputMode,
  placeholder,
  suggestions,
  required = false,
  tone = 'neutral',
}: {
  label: string;
  ariaLabel?: string;
  value: string;
  onChange: (value: string) => void;
  inputMode?: HTMLAttributes<HTMLInputElement>['inputMode'];
  placeholder?: string;
  suggestions?: readonly string[];
  required?: boolean;
  tone?: 'neutral' | 'success';
}) {
  const listId = suggestions?.length
    ? `input-suggestions-${(ariaLabel ?? label).replace(/\s+/g, '-').toLowerCase()}`
    : undefined;
  const inputColors =
    tone === 'success'
      ? 'border-amber-700/50 bg-slate-950 focus:border-amber-500 focus:ring-amber-500/30'
      : 'border-slate-800 bg-slate-950 focus:ring-amber-500/30';
  return (
    <FieldShell label={label} required={required} tone={tone}>
      <input
        aria-label={ariaLabel ?? label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={`${inputCls} focus:ring-1 ${inputColors}`}
        inputMode={inputMode}
        placeholder={placeholder}
        list={listId}
        required={required}
      />
      {listId ? (
        <datalist id={listId}>
          {suggestions?.map((suggestion) => (
            <option key={suggestion} value={suggestion} />
          ))}
        </datalist>
      ) : null}
    </FieldShell>
  );
}

export function TextAreaField({
  label,
  ariaLabel,
  value,
  onChange,
  placeholder,
  required,
  tone = 'neutral',
}: {
  label: string;
  ariaLabel?: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  required?: boolean;
  tone?: 'neutral' | 'success';
}) {
  const inputColors =
    tone === 'success'
      ? 'border-amber-700/50 bg-slate-950 focus:border-amber-500 focus:ring-amber-500/30'
      : 'border-slate-800 bg-slate-950 focus:ring-amber-500/30';
  return (
    <FieldShell label={label} tone={tone}>
      <textarea
        aria-label={ariaLabel ?? label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={`min-h-[92px] w-full rounded-[10px] border px-3 py-1.5 text-compact leading-5 text-slate-200 placeholder:text-slate-500 outline-none transition focus:ring-1 ${inputColors}`}
        placeholder={placeholder}
        required={required}
      />
    </FieldShell>
  );
}

export function SelectField({
  label,
  ariaLabel,
  value,
  options,
  onChange,
  disabled = false,
  required = false,
  tone = 'neutral',
}: {
  label: string;
  ariaLabel?: string;
  value: string;
  options: Array<{ value: string; label: string; disabled?: boolean }>;
  onChange: (value: string) => void;
  disabled?: boolean;
  required?: boolean;
  tone?: 'neutral' | 'success';
}) {
  const inputColors =
    tone === 'success'
      ? 'border-amber-700/50 bg-slate-950 focus:border-amber-500 focus:ring-amber-500/30'
      : 'border-slate-800 bg-slate-950 focus:ring-amber-500/30';
  return (
    <FieldShell label={label} required={required} tone={tone}>
      <select
        aria-label={ariaLabel ?? label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        required={required}
        className={`w-full rounded-[10px] border px-3 py-1.5 text-compact leading-5 text-slate-200 outline-none transition focus:ring-1 disabled:cursor-not-allowed disabled:opacity-60 ${inputColors}`}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value} disabled={option.disabled}>
            {option.label}
          </option>
        ))}
      </select>
    </FieldShell>
  );
}

export function Toggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      onClick={() => onChange(!on)}
      className={`h-6 w-11 shrink-0 rounded-full p-0.5 transition-colors ${on ? 'bg-amber-600' : 'bg-slate-700'}`}
      aria-pressed={on}
    >
      <div
        className={`h-5 w-5 rounded-full bg-white shadow-md transform transition-transform ${
          on ? 'translate-x-5' : 'translate-x-0'
        }`}
      />
    </button>
  );
}

export function PersistenceBanner() {
  return (
    <div className="rounded-[16px] bg-amber-950/30 border border-amber-700/30 p-4">
      <p className="text-compact font-extrabold text-amber-400">运行时持久化</p>
      <p className="mt-1.5 text-xs font-bold leading-5 text-amber-300/90">
        所有配置修改在运行时即时生效，并自动持久化到 `.sounds-great-ai/dog-catalog.json` 文件。重启后自动恢复，无需手动保存。
      </p>
    </div>
  );
}
