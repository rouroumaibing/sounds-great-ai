import { useEffect, useState } from 'react';
import type { DragEvent as ReactDragEvent } from 'react';
import type { BreedConfig } from '../../types/api';
import type { MemberFilterType } from '../../types';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { useSettings } from '../../hooks/useSettings';
import { useBreeds } from '../../hooks/useBreeds';
import { useLeaderConfig } from '../../hooks/useLeaderConfig';
import { settingsService } from '../../services/settingsService';
import { HubBreedEditor } from './HubBreedEditor';
import { HubLeaderEditor } from './HubLeaderEditor';
import type { SettingsMember } from '../../types';
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
// Mirrors clowder-ai's member management page (config-viewer-tabs.tsx →
// CatOverviewTab + HubMemberOverviewCard / HubCoCreatorOverviewCard /
// DefaultCatSelector / HubOverviewToolbar). The layout is reproduced 1:1:
//   toolbar (filter tabs + add) → global default dog → leader card →
//   draggable member rows → disabled section.
// The data source is the dog platform's own member/breed/leader stores.
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

// --- breed <-> member mapping (kept from the previous implementation) -------

function memberToBreed(m: SettingsMember): BreedConfig {
  return {
    id: m.id, name: m.name, display_name: m.breed, avatar: m.avatar ?? '',
    color: { primary: m.colorPrimary ?? m.color, secondary: m.colorSecondary ?? '' },
    personality: m.personality ?? '', role_description: m.roleDescription,
    team_strengths: m.teamStrengths?.join(','), mention_patterns: m.mentionPatterns ?? [],
    roles: [], default_variant_id: 'default',
    nickname: m.nickname, caution: m.caution,
    variants: [{
      id: 'default', client_id: m.clientId ?? '', default_model: m.defaultModel ?? m.model,
      mcp_support: m.mcpSupport ?? false, account_ref: m.accountRef, provider: m.provider,
      session_chain: m.sessionChain, strategy: m.strategy,
      cli: { command: m.cliCommand ?? '', output_format: m.outputFormat ?? '', default_args: m.defaultArgs?.split(' '), effort: m.effort },
      context_budget: { max_prompt_tokens: m.maxPromptTokens, max_context_tokens: m.maxContextTokens, max_messages: m.maxMessages },
    }],
    source: 'user', enabled: m.enabled,
  };
}

function breedToMemberUpdates(b: BreedConfig): Partial<SettingsMember> {
  const v = b.variants[0];
  return {
    name: b.name, breed: b.display_name, color: b.color?.primary ?? '',
    model: v?.default_model ?? '', provider: v?.provider ?? '',
    sessionChain: v?.session_chain ?? false, enabled: b.enabled,
    clientId: v?.client_id, accountRef: v?.account_ref, defaultModel: v?.default_model,
    nickname: b.nickname, avatar: b.avatar, colorPrimary: b.color?.primary, colorSecondary: b.color?.secondary,
    mentionPatterns: b.mention_patterns, personality: b.personality, roleDescription: b.role_description,
    teamStrengths: b.team_strengths?.split(',').filter(Boolean), caution: b.caution,
    cliCommand: v?.cli.command, outputFormat: v?.cli.output_format, defaultArgs: v?.cli.default_args?.join(' '),
    effort: v?.cli.effort, maxPromptTokens: v?.context_budget?.max_prompt_tokens,
    maxContextTokens: v?.context_budget?.max_context_tokens, maxMessages: v?.context_budget?.max_messages,
    mcpSupport: v?.mcp_support, strategy: v?.strategy,
  };
}

const MEMBER_FILTER_TABS = [
  { key: 'all', label: '全部' },
  { key: 'enabled', label: '已启用' },
  { key: 'disabled', label: '已停用' },
  { key: 'oauth', label: 'CLI（OAuth）' },
  { key: 'config', label: 'CLI（配置）' },
];

