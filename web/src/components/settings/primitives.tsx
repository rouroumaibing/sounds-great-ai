import type { CSSProperties, DragEvent, ElementType, KeyboardEvent, MouseEvent, ReactNode } from 'react';
import clsx from 'clsx';

// ---------------------------------------------------------------------------
// Dark-themed equivalents of clowder-ai's settings primitives.
//
// clowder ships a warm "cafe / console" token system (e.g. var(--cafe-accent),
// var(--console-card-bg)). The dog platform is a dark slate + amber app, so we
// map those tokens onto the existing slate/amber palette. This keeps the
// settings area visually consistent with the rest of the app while reproducing
// clowder's component vocabulary (page header, sections, collapsible cards,
// badges, coverage bar, empty state).
// ---------------------------------------------------------------------------

type BadgeTone = 'emerald' | 'amber' | 'slate' | 'red' | 'purple' | 'blue';

const badgeToneStyles: Record<BadgeTone, string> = {
  emerald: 'bg-emerald-500/15 border border-emerald-500/30 text-emerald-300',
  amber: 'bg-amber-500/15 border border-amber-500/30 text-amber-300',
  slate: 'bg-slate-800 border border-slate-700 text-slate-400',
  red: 'bg-rose-500/15 border border-rose-500/30 text-rose-300',
  purple: 'bg-purple-500/15 border border-purple-500/30 text-purple-300',
  blue: 'bg-sky-500/15 border border-sky-500/30 text-sky-300',
};

export function SettingsBadge({
  tone,
  size = 'xs',
  as = 'span',
  onClick,
  disabled,
  title,
  children,
  className,
}: {
  tone: BadgeTone;
  size?: 'xs' | 'xxs';
  as?: 'span' | 'button';
  onClick?: (e: MouseEvent) => void;
  disabled?: boolean;
  title?: string;
  children: ReactNode;
  className?: string;
}) {
  const sizeClass = size === 'xxs' ? 'px-1.5 py-0.5 text-[10px]' : 'px-2.5 py-0.5 text-[11px]';
  const base = `rounded-full font-semibold ${sizeClass} ${badgeToneStyles[tone]} ${className ?? ''}`;
  if (as === 'button') {
    return (
      <button
        type="button"
        className={`${base} transition disabled:cursor-default disabled:opacity-50`}
        onClick={(e) => {
          e.stopPropagation();
          onClick?.(e);
        }}
        disabled={disabled}
        title={title}
      >
        {children}
      </button>
    );
  }
  return (
    <span className={base} title={title}>
      {children}
    </span>
  );
}

export function SettingsPageHeader({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:gap-4">
      <div className="min-w-0 flex-1 space-y-1">
        <h2 className="text-2xl font-extrabold text-slate-100">{title}</h2>
        <p className="text-sm leading-tight text-slate-400">{subtitle}</p>
      </div>
      {children && <div className="flex shrink-0 items-center justify-start sm:justify-end">{children}</div>}
    </div>
  );
}

