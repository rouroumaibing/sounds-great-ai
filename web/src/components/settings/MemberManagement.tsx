import { useEffect, useMemo, useState } from 'react';
import type { DragEvent as ReactDragEvent } from 'react';
import type { BreedConfig, RosterEntry } from '../../types/api';
import type { MemberFilterType, SettingsMember } from '../../types';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { useSettings } from '../../hooks/useSettings';
import { useBreeds } from '../../hooks/useBreeds';
import { useLeaderConfig } from '../../hooks/useLeaderConfig';
import { settingsService } from '../../services/settingsService';
import { HubBreedEditor } from './HubBreedEditor';
import { HubLeaderEditor } from './HubLeaderEditor';
import {
  SettingsBadge,
  SettingsRow,
  SettingsPrimaryButton,
  SettingsStatusStrip,
  SettingsFilterTabs,
  SettingsText,
  SettingsIconButton,
  SettingsToggleSwitch,
} from './primitives';

// ---------------------------------------------------------------------------
// 成员管理页布局：
//   toolbar (filter tabs + add) → global default dog → leader card →
//   draggable member rows → disabled section.
// 数据源：犬平台自身的 breeds（合并后成员）+ roster（运行时名册元信息）。
//   - 列表 = breeds（合并 template + catalog）按 roster 投影 enabled。
//   - 启用/停用写 roster.available（唯一开关）。
//   - 默认犬 / 排序走 default-breed / breed-order 端点，落盘持久化。
// ---------------------------------------------------------------------------

const RUNTIME_LABELS: Record<string, string> = {
  claude: 'Claude',
  codex: 'Codex',
  gemini: 'Gemini',
  kimi: 'Kimi',
  opencode: 'OpenCode',
};

const OAUTH_REFS = new Set(['claude', 'codex', 'gemini', 'kimi', 'opencode']);

function runtimeLabel(clientId?: string): string {
  if (!clientId) return '';
  return RUNTIME_LABELS[clientId] ?? clientId;
}

function accountSummary(m: SettingsMember): string {
  const ref = m.accountRef?.trim() ?? '';
  if (!ref) return '';
  if (OAUTH_REFS.has(ref.toLowerCase())) return 'CLI（OAuth）账号';
  return `CLI（配置） · ${ref}`;
}

function memberMetaSummary(m: SettingsMember): string {
  const rt = runtimeLabel(m.clientId) || m.provider || '';
  return [rt, m.defaultModel ?? m.model, accountSummary(m)].filter(Boolean).join(' · ');
}

function isOauthMember(m: SettingsMember): boolean {
  const ref = m.accountRef?.trim().toLowerCase() ?? '';
  return ref !== '' && OAUTH_REFS.has(ref);
}

function formatMentionPreview(patterns?: string[], max = 3): string {
  if (!patterns || patterns.length === 0) return '';
  const visible = patterns.slice(0, max);
  const rest = patterns.length - visible.length;
  return rest > 0 ? `${visible.join(' ')}  +${rest}` : visible.join(' ');
}

// BreedConfig（合并后成员）+ roster 投影 → UI 模型 SettingsMember。
// roster.available 为启用唯一真相；缺省回退 breed.enabled。
function breedToSettingsMember(b: BreedConfig, r?: RosterEntry): SettingsMember {
  const variant = b.variants?.find((v) => v.id === b.default_variant_id) ?? b.variants?.[0];
  const available = r?.available ?? b.enabled;
  const ref = (variant?.account_ref ?? '').trim().toLowerCase();
  return {
    id: b.id,
    name: b.name,
    breed: b.display_name,
    color: b.color?.primary ?? '#4A90D9',
    icon: 'fa-solid fa-dog',
    model: variant?.default_model ?? '',
    handle: `@${b.id}`,
    sessionChain: Boolean(variant?.session_chain),
    enabled: available,
    provider: variant?.provider ?? '',
    type: OAUTH_REFS.has(ref) ? 'CLI (OAuth)' : 'CLI (config)',
    clientId: variant?.client_id,
    accountRef: variant?.account_ref,
    defaultModel: variant?.default_model,
    nickname: b.nickname,
    avatar: b.avatar,
    colorPrimary: b.color?.primary,
    colorSecondary: b.color?.secondary,
    mentionPatterns: b.mention_patterns,
    personality: b.personality,
    roleDescription: b.role_description,
    teamStrengths: b.team_strengths ? b.team_strengths.split(',').filter(Boolean) : [],
    caution: b.caution,
    cliCommand: variant?.cli?.command,
    outputFormat: variant?.cli?.output_format,
    defaultArgs: variant?.cli?.default_args?.join(' '),
    effort: variant?.cli?.effort,
    maxPromptTokens: variant?.context_budget?.max_prompt_tokens,
    maxContextTokens: variant?.context_budget?.max_context_tokens,
    maxMessages: variant?.context_budget?.max_messages,
    mcpSupport: variant?.mcp_support,
    strategy: variant?.strategy,
    credentialReady: r?.credential_ready ?? false,
  };
}