// ---------------------------------------------------------------------------
// Sub-components (mirror clowder's Hub*OverviewCard family)
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
          </span>
        </>
      }
      badges={<SettingsBadge tone={m.enabled ? 'emerald' : 'slate'}>{m.enabled ? '已启用' : '已停用'}</SettingsBadge>}
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
}: {
  breeds: BreedConfig[];
  currentDefaultDogId: string;
  onSelect: (id: string) => void;
  isLoading?: boolean;
  fetchError?: boolean;
  saveError?: string | null;
  onRetry?: () => void;
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
        </div>
      </div>
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
  const globalDefaultDog = useAppStore((s) => s.globalDefaultDog);
  const setGlobalDefaultDog = useAppStore((s) => s.setGlobalDefaultDog);
  const showAddMemberModal = useAppStore((s) => s.showAddMemberModal);
  const setShowAddMemberModal = useAppStore((s) => s.setShowAddMemberModal);
  const showToast = useAppStore((s) => s.showToast);

  const { members, loading, addMember, toggleMemberEnabled, deleteMember, refetch } = useSettings();
  const { breeds, error: breedsError, refetch: refetchBreeds } = useBreeds();
  const { leader, updateLeader } = useLeaderConfig();

  const [editingMember, setEditingMember] = useState<SettingsMember | null>(null);
  const [confirmDisable, setConfirmDisable] = useState<SettingsMember | null>(null);
  const [showLeaderEditor, setShowLeaderEditor] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [localMembers, setLocalMembers] = useState<SettingsMember[]>(members);
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dragError, setDragError] = useState<string | null>(null);

  useEffect(() => { setLocalMembers(members); }, [members]);

  useEffect(() => {
    if (!globalDefaultDog && breeds.length > 0) {
      setGlobalDefaultDog(breeds[0].id);
    }
  }, [globalDefaultDog, breeds, setGlobalDefaultDog]);

  const enabledMembers = localMembers.filter((m) => m.enabled);
  const disabledMembers = localMembers.filter((m) => !m.enabled);

  const handleSaveBreed = async (breed: BreedConfig) => {
    try {
      const updates = breedToMemberUpdates(breed);
      if (editingMember) {
        await settingsService.updateMember(editingMember.id, updates);
      } else {
        await addMember({
          ...updates, icon: 'fa-solid fa-dog', handle: `@${breed.name}`, type: 'CLI (OAuth)',
        } as Omit<SettingsMember, 'id'>);
      }
      await refetch();
      setShowAddMemberModal(false);
      setEditingMember(null);
      showToast({ message: t('members.saved'), type: 'success' });
    } catch {
      showToast({ message: t('members.saveFailed'), type: 'error' });
    }
  };

  const handleToggleEnabled = async (m: SettingsMember) => {
    if (m.enabled) {
      const dependents = members.filter((x) => x.id !== m.id && x.breed === m.breed && x.enabled);
      if (dependents.length > 0) {
        setConfirmDisable(m);
        return;
      }
    }
    setTogglingId(m.id);
    await toggleMemberEnabled(m.id, !m.enabled);
    setTogglingId(null);
  };

  const handleDelete = (m: SettingsMember) => {
    if (!window.confirm(t('members.confirmDelete').replace('{name}', m.name))) return;
    deleteMember(m.id);
  };

  const handleDefaultDogChange = async (id: string) => {
    setSaveError(null);
    const target = members.find((x) => x.id === id);
    try {
      if (target && !target.enabled) await toggleMemberEnabled(id, true);
      setGlobalDefaultDog(id);
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
    setLocalMembers((prev) => {
      const currentIds = prev.filter((m) => m.enabled).map((m) => m.id);
      const next = reorderIds(currentIds, srcId, target.id);
      if (next.length === 0) return prev;
      const reorderedEnabled = next.map((id) => prev.find((m) => m.id === id)!).filter(Boolean);
      const disabled = prev.filter((m) => !m.enabled);
      return [...reorderedEnabled, ...disabled];
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
        currentDefaultDogId={globalDefaultDog}
        onSelect={handleDefaultDogChange}
        isLoading={loading}
        fetchError={Boolean(breedsError)}
        saveError={saveError}
        onRetry={() => refetchBreeds()}
      />

      {/* leader / owner card (clowder's HubCoCreatorOverviewCard) */}
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
              onEdit={(mm) => setEditingMember(mm)}
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
      {loading && <SettingsStatusStrip tone="muted">{t('common.loading')}</SettingsStatusStrip>}
      {!loading && localMembers.length === 0 && (
        <SettingsStatusStrip tone="muted">{t('members.notFound')}</SettingsStatusStrip>
      )}

      {/* drag hint */}
      <SettingsStatusStrip tone="muted">{t('members.dragHint')}</SettingsStatusStrip>

      {/* disabled members */}
      {showDisabled && disabledMembers.length > 0 && (
        <div className="space-y-3">
          <SettingsStatusStrip tone="muted">{t('members.disabledSection')}</SettingsStatusStrip>
          {disabledMembers.map((m) => (
            <MemberOverviewCard
              key={m.id}
              member={m}
              onEdit={(mm) => setEditingMember(mm)}
              onToggle={handleToggleEnabled}
              onDelete={handleDelete}
              toggling={togglingId === m.id}
            />
          ))}
        </div>
      )}

      {/* modals */}
      {showAddMemberModal && <HubBreedEditor onSave={handleSaveBreed} onClose={() => setShowAddMemberModal(false)} />}
      {editingMember && <HubBreedEditor breed={memberToBreed(editingMember)} onSave={handleSaveBreed} onClose={() => setEditingMember(null)} />}
      {showLeaderEditor && <HubLeaderEditor leader={leader} onSave={updateLeader} onClose={() => setShowLeaderEditor(false)} />}

      {/* confirm disable (dependency check) */}
      {confirmDisable && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-sm rounded-xl border border-slate-800 bg-slate-900 p-6 shadow-2xl">
            <h3 className="text-sm font-bold text-amber-300">{t('members.disableCheck')}</h3>
            <p className="mt-2 text-xs text-slate-400">{t('members.disableWarning').replace('{breed}', confirmDisable.breed)}</p>
            <div className="mt-4 flex justify-end space-x-2">
              <button onClick={() => setConfirmDisable(null)} className="rounded-xl bg-slate-800 px-4 py-2 text-xs text-slate-300 hover:bg-slate-700">{t('common.cancel')}</button>
              <button
                onClick={() => {
                  toggleMemberEnabled(confirmDisable.id, false);
                  setConfirmDisable(null);
                }}
                className="rounded-xl bg-amber-600 px-4 py-2 text-xs font-semibold text-white hover:bg-amber-500"
              >
                {t('members.confirmDisable')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
