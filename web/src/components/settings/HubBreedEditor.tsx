// 添加成员弹窗：适配 SG 数据层与深色主题。
// 保留对外契约：{ breed?, onSave, onClose }（MemberManagement 依赖）。
//
// 设计说明：
//  - 关闭确认用本地内联弹窗（SG 无全局确认 context）
//  - 空账号态复用 SG 已实现的 AccountAuthModal
//  - 会话策略 / Codex 运行参数无独立端点 → 已在前端分区内收敛到 variant 字段
//  - 别名唯一性通过内部加载成员列表（reservedPatterns）实现

import { useEffect, useMemo, useRef, useState } from 'react';
import type { BreedConfig } from '../../types/api';
import type { SettingsAccount } from '../../types';
import { CLIENT_IDS } from '../../constants/clientIds';
import { settingsService } from '../../services/settingsService';
import { breedService } from '../../services/breedService';
import { apiGet } from '../../services/http';
import { useI18n } from '../../store/useI18n';
import {
  autoSlug,
  buildBreedPayload,
  filterAccounts,
  initialState,
  joinTags,
  normalizeMentionPattern,
  splitMentionPatterns,
  withDefaultModelMentionPattern,
  type BreedEditorFormState,
} from './breed-editor.model';
import { IdentitySection, AccountSection, RoutingSection } from './breed-editor.sections';
import { AdvancedRuntimeSection } from './breed-editor-advanced';
import { PersistenceBanner } from './breed-editor-fields';
import { AccountAuthModal } from './AccountAuthModal';

interface HubBreedEditorProps {
  breed?: BreedConfig;
  onSave: (breed: BreedConfig) => void;
  onClose: () => void;
}