export function SettingsSection({
  title,
  description,
  badge,
  children,
}: {
  title: string;
  description?: string;
  badge?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <section className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-[18px]">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-base font-semibold text-slate-100">{title}</h3>
          {description && <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-400">{description}</p>}
        </div>
        {badge}
      </div>
      {children && <div className="mt-3">{children}</div>}
    </section>
  );
}

export function SettingsCard({
  variant = 'default',
  onClick,
  className,
  style,
  children,
}: {
  variant?: 'default' | 'highlight';
  onClick?: () => void;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}) {
  const variantClass =
    variant === 'highlight' ? 'bg-amber-500/5 border-amber-500/20' : 'bg-slate-900/60 border-slate-800/80';
  return (
    <div
      className={clsx(
        'rounded-xl border p-4 shadow-sm transition',
        variantClass,
        onClick && 'cursor-pointer hover:border-slate-700',
        className,
      )}
      onClick={onClick}
      style={style}
      role={onClick ? 'button' : undefined}
    >
      {children}
    </div>
  );
}

export function SettingsCollapsibleCard({
  title,
  count,
  collapsed,
  onToggle,
  children,
}: {
  title: string;
  count?: number;
  collapsed: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-slate-800/80 bg-slate-900/60">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center gap-2.5 px-4 py-3 text-left transition-colors hover:bg-slate-800/40"
      >
        <span className={clsx('text-xs text-slate-500 transition-transform', collapsed && '-rotate-90')}>▾</span>
        <span className="text-sm font-semibold text-slate-100">{title}</span>
        {count !== undefined && <SettingsBadge tone="slate" size="xxs">{count}</SettingsBadge>}
      </button>
      {!collapsed && <div className="divide-y divide-slate-800/60 px-4 pb-2">{children}</div>}
    </div>
  );
}

export function SettingsEmptyState({
  icon,
  title,
  description,
}: {
  icon?: ReactNode;
  title: string;
  description?: string;
}) {
  return (
    <SettingsCard className="flex flex-col items-center justify-center px-8 py-16 text-center">
      {icon}
      <p className="font-semibold text-slate-200">{title}</p>
      {description && <p className="mt-1 text-sm text-slate-400">{description}</p>}
    </SettingsCard>
  );
}

// ---------------------------------------------------------------------------
// The primitives below mirror clowder-ai's settings/primitives vocabulary
// (SettingsRow, SettingsPrimaryButton, SettingsStatusStrip, SettingsFilterTabs,
// SettingsText, SettingsIconButton, SettingsToggleSwitch) on the dog platform's
// dark slate + amber palette. They let the member management page reproduce
// clowder's exact layout (toolbar → default selector → leader card → member
// rows → disabled section) without pulling in clowder's cafe/console tokens.
// ---------------------------------------------------------------------------

type RowTone = 'default' | 'active' | 'inactive';

const rowToneClasses: Record<RowTone, string> = {
  default: 'bg-slate-900/60 border border-slate-800/80',
  active: 'bg-slate-900/60 border border-slate-800/80',
  inactive: 'bg-slate-900/30 border border-slate-800/60',
};

export function SettingsRow({
  icon,
  title,
  meta,
  badges,
  actions,
  dragHandle,
  children,
  className,
  tone = 'default',
  expanded,
  onToggle,
  onClick,
  onKeyDown,
  draggable,
  isDragging,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  'data-testid': dataTestId,
}: {
  icon?: ReactNode;
  title: ReactNode;
  meta?: ReactNode;
  badges?: ReactNode;
  actions?: ReactNode;
  dragHandle?: ReactNode;
  children?: ReactNode;
  className?: string;
  tone?: RowTone;
  expanded?: boolean;
  onToggle?: () => void;
  onClick?: () => void;
  onKeyDown?: (e: KeyboardEvent<HTMLElement>) => void;
  draggable?: boolean;
  isDragging?: boolean;
  onDragStart?: (e: DragEvent<HTMLElement>) => void;
  onDragOver?: (e: DragEvent<HTMLElement>) => void;
  onDrop?: (e: DragEvent<HTMLElement>) => void;
  onDragEnd?: (e: DragEvent<HTMLElement>) => void;
  'data-testid'?: string;
}) {
  const isExpandable = onToggle !== undefined;
  const isExpanded = expanded ?? true;
  return (
    <div
      className={clsx(
        'rounded-xl px-4 py-3 shadow-sm transition',
        rowToneClasses[tone],
        onClick && 'cursor-pointer hover:border-slate-700',
        isDragging && 'opacity-40',
        className,
      )}
      onClick={onClick}
      onKeyDown={onKeyDown}
      draggable={draggable || undefined}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragEnd={onDragEnd}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      data-testid={dataTestId}
    >
      <div className="flex items-center gap-3">
        {dragHandle && <div className="shrink-0 cursor-grab select-none text-slate-500">{dragHandle}</div>}
        {icon && <div className="shrink-0">{icon}</div>}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="flex-1 truncate text-sm font-bold text-slate-100">{title}</span>
            {badges}
          </div>
          {meta && <div className="mt-0.5 break-words text-xs text-slate-400">{meta}</div>}
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        {isExpandable && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onToggle?.();
            }}
            className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-amber-300 transition-colors hover:text-slate-100"
            aria-expanded={isExpanded}
            aria-label={isExpanded ? '收起' : '展开'}
          >
            <svg
              aria-hidden="true"
              className={clsx('h-3.5 w-3.5 transition-transform', isExpanded && 'rotate-180')}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path strokeLinecap="round" strokeLinejoin="round" d="m19 9-7 7-7-7" />
            </svg>
          </button>
        )}
      </div>
      {children && isExpanded && <div className="mt-3 border-t border-slate-800/60 pt-3">{children}</div>}
    </div>
  );
}