const MEMBER_FILTER_TABS = [
  { key: 'all', label: '全部' },
  { key: 'enabled', label: '已启用' },
  { key: 'disabled', label: '已停用' },
  { key: 'oauth', label: 'CLI（OAuth）' },
  { key: 'config', label: 'CLI（配置）' },
];

// 三态派生（决策 D2）：有效可用性 = enabled ∧ credential_ready。
//  - ready：已启用且密钥/CLI 就绪
//  - needsConfig：已启用但缺密钥或 CLI
//  - disabled：已停用
type MemberStatus = { label: string; tone: 'emerald' | 'amber' | 'slate'; hint?: string };

function memberStatus(m: SettingsMember): MemberStatus {
  if (!m.enabled) return { label: '已停用', tone: 'slate' };
  if (m.credentialReady) return { label: '就绪', tone: 'emerald' };
  return { label: '待配置', tone: 'amber', hint: '缺密钥或 CLI' };
}

// ---------------------------------------------------------------------------
// Sub-components (overview cards)
// ---------------------------------------------------------------------------

function LeaderOverviewCard({ leader, onClick }: { leader: import('../../hooks/useLeaderConfig').LeaderConfig; onClick: () => void }) {
  const { t } = useI18n();
  const primary = leader.colorPrimary ?? '#6366f1';
  const aliases = leader.aliases?.join(' · ') || 'Owner';
  const mentions = leader.mentionPatterns?.join(' ') || '';
  const initial = (leader.name?.[0] ?? 'Y').toUpperCase();

  return (
    <SettingsRow
      icon={
        <div className="flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl text-xs font-bold text-white shadow" style={{ backgroundColor: primary }}>
          {leader.avatar && leader.avatar.length <= 2 ? leader.avatar : initial}
        </div>
      }
      title={leader.name || 'You'}
      badges={
        <SettingsBadge tone="amber" className="inline-flex items-center gap-1">
          <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
          </svg>
          Owner
        </SettingsBadge>
      }
      meta={
        <>
          <span>{t('members.aliases')} {aliases} · {t('members.ownerNote')}</span>
          {mentions && (
            <span className="mt-0.5 block" style={{ color: primary }}>{mentions}</span>
          )}
        </>
      }
      onClick={onClick}
    />
  );
}

function MemberOverviewCard({
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
}: {
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
}) {
  const title = [m.breed || m.name, m.nickname].filter(Boolean).join(' · ');
  const mentionPreview = formatMentionPreview(m.mentionPatterns);
  const status = memberStatus(m);
  const connectionBadge = isOauthMember(m) ? (
    <SettingsBadge tone="blue" size="xxs" className="ml-1.5 inline-block">OAuth</SettingsBadge>
  ) : m.accountRef ? (
    <SettingsBadge tone="slate" size="xxs" className="ml-1.5 inline-block">{m.accountRef}</SettingsBadge>
  ) : m.provider ? (
    <SettingsBadge tone="slate" size="xxs" className="ml-1.5 inline-block">{m.provider}</SettingsBadge>
  ) : null;

  return (
    <SettingsRow
      data-testid={`member-card-${m.id}`}
      draggable={draggable}
      onDragStart={draggable ? (e) => onDragStart?.(m, e) : undefined}
      onDragOver={draggable ? (e) => onDragOver?.(m, e) : undefined}
      onDrop={draggable ? (e) => onDrop?.(m, e) : undefined}
      onDragEnd={draggable ? (e) => onDragEnd?.(m, e) : undefined}
      onClick={() => onEdit(m)}
      isDragging={isDragging}
      dragHandle={
        draggable ? (
          <span aria-hidden="true" title="拖动排序" className="select-none leading-none text-lg">⠿</span>
        ) : undefined
      }
      icon={
        <div className="flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl text-xs font-bold text-white shadow" style={{ backgroundColor: m.colorPrimary ?? m.color }}>
          {m.avatar && m.avatar.length <= 2 ? m.avatar : <i className={m.icon}></i>}
        </div>
      }
      title={title}
      meta={
        <>
          <span>
            <SettingsText tone="muted" className="mr-1.5 font-mono text-micro">{m.id}</SettingsText>
            {memberMetaSummary(m)}
            {connectionBadge}
          </span>
          <span className="mt-0.5 flex flex-wrap items-center gap-2">
            {mentionPreview && <SettingsText tone="purple">{mentionPreview}</SettingsText>}
            <SettingsBadge tone={m.sessionChain ? 'emerald' : 'slate'} size="xxs" className="inline-block">
              {m.sessionChain ? 'Session Chain 已开启' : 'Session Chain 未开启'}
            </SettingsBadge>
            {status.hint && (
              <SettingsText tone="muted" className="text-micro">{status.hint}</SettingsText>
            )}
          </span>
        </>
      }
      badges={<SettingsBadge tone={status.tone}>{status.label}</SettingsBadge>}
      actions={
        <>
          <SettingsToggleSwitch
            enabled={m.enabled}
            busy={toggling}
            onClick={(e) => {
              e.stopPropagation();
              onToggle(m);
            }}
            title={m.enabled ? '停用成员' : '启用成员'}
            ariaLabel={m.enabled ? '停用成员' : '启用成员'}
          />
          <SettingsIconButton
            tone="danger"
            onClick={(e) => {
              e.stopPropagation();
              onDelete(m);
            }}
            title="删除成员"
            aria-label="删除成员"
          >
            <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2} aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
            </svg>
          </SettingsIconButton>
        </>
      }
      tone={m.enabled ? 'active' : 'inactive'}
    />
  );
}