export function HubBreedEditor({ breed, onSave, onClose }: HubBreedEditorProps) {
  const { t } = useI18n();
  const isEdit = Boolean(breed);

  const [profiles, setProfiles] = useState<SettingsAccount[]>([]);
  const [members, setMembers] = useState<BreedConfig[]>([]);
  const [loadingProfiles, setLoadingProfiles] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, boolean>>({});
  const [form, setForm] = useState<BreedEditorFormState>(() => initialState(breed));
  const [templates, setTemplates] = useState<BreedConfig[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>('custom');
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const [profilesVersion, setProfilesVersion] = useState(0);
  const pendingProfileIdRef = useRef<string | null>(null);

  const availableProfiles = useMemo(() => filterAccounts(form.clientId, profiles), [form.clientId, profiles]);
  const selectedProfile = useMemo(
    () => availableProfiles.find((profile) => profile.id === form.accountRef) ?? null,
    [availableProfiles, form.accountRef],
  );
  const modelOptions = useMemo(() => selectedProfile?.models ?? [], [selectedProfile]);
  const hasEmptyCreatableAccounts = !isEdit && !loadingProfiles && availableProfiles.length === 0;
  const saveBlockedByProfileBinding = hasEmptyCreatableAccounts;

  // 别名唯一性：收集其他成员的 mention patterns（小写）
  const reservedPatterns = useMemo(() => {
    const set = new Set<string>();
    for (const m of members) {
      if (m.id === breed?.id) continue;
      for (const p of m.mention_patterns ?? []) set.add(p.toLowerCase());
    }
    return set;
  }, [members, breed?.id]);

  useEffect(() => {
    let cancelled = false;
    setLoadingProfiles(true);
    settingsService
      .getAccounts()
      .then((data) => {
        if (cancelled) return;
        setProfiles(data);
        const pendingProfileId = pendingProfileIdRef.current;
        if (!pendingProfileId) return;
        const createdProfile = data.find((profile) => profile.id === pendingProfileId);
        if (!createdProfile) return;
        pendingProfileIdRef.current = null;
        setHasUnsavedChanges(true);
        setForm((prev) => ({
          ...prev,
          accountRef: createdProfile.id,
          defaultModel: createdProfile.models?.[0] ?? '',
        }));
      })
      .catch(() => {
        if (!cancelled) setProfiles([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingProfiles(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profilesVersion]);

  useEffect(() => {
    if (isEdit) return;
    let cancelled = false;
    apiGet<BreedConfig[]>('/api/breeds/templates')
      .then((data) => {
        if (!cancelled) setTemplates(Array.isArray(data) ? data : []);
      })
      .catch(() => {
        if (!cancelled) setTemplates([]);
      });
    return () => {
      cancelled = true;
    };
  }, [isEdit]);

  useEffect(() => {
    let cancelled = false;
    breedService
      .getBreeds()
      .then((data) => {
        if (!cancelled) setMembers(data);
      })
      .catch(() => {
        if (!cancelled) setMembers([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // 切换 client / 加载模型后，若未手动填写则自动填充首个模型（编辑态已带值，不会覆盖）
  useEffect(() => {
    if (form.clientId === 'antigravity') return;
    setForm((prev) => {
      if (prev.defaultModel.trim().length > 0) return prev;
      return { ...prev, defaultModel: modelOptions[0] ?? '' };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 仅随 client / 模型列表变化自动填充
  }, [form.clientId, modelOptions]);

  const patchForm = (patch: Partial<BreedEditorFormState>) => {
    setHasUnsavedChanges(true);
    setForm((prev) => ({ ...prev, ...patch }));
    if (patch.mentionPatterns !== undefined) {
      setFieldErrors((prev) => ({ ...prev, routing: false }));
    }
    if (patch.name !== undefined || patch.roleDescription !== undefined) {
      setFieldErrors((prev) => ({ ...prev, identity: false }));
    }
    if (patch.defaultModel !== undefined || patch.accountRef !== undefined) {
      setFieldErrors((prev) => ({ ...prev, account: false }));
    }
  };

  const handleTemplateSelect = (tpl: BreedConfig | null) => {
    if (!tpl) {
      setSelectedTemplateId('custom');
      setForm(initialState(null));
      setHasUnsavedChanges(false);
      return;
    }
    setSelectedTemplateId(tpl.id);
    // 全量实例化：从模板 BreedConfig 复制 variant / CLI / role / caution 等，
    // 用户只需绑定账号与模型即可得到与模板设计一致的可用犬（决策 D1/D3）。
    const base = initialState(tpl);
    const name = tpl.name;
    const dogId = autoSlug(name);
    // 别名去重：模板的 nickname / name / mention_patterns 投影为可 @ 别名。
    const rawAliases = [tpl.nickname, name, ...(tpl.mention_patterns ?? [])].filter(
      (x): x is string => Boolean(x),
    );
    const deduped = rawAliases.map((alias) => {
      const normalized = normalizeMentionPattern(alias);
      if (!reservedPatterns.has(normalized.toLowerCase())) return normalized;
      for (let i = 2; i <= 99; i++) {
        const candidate = normalizeMentionPattern(`${alias}${i}`);
        if (!reservedPatterns.has(candidate.toLowerCase())) return candidate;
      }
      return normalized;
    });
    patchForm({
      ...base,
      name,
      displayName: tpl.display_name || name,
      dogId,
      mentionPatterns: joinTags(deduped),
    });
  };

  const requestClose = () => {
    if (!hasUnsavedChanges) {
      onClose();
      return;
    }
    setConfirmClose(true);
  };

  const handleSave = async () => {
    const errors: Record<string, boolean> = {};
    const errorMessages: string[] = [];
    if (!isEdit) {
      if (!form.name.trim()) {
        errors.identity = true;
        errorMessages.push('名称');
      }
      if (!form.roleDescription.trim()) {
        errors.identity = true;
        errorMessages.push('角色描述');
      }
      if (!form.defaultModel.trim() && selectedProfile?.authType === 'api_key') {
        errors.account = true;
        errorMessages.push('Model');
      }
      const effectiveCreateForm =
        selectedProfile?.authType === 'api_key' ? withDefaultModelMentionPattern(form) : form;
      if (splitMentionPatterns(effectiveCreateForm.mentionPatterns).length === 0) {
        errors.routing = true;
        errorMessages.push('别名');
      }
    }
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      setError(`请填写必填字段：${errorMessages.join('、')}`);
      return;
    }
    setFieldErrors({});
    setSaving(true);
    setError(null);
    try {
      const effectiveForm =
        !isEdit && selectedProfile?.authType === 'api_key' ? withDefaultModelMentionPattern(form) : form;
      const payload = buildBreedPayload(effectiveForm, breed);
      onSave(payload);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const createAccountClient = form.clientId as (typeof CLIENT_IDS)[number]['id'];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4 backdrop-blur-sm"
      onClick={(event) => {
        if (event.target === event.currentTarget) void requestClose();
      }}
      data-bootcamp-host="breed-editor-modal"
    >
      <div
        className="flex max-h-[88vh] w-full max-w-[720px] flex-col overflow-hidden rounded-[28px] border border-slate-800 bg-slate-900 shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-labelledby="breed-editor-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex shrink-0 items-start justify-between px-7 py-5">
          <p id="breed-editor-title" className="text-compact font-extrabold text-slate-100">
            {isEdit ? form.displayName || form.name || breed?.id || t('personas.editDog') : t('members.add')}
          </p>
          <button
            type="button"
            onClick={requestClose}
            className="flex h-8 w-8 items-center justify-center rounded-[10px] bg-slate-800 text-lg leading-none text-slate-300 transition hover:bg-slate-700"
            aria-label="关闭"
          >
            ×
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-7 py-5">
          {!isEdit ? (
            <p className="rounded-[14px] bg-slate-800/60 px-4 py-3 text-xs font-semibold text-slate-300">
              {t('breedEditor.createHint')}
            </p>
          ) : null}
          {!isEdit && templates.length > 0 && (
            <section className="space-y-3 rounded-[18px] bg-slate-950/30 p-[18px] border border-slate-800">
              <h4 className="text-base font-extrabold text-slate-100">{t('breedEditor.memberTemplate')}</h4>
              <p className="text-xs font-semibold text-slate-400">{t('breedEditor.templateHint')}</p>
              <div className="flex flex-wrap gap-2.5">
                <button
                  type="button"
                  onClick={() => handleTemplateSelect(null)}
                  className={`h-8 rounded-2xl px-3.5 text-xs font-extrabold transition ${
                    selectedTemplateId === 'custom'
                      ? 'bg-amber-600 text-white'
                      : 'bg-slate-800 text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {t('breedEditor.custom')}
                </button>
                {templates.map((tpl) => (
                  <button
                    key={tpl.id}
                    type="button"
                    onClick={() => handleTemplateSelect(selectedTemplateId === tpl.id ? null : tpl)}
                    className={`h-8 rounded-2xl px-3.5 text-xs font-extrabold transition ${
                      selectedTemplateId === tpl.id
                        ? 'bg-amber-600 text-white'
                        : 'bg-slate-800 text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {tpl.display_name || tpl.nickname || tpl.name}
                  </button>
                ))}
              </div>
            </section>
          )}

          <IdentitySection
            breed={breed}
            form={form}
            hasError={fieldErrors.identity}
            onChange={patchForm}
          />
          <AccountSection
            form={form}
            hasError={fieldErrors.account}
            modelOptions={modelOptions}
            availableProfiles={availableProfiles}
            loadingProfiles={hasEmptyCreatableAccounts ? true : loadingProfiles}
            onChange={patchForm}
          />
          {hasEmptyCreatableAccounts ? (
            <section
              aria-label="认证账号空状态"
              className="rounded-[18px] border border-slate-800 bg-slate-950/30 p-[18px]"
            >
              <p className="text-xs leading-5 text-slate-400">
                当前没有可用的认证账号。先新建或登录账号，再继续选择模型并保存成员。
              </p>
              <button
                type="button"
                onClick={() => setShowAuthModal(true)}
                className="mt-2 rounded-lg bg-amber-600 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-amber-500"
              >
                新建 / 登录账号
              </button>
            </section>
          ) : null}
          <RoutingSection
            form={form}
            hasError={fieldErrors.routing}
            reservedPatterns={reservedPatterns}
            onChange={patchForm}
          />
          <AdvancedRuntimeSection form={form} onChange={patchForm} />
          <PersistenceBanner />
          {error ? (
            <p className="rounded-2xl bg-rose-500/10 px-4 py-3 text-sm text-rose-300">{error}</p>
          ) : null}
        </div>

        <div className="flex items-center justify-end px-7 pb-5 pt-4">
          <button
            type="button"
            onClick={handleSave}
            disabled={saving || saveBlockedByProfileBinding}
            className="h-8 rounded-[10px] bg-amber-600 px-4 text-xs font-extrabold text-white transition hover:bg-amber-500 disabled:opacity-50"
          >
            {saving ? '保存中…' : t('common.save')}
          </button>
        </div>
      </div>

      {confirmClose && (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-sm rounded-xl border border-slate-800 bg-slate-900 p-6 shadow-2xl">
            <h3 className="text-sm font-bold text-amber-300">关闭确认</h3>
            <p className="mt-2 text-xs text-slate-400">有未保存的修改，确定要关闭吗？</p>
            <div className="mt-4 flex justify-end space-x-2">
              <button
                onClick={() => setConfirmClose(false)}
                className="rounded-xl bg-slate-800 px-4 py-2 text-xs text-slate-300 hover:bg-slate-700"
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={() => {
                  setConfirmClose(false);
                  onClose();
                }}
                className="rounded-xl bg-amber-600 px-4 py-2 text-xs font-semibold text-white hover:bg-amber-500"
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}

      <AccountAuthModal
        open={showAuthModal && createAccountClient !== undefined}
        initialClientId={createAccountClient}
        onClose={() => setShowAuthModal(false)}
        onCreated={(profileId) => {
          pendingProfileIdRef.current = profileId;
          setShowAuthModal(false);
          setProfilesVersion((version) => version + 1); // 重新加载账号列表
        }}
      />
    </div>
  );
}