export function SettingsPrimaryButton({
  onClick,
  disabled,
  children,
  className,
  ...rest
}: {
  onClick: () => void;
  disabled?: boolean;
  children: ReactNode;
  className?: string;
  'data-bootcamp-step'?: string;
  'data-guide-id'?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={clsx(
        'shrink-0 rounded-full bg-amber-600 px-4 py-1.5 text-xs font-semibold text-white transition hover:bg-amber-500 disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}

export function SettingsSecondaryButton({
  onClick,
  disabled,
  children,
  className,
}: {
  onClick?: () => void;
  disabled?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={clsx(
        'shrink-0 rounded-full border border-slate-700 bg-slate-800/60 px-4 py-1.5 text-xs font-semibold text-slate-200 transition hover:border-slate-600 hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
    >
      {children}
    </button>
  );
}

type StripTone = 'info' | 'success' | 'warn' | 'error' | 'muted';

const stripColors: Record<StripTone, string> = {
  info: 'text-sky-300',
  success: 'text-emerald-300',
  warn: 'text-amber-300',
  error: 'text-rose-300',
  muted: 'text-slate-500',
};

const stripBoxed: Record<StripTone, boolean> = {
  info: false,
  success: false,
  warn: false,
  error: true,
  muted: false,
};

export function SettingsStatusStrip({
  tone = 'muted',
  children,
}: {
  tone?: StripTone;
  children: ReactNode;
}) {
  return (
    <p
      className={clsx(
        'text-sm',
        stripBoxed[tone] && 'rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2',
        stripColors[tone],
      )}
    >
      {children}
    </p>
  );
}

interface FilterTab {
  key: string;
  label: string;
  count?: number;
}

export function SettingsFilterTabs({
  tabs,
  activeKey,
  onTabChange,
}: {
  tabs: FilterTab[];
  activeKey: string;
  onTabChange: (key: string) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1 text-xs">
      {tabs.map((tab) => {
        const isActive = tab.key === activeKey;
        return (
          <button
            key={tab.key}
            type="button"
            onClick={() => onTabChange(tab.key)}
            className={clsx(
              'rounded-full px-3 py-1 font-medium transition',
              isActive
                ? 'border border-amber-500/30 bg-amber-500/15 text-amber-300'
                : 'text-slate-400 hover:bg-slate-800/50 hover:text-slate-200',
            )}
          >
            {tab.label}
            {tab.count != null && (
              <span className={clsx('ml-1', isActive ? 'opacity-80' : 'text-slate-500')}>{tab.count}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

type TextVariant = 'base' | 'sm' | 'xs' | 'micro';
type TextTone = 'default' | 'secondary' | 'muted' | 'emerald' | 'amber' | 'red' | 'blue' | 'purple';

const textVariantMap: Record<TextVariant, string> = {
  base: 'text-base',
  sm: 'text-sm',
  xs: 'text-xs',
  micro: 'text-[11px]',
};

const textToneMap: Record<TextTone, string> = {
  default: 'text-slate-200',
  secondary: 'text-slate-400',
  muted: 'text-slate-500',
  emerald: 'text-emerald-300',
  amber: 'text-amber-300',
  red: 'text-rose-300',
  blue: 'text-sky-300',
  purple: 'text-purple-300',
};

export function SettingsText({
  as: Tag = 'span',
  variant = 'xs',
  tone = 'muted',
  className,
  title,
  style,
  children,
}: {
  as?: ElementType;
  variant?: TextVariant;
  tone?: TextTone;
  className?: string;
  title?: string;
  style?: CSSProperties;
  children: ReactNode;
}) {
  return (
    <Tag
      className={clsx(textVariantMap[variant], textToneMap[tone], className)}
      title={title}
      style={style}
    >
      {children}
    </Tag>
  );
}

export function SettingsIconButton({
  children,
  className,
  tone = 'neutral',
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  tone?: 'neutral' | 'danger';
}) {
  return (
    <button
      type="button"
      {...props}
      className={clsx(
        'flex h-[30px] w-[30px] items-center justify-center rounded-lg transition-colors disabled:cursor-default disabled:opacity-50',
        tone === 'danger'
          ? 'text-slate-500 hover:bg-slate-800/60 hover:text-rose-400'
          : 'text-slate-500 hover:bg-slate-800/60 hover:text-amber-300',
        className,
      )}
    >
      {children}
    </button>
  );
}

export function SettingsToggleSwitch({
  enabled,
  busy,
  onClick,
  title,
  disabled,
  ariaLabel,
}: {
  enabled: boolean;
  busy?: boolean;
  onClick: (e: MouseEvent<HTMLButtonElement>) => void;
  title?: string;
  disabled?: boolean;
  ariaLabel?: string;
}) {
  return (
    <button
      type="button"
      disabled={disabled || busy}
      onClick={onClick}
      aria-label={ariaLabel}
      aria-pressed={enabled}
      title={title ?? (enabled ? '禁用' : '启用')}
      className={clsx(
        'relative inline-flex h-[22px] w-10 shrink-0 rounded-full transition-colors disabled:cursor-default',
        disabled || busy ? 'cursor-default opacity-50' : 'cursor-pointer',
        disabled ? 'bg-slate-700' : enabled ? 'bg-amber-600' : 'bg-slate-700',
      )}
    >
      <span
        className={clsx(
          'pointer-events-none absolute top-[3px] h-4 w-4 rounded-full bg-white transition-[left]',
          enabled ? 'left-[21px]' : 'left-[3px]',
        )}
      />
    </button>
  );
}
