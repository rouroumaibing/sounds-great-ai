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
import { FilterChips } from './FilterChips';
import { HubBreedEditor } from './HubBreedEditor';
import { HubLeaderEditor } from './HubLeaderEditor';
import { MemberCard } from './MemberCard';
import type { SettingsMember } from '../../types';

  const _t3 = useI18n.getState().t;
  const memberFilterChips = [
    { id: 'all', label: _t3('members.all'), activeClass: 'bg-amber-500/20 border-amber-500/40 text-amber-300' },
    { id: 'enabled', label: _t3('members.enabled'), activeClass: 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300' },
    { id: 'disabled', label: _t3('members.disabled'), activeClass: 'bg-slate-800 border-slate-700 text-slate-300' },
  ];

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
      const dependents = members.filter(x => x.id !== m.id && x.breed === m.breed && x.enabled);
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
    try {
      await settingsService.updateMember(id, { enabled: true }).catch(() => {});
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
    if (!draggedId || draggedId === target.id) { setDraggedId(null); return; }
    setLocalMembers((prev) => {
      const enabled = prev.filter((m) => m.enabled);
      const fromIdx = enabled.findIndex((m) => m.id === draggedId);
      const toIdx = enabled.findIndex((m) => m.id === target.id);
      if (fromIdx === -1 || toIdx === -1) return prev;
      const reordered = [...enabled];
      const [moved] = reordered.splice(fromIdx, 1);
      reordered.splice(toIdx, 0, moved);
      const reorderedIds = new Set(reordered.map((m) => m.id));
      const disabled = prev.filter((m) => !m.enabled);
      const enabledOrdered = reordered.filter((m) => reorderedIds.has(m.id));
      return [...enabledOrdered, ...disabled];
    });
    setDraggedId(null);
  };
  const onDragEnd = () => setDraggedId(null);

  const showEnabled = memberFilter === 'all' || memberFilter === 'enabled';
  const showDisabled = memberFilter === 'all' || memberFilter === 'disabled';
  const selectedBreed = breeds.find((b) => b.id === globalDefaultDog);
  const breedColor = selectedBreed?.color?.primary ?? '#6366f1';

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="flex items-start justify-between border-b border-slate-800/80 pb-5">
        <div>
          <h2 className="text-2xl font-bold text-slate-100 flex items-center gap-2"><span>{t('members.title')}</span></h2>
          <p className="text-xs text-slate-400 mt-1">{t('members.desc')}</p>
        </div>
        <button onClick={() => setShowAddMemberModal(true)} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-lg shadow-amber-600/20">
          <i className="fa-solid fa-plus"></i><span>{t('members.add')}</span>
        </button>
      </div>

      <LeaderCard leader={leader} onClick={() => setShowLeaderEditor(true)} />

      <FilterChips chips={memberFilterChips} activeFilter={memberFilter} onFilterChange={(f) => setMemberFilter(f as MemberFilterType)} />

      <div className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800/80 flex items-center justify-between shadow-sm">
        <div>
          <h4 className="text-xs font-bold text-slate-200">{t('members.globalDefault')}</h4>
          <p className="text-[11px] text-slate-400 mt-0.5">{t('members.globalDefaultHint')}</p>
        </div>
        <div className="flex items-center space-x-2">
          {breedsError && (
            <span className="text-[10px] text-amber-400 flex items-center gap-1">
              {t('common.error')}
              <button onClick={() => refetchBreeds()} className="underline hover:text-amber-300">{t('common.retry')}</button>
            </span>
          )}
          {saveError && <span className="text-[10px] text-rose-400">{saveError}</span>}
          <span className="w-2 h-2 rounded-full" style={{ backgroundColor: breedColor }}></span>
          <select
            value={globalDefaultDog}
            onChange={(e) => handleDefaultDogChange(e.target.value)}
            className="bg-slate-950 border border-slate-700 text-slate-200 text-xs rounded-xl px-3 py-1.5 focus:outline-none focus:border-amber-500 font-medium"
          >
            {breeds.length === 0 && <option value="" disabled>{t('members.selectDefault')}</option>}
            {breeds.map(b => (
              <option key={b.id} value={b.id}>{b.name} · {b.display_name}</option>
            ))}
          </select>
        </div>
      </div>

      {loading && <div className="text-center text-slate-500 text-xs py-8">{t('common.loading')}</div>}

      {!loading && localMembers.length === 0 && (
        <div className="text-center text-slate-500 text-xs py-12">{t('members.notFound')}</div>
      )}

      {showEnabled && enabledMembers.length > 0 && (
        <div className="space-y-3">
          <p className="text-[11px] text-slate-500">{t('members.dragHint')}</p>
          {enabledMembers.map((m) => (
            <MemberCard
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

      {showDisabled && disabledMembers.length > 0 && (
        <div className="space-y-3">
          <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider pt-2">{t('members.disabledSection')}</h4>
          {disabledMembers.map((m) => (
            <MemberCard
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

      {showAddMemberModal && <HubBreedEditor onSave={handleSaveBreed} onClose={() => setShowAddMemberModal(false)} />}
      {editingMember && <HubBreedEditor breed={memberToBreed(editingMember)} onSave={handleSaveBreed} onClose={() => setEditingMember(null)} />}
      {showLeaderEditor && <HubLeaderEditor leader={leader} onSave={updateLeader} onClose={() => setShowLeaderEditor(false)} />}

      {confirmDisable && (
        <div className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-sm p-6 space-y-4 shadow-2xl">
            <h3 className="text-sm font-bold text-amber-300">{t('members.disableCheck')}</h3>
            <p className="text-xs text-slate-400">{t('members.disableWarning').replace('{breed}', confirmDisable.breed)}</p>
            <div className="flex justify-end space-x-2">
              <button onClick={() => setConfirmDisable(null)} className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs">{t('common.cancel')}</button>
              <button onClick={() => { toggleMemberEnabled(confirmDisable.id, false); setConfirmDisable(null); }} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold">{t('members.confirmDisable')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function LeaderCard({ leader, onClick }: { leader: import('../../hooks/useLeaderConfig').LeaderConfig; onClick: () => void }) {
  const { t } = useI18n();
  const primary = leader.colorPrimary ?? '#6366f1';
  const aliases = leader.aliases?.join(' · ') || 'Owner';
  const mentions = leader.mentionPatterns?.join(' ') || '';
  const initial = (leader.name?.[0] ?? 'Y').toUpperCase();

  return (
    <div
      onClick={onClick}
      className="p-4 rounded-2xl bg-indigo-950/40 border border-indigo-800/50 flex items-center justify-between shadow-sm cursor-pointer hover:border-indigo-700/60 transition group"
    >
      <div className="flex items-center space-x-3 min-w-0">
        <div className="w-9 h-9 rounded-xl flex items-center justify-center text-white text-xs font-bold shrink-0 shadow" style={{ backgroundColor: primary }}>
          {leader.avatar && leader.avatar.length <= 2 ? leader.avatar : initial}
        </div>
        <div className="space-y-1 truncate">
          <div className="flex items-center space-x-2">
            <span className="text-xs font-bold text-slate-100">{leader.name || 'You'}</span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded-md bg-amber-500/15 border border-amber-500/30 text-amber-300 font-semibold inline-flex items-center gap-1">
              <i className="fa-solid fa-lock text-[8px]"></i>Owner
            </span>
          </div>
          <div className="text-[11px] font-mono text-slate-400 truncate">
            {t('members.aliases')} {aliases}
          </div>
          {mentions && (
            <div className="text-[10px] font-mono text-purple-400 font-medium truncate">{mentions}</div>
          )}
        </div>
      </div>
      <div className="flex items-center shrink-0 pl-3 border-l border-slate-800/80">
        <i className="fa-solid fa-chevron-right text-xs text-slate-600 group-hover:text-slate-400 transition"></i>
      </div>
    </div>
  );
}