function DefaultDogSelector({
  breeds,
  currentDefaultDogId,
  onSelect,
  isLoading,
  fetchError,
  saveError,
  onRetry,
  disabled,
}: {
  breeds: BreedConfig[];
  currentDefaultDogId: string;
  onSelect: (id: string) => void;
  isLoading?: boolean;
  fetchError?: boolean;
  saveError?: string | null;
  onRetry?: () => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  const currentBreed = breeds.find((b) => b.id === currentDefaultDogId);
  const valueInList = currentDefaultDogId && breeds.some((b) => b.id === currentDefaultDogId);
  const dotColor = currentBreed?.color?.primary ?? '#6366f1';

  return (
    <div className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-4 shadow-sm">
      {fetchError && (
        <div className="mb-3 flex items-center gap-2 text-xs text-amber-300">
          <span>加载失败，当前默认犬未知</span>
          {onRetry && (
            <button type="button" onClick={onRetry} className="font-medium underline hover:text-amber-200">
              重试
            </button>
          )}
        </div>
      )}
      {saveError && (
        <div className="mb-3 rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-300">{saveError}</div>
      )}
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-bold text-slate-100">{t('members.globalDefault')}</h3>
          <p className="mt-0.5 text-xs text-slate-400">{t('members.globalDefaultHint')}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {disabled ? (
            <span className="text-xs text-slate-500">{t('members.noDefaultHint')}</span>
          ) : (
            <>
              {currentBreed && (
                <span className="h-3 w-3 flex-shrink-0 rounded-full" style={{ backgroundColor: dotColor }} />
              )}
              <select
                value={valueInList ? currentDefaultDogId : ''}
                disabled={isLoading}
                onChange={(e) => onSelect(e.target.value)}
                className={`h-[34px] w-[220px] rounded-[10px] border border-slate-700 bg-slate-950 px-3 py-1 text-xs text-slate-200 focus:outline-none focus:ring-1 focus:ring-amber-500 ${isLoading ? 'cursor-wait opacity-50' : 'cursor-pointer'}`}
              >
                {!valueInList && (
                  <option value="" disabled>
                    {currentDefaultDogId ? t('members.selectDefault') : t('members.selectDefault')}
                  </option>
                )}
                {breeds.map((b) => (
                  <option key={b.id} value={b.id}>{b.name} · {b.display_name}</option>
                ))}
              </select>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

// 首启空 catalog（决策 D1）：0 成员时不崩溃，给出强引导空态。
function EmptyMembersState({ onAdd }: { onAdd: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-col items-center rounded-xl border border-dashed border-slate-800 bg-slate-900/40 px-6 py-12 text-center">
      <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-800 text-lg text-amber-300">
        <i className="fa-solid fa-dog" aria-hidden="true" />
      </div>
      <h3 className="text-sm font-bold text-slate-100">{t('members.emptyTitle')}</h3>
      <p className="mt-1.5 max-w-md text-xs leading-5 text-slate-400">
        {t('members.emptyHint')}
      </p>
      <button
        type="button"
        onClick={onAdd}
        className="mt-5 h-9 rounded-[10px] bg-amber-600 px-4 text-xs font-extrabold text-white transition hover:bg-amber-500"
      >
        + {t('members.emptyAdd')}
      </button>
      <p className="mt-2 text-micro text-slate-500">{t('members.emptyFromTemplate')}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

function reorderIds(ids: string[], srcId: string, targetId: string): string[] {
  const withoutSrc = ids.filter((id) => id !== srcId);
  const targetIdx = withoutSrc.indexOf(targetId);
  if (targetIdx < 0) return ids;
  return [...withoutSrc.slice(0, targetIdx), srcId, ...withoutSrc.slice(targetIdx)];
}

export function MemberManagement() {
  const { t } = useI18n();
  const memberFilter = useAppStore((s) => s.memberFilter);
  const setMemberFilter = useAppStore((s) => s.setMemberFilter);
  const showAddMemberModal = useAppStore((s) => s.showAddMemberModal);
  const setShowAddMemberModal = useAppStore((s) => s.setShowAddMemberModal);
  const showToast = useAppStore((s) => s.showToast);

  const { roster, loading: rosterLoading, updateRosterEntry, refetch: refetchSettings } = useSettings();
  const {
    breeds,
    loading: breedsLoading,
    error: breedsError,
    refetch: refetchBreeds,
    deleteBreed,
    createBreed,
    updateBreed,
  } = useBreeds();
  const { leader, updateLeader } = useLeaderConfig();

  const members = useMemo<SettingsMember[]>(
    () => breeds.map((b) => breedToSettingsMember(b, roster[b.id])),
    [breeds, roster],
  );

  const [editingBreed, setEditingBreed] = useState<BreedConfig | null>(null);
  const [showLeaderEditor, setShowLeaderEditor] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [localMembers, setLocalMembers] = useState<SettingsMember[]>(members);
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dragError, setDragError] = useState<string | null>(null);
  const [defaultBreedId, setDefaultBreedId] = useState('');

  useEffect(() => { setLocalMembers(members); }, [members]);

  useEffect(() => {
    settingsService
      .getDefaultBreed()
      .then((d) => setDefaultBreedId(d.breed_id))
      .catch(() => {});
  }, []);

  const enabledMembers = localMembers.filter((m) => m.enabled);
  const disabledMembers = localMembers.filter((m) => !m.enabled);

  const handleSaveBreed = async (breed: BreedConfig) => {
    try {
      if (editingBreed) {
        await updateBreed(breed.id, breed);
      } else {
        await createBreed(breed);
      }
      await Promise.all([refetchBreeds(), refetchSettings()]);
      setShowAddMemberModal(false);
      setEditingBreed(null);
      showToast({ message: t('members.saved'), type: 'success' });
    } catch {
      showToast({ message: t('members.saveFailed'), type: 'error' });
    }
  };

  const handleToggleEnabled = async (m: SettingsMember) => {
    setTogglingId(m.id);
    try {
      await updateRosterEntry(m.id, { available: !m.enabled });
      await Promise.all([refetchBreeds(), refetchSettings()]);
    } catch {
      // 错误 toast 已由 updateRosterEntry 提示
    } finally {
      setTogglingId(null);
    }
  };

  const handleDelete = (m: SettingsMember) => {
    if (!window.confirm(t('members.confirmDelete').replace('{name}', m.name))) return;
    deleteBreed(m.id)
      .then(() => Promise.all([refetchBreeds(), refetchSettings()]))
      .catch(() => {});
  };

  const handleDefaultDogChange = async (id: string) => {
    setSaveError(null);
    try {
      const breed = breeds.find((b) => b.id === id);
      const isAvailable = roster[id]?.available ?? breed?.enabled ?? false;
      if (!isAvailable) {
        await updateRosterEntry(id, { available: true });
      }
      await settingsService.setDefaultBreed(id);
      setDefaultBreedId(id);
    } catch {
      setSaveError(t('members.saveDefaultFailed'));
    }
  };

  const onDragStart = (_m: SettingsMember, e: ReactDragEvent<HTMLElement>) => {
    setDraggedId(_m.id);
    e.dataTransfer.effectAllowed = 'move';
  };
  const onDragOver = (_m: SettingsMember, e: ReactDragEvent<HTMLElement>) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };
  const onDrop = (target: SettingsMember, e: ReactDragEvent<HTMLElement>) => {
    e.preventDefault();
    const srcId = draggedId ?? e.dataTransfer.getData('text/plain') ?? '';
    setDraggedId(null);
    if (!srcId || srcId === target.id) return;
    const currentIds = localMembers.filter((m) => m.enabled).map((m) => m.id);
    const next = reorderIds(currentIds, srcId, target.id);
    if (next.length === 0) return;
    // 乐观更新：启用段按拖拽重排，停用段保持原位。
    setLocalMembers((prev) => {
      const reorderedEnabled = next.map((id) => prev.find((m) => m.id === id)!).filter(Boolean);
      const disabled = prev.filter((m) => !m.enabled);
      return [...reorderedEnabled, ...disabled];
    });
    const disabledIds = localMembers.filter((m) => !m.enabled).map((m) => m.id);
    const newOrder = [...next, ...disabledIds];
    settingsService
      .setBreedOrder(newOrder)
      .then(() => Promise.all([refetchBreeds(), refetchSettings()]))
      .catch(() => {
        setDragError(t('members.reorderFailed'));
        setLocalMembers(members); // 回滚到服务端顺序
      });
    setDragError(null);
  };
  const onDragEnd = () => setDraggedId(null);

  const showEnabled = memberFilter === 'all' || memberFilter === 'enabled' || memberFilter === 'oauth' || memberFilter === 'config';
  const showDisabled = memberFilter === 'all' || memberFilter === 'disabled';
  const visibleEnabled = enabledMembers.filter((m) => {
    if (memberFilter === 'oauth') return isOauthMember(m);
    if (memberFilter === 'config') return !isOauthMember(m);
    return true;
  });

  return (
    <div className="max-w-5xl mx-auto w-full space-y-4">
      {/* toolbar: filter tabs + add member */}
      <div className="flex items-center justify-between gap-3">
        <SettingsFilterTabs
          tabs={MEMBER_FILTER_TABS}
          activeKey={memberFilter}
          onTabChange={(key) => setMemberFilter(key as MemberFilterType)}
        />
        <SettingsPrimaryButton onClick={() => setShowAddMemberModal(true)}>
          + {t('members.add')}
        </SettingsPrimaryButton>
      </div>

      {/* global default dog */}
      <DefaultDogSelector
        breeds={breeds}
        currentDefaultDogId={defaultBreedId}
        onSelect={handleDefaultDogChange}
        isLoading={breedsLoading || rosterLoading}
        fetchError={Boolean(breedsError)}
        saveError={saveError}
        onRetry={() => refetchBreeds()}
        disabled={breeds.length === 0}
      />

      {/* leader / owner card */}
      {leader && <LeaderOverviewCard leader={leader} onClick={() => setShowLeaderEditor(true)} />}

      {/* drag error */}
      {dragError && <SettingsStatusStrip tone="error">{dragError}</SettingsStatusStrip>}

      {/* enabled members */}
      {showEnabled && (
        <div className="space-y-3">
          {visibleEnabled.map((m) => (
            <MemberOverviewCard
              key={m.id}
              member={m}
              onEdit={(mm) => setEditingBreed(breeds.find((b) => b.id === mm.id) ?? null)}
              onToggle={handleToggleEnabled}
              onDelete={handleDelete}
              toggling={togglingId === m.id}
              draggable
              onDragStart={onDragStart}
              onDragOver={onDragOver}
              onDrop={onDrop}
              onDragEnd={onDragEnd}
              isDragging={draggedId === m.id}
            />
          ))}
        </div>
      )}

      {/* loading / empty states */}
      {breedsLoading && <SettingsStatusStrip tone="muted">{t('common.loading')}</SettingsStatusStrip>}
      {!breedsLoading && localMembers.length === 0 && (
        <EmptyMembersState onAdd={() => setShowAddMemberModal(true)} />
      )}

      {/* drag hint — 仅在有成员时显示 */}
      {localMembers.length > 0 && (
        <SettingsStatusStrip tone="muted">{t('members.dragHint')}</SettingsStatusStrip>
      )}

      {/* disabled members */}
      {showDisabled && disabledMembers.length > 0 && (
        <div className="space-y-3">
          <SettingsStatusStrip tone="muted">{t('members.disabledSection')}</SettingsStatusStrip>
          {disabledMembers.map((m) => (
            <MemberOverviewCard
              key={m.id}
              member={m}
              onEdit={(mm) => setEditingBreed(breeds.find((b) => b.id === mm.id) ?? null)}
              onToggle={handleToggleEnabled}
              onDelete={handleDelete}
              toggling={togglingId === m.id}
            />
          ))}
        </div>
      )}

      {/* modals */}
      {showAddMemberModal && <HubBreedEditor onSave={handleSaveBreed} onClose={() => setShowAddMemberModal(false)} />}
      {editingBreed && <HubBreedEditor breed={editingBreed} onSave={handleSaveBreed} onClose={() => setEditingBreed(null)} />}
      {showLeaderEditor && <HubLeaderEditor leader={leader} onSave={updateLeader} onClose={() => setShowLeaderEditor(false)} />}
    </div>
  );
}
