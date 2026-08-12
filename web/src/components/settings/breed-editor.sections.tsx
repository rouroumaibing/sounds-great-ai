// 添加成员弹窗的分段（Identity / Account / Routing）组件，适配 SG 数据层与深色主题。
// 裁剪项：
//  - Avatar / 参考音频上传按钮 → 纯文本输入（SG 无资源上传通道）
//  - CatColorField 颜色组件 → 两个颜色文本输入框
//  - Transport / ACP / Codex Carrier / OpenCode Provider → SG 无对应后端能力
//  - teamStrengths 与 strengths 合并为单一字段（SG BreedConfig.team_strengths）

'use client';

import { useMemo } from 'react';
import type { BreedConfig } from '../../types/api';
import type { SettingsAccount } from '../../types';
import type { BreedEditorFormState } from './breed-editor.model';
import {
  autoSlug,
  CLIENT_OPTIONS,
  joinTags,
  normalizeMentionPattern,
  splitMentionPatterns,
  splitStrengthTags,
} from './breed-editor.model';
import { SectionCard, SelectField, TextField, TextAreaField, Toggle } from './breed-editor-fields';
import { TagEditor } from './TagEditor';
import { CLIENT_IDS, providerFromClientId } from '../../constants/clientIds';
import { useI18n } from '../../store/useI18n';

type FormPatch = Partial<BreedEditorFormState>;

function currentAliasTags(form: BreedEditorFormState): string[] {
  return splitMentionPatterns(form.mentionPatterns).map(normalizeMentionPattern).filter(Boolean);
}

function clientLabel(id?: string): string {
  const found = CLIENT_IDS.find((c) => c.id === id);
  return found?.label ?? (typeof id === 'string' ? id : '');
}

export function IdentitySection({
  breed,
  form,
  hasError,
  onChange,
}: {
  breed?: BreedConfig | null;
  form: BreedEditorFormState;
  hasError?: boolean;
  onChange: (patch: FormPatch) => void;
}) {
  const { t } = useI18n();
  const strengthTags = splitStrengthTags(form.teamStrengths);

  return (
    <SectionCard title={t('breedEditor.identityInfo')} tone={hasError ? 'error' : 'neutral'}>
      {!breed ? (
        <>
          <TextField
            label={t('breedEditor.name')}
            ariaLabel="Name"
            value={form.name}
            onChange={(value) => onChange({ name: value, displayName: value, dogId: autoSlug(value, form.dogId) })}
            required
            placeholder={t('settings.hubbreededitor.s1')}
          />
          <input type="hidden" aria-label="Dog ID" value={form.dogId} />
        </>
      ) : (
        <>
          <label className="flex flex-col gap-1.5 text-slate-300 sm:flex-row sm:items-center sm:gap-[14px]">
            <span className="text-xs font-bold text-slate-300 sm:w-[150px] sm:shrink-0">Dog ID</span>
            <div className="min-w-0 flex-1">
              <code className="block w-full rounded-lg border border-transparent bg-slate-950 px-3 py-1.5 font-mono text-xs text-slate-400 select-all">
                {form.dogId}
              </code>
            </div>
          </label>
          <TextField
            label={t('breedEditor.name')}
            ariaLabel="Name"
            value={form.name}
            onChange={(value) => onChange({ name: value, displayName: value })}
          />
        </>
      )}

      <TextField
        label={t('breedEditor.nickname')}
        ariaLabel="Nickname"
        value={form.nickname}
        onChange={(value) => onChange({ nickname: value })}
        placeholder={t('settings.hubbreededitor.s2')}
      />
      <TextField
        label={t('breedEditor.variantLabel')}
        ariaLabel="Variant Label"
        value={form.variantLabel}
        onChange={(value) => onChange({ variantLabel: value })}
        placeholder={t('settings.hubbreededitor.s4')}
      />
      <TextAreaField
        label={t('breedEditor.roleDesc')}
        ariaLabel="Description"
        value={form.roleDescription}
        onChange={(value) => onChange({ roleDescription: value })}
        required
        placeholder={t('settings.hubbreededitor.s7')}
      />
      <TextField
        label={t('breedEditor.avatar')}
        ariaLabel="Avatar"
        value={form.avatar}
        onChange={(value) => onChange({ avatar: value })}
        placeholder={t('settings.hubbreededitor.s3')}
      />
      <TextField
        label={t('breedEditor.colorPrimary')}
        ariaLabel="Color Primary"
        value={form.colorPrimary}
        onChange={(value) => onChange({ colorPrimary: value })}
        placeholder={t('settings.hubbreededitor.s4')}
      />
      <TextField
        label={t('breedEditor.colorSecondary')}
        ariaLabel="Color Secondary"
        value={form.colorSecondary}
        onChange={(value) => onChange({ colorSecondary: value })}
        placeholder={t('settings.hubbreededitor.s5')}
      />
      <div className="flex items-start gap-3">
        <span className="w-[140px] shrink-0 pt-1 text-xs font-bold text-slate-300">Strengths</span>
        <div className="min-w-0 flex-1">
          <TagEditor
            tags={strengthTags}
            onChange={(tags) => onChange({ teamStrengths: joinTags(tags) })}
            addLabel="+ 选择"
            placeholder={t('settings.hubbreededitor.s9')}
            emptyLabel="(无)"
          />
        </div>
      </div>
      <TextAreaField
        label={t('breedEditor.personality')}
        ariaLabel="Personality"
        value={form.personality}
        onChange={(value) => onChange({ personality: value })}
        placeholder={t('settings.hubbreededitor.s8')}
      />
      <TextAreaField
        label={t('breedEditor.caution')}
        ariaLabel="Caution"
        value={form.caution}
        onChange={(value) => onChange({ caution: value })}
        placeholder={t('settings.hubbreededitor.s10')}
      />

      <VoiceConfigSection form={form} onChange={onChange} />
    </SectionCard>
  );
}

function VoiceConfigSection({
  form,
  onChange,
}: {
  form: BreedEditorFormState;
  onChange: (patch: FormPatch) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="rounded-[14px] border border-slate-800 p-3 space-y-2">
      <div className="text-[11px] font-bold text-slate-300">{t('breedEditor.voice')}</div>
      <div className="grid grid-cols-2 gap-3">
        <TextField
          label={t('breedEditor.voiceName')}
          value={form.voiceVoice}
          onChange={(value) => onChange({ voiceVoice: value })}
          placeholder="zh-CN-Yunxi"
        />
        <TextField
          label={t('breedEditor.voiceLang')}
          value={form.voiceLangCode}
          onChange={(value) => onChange({ voiceLangCode: value })}
          placeholder="zh-CN"
        />
        <TextField
          label={t('breedEditor.voiceSpeed')}
          value={form.voiceSpeed}
          onChange={(value) => onChange({ voiceSpeed: value })}
          inputMode="decimal"
          placeholder="1"
        />
        <TextField
          label={t('breedEditor.voiceRefAudio')}
          value={form.voiceRefAudio}
          onChange={(value) => onChange({ voiceRefAudio: value })}
          placeholder="https://..."
        />
      </div>
    </div>
  );
}

export function AccountSection({
  form,
  hasError,
  modelOptions,
  availableProfiles,
  loadingProfiles,
  onChange,
}: {
  form: BreedEditorFormState;
  hasError?: boolean;
  modelOptions: string[];
  availableProfiles: SettingsAccount[];
  loadingProfiles: boolean;
  onChange: (patch: FormPatch) => void;
}) {
  const { t } = useI18n();
  const accountOptions = availableProfiles;
  const provider = providerFromClientId(form.clientId);

  return (
    <SectionCard title={t('breedEditor.accountBinding')} tone={hasError ? 'error' : 'neutral'}>
      <div className="space-y-2">
        <SelectField
          label={t('breedEditor.client')}
          value={form.clientId}
          options={CLIENT_OPTIONS}
          onChange={(value) => onChange({ clientId: value, accountRef: '', defaultModel: '' })}
          required
        />

        <label className="flex flex-col gap-1.5 text-slate-300 sm:flex-row sm:items-center sm:gap-[14px]">
          <span className="text-xs font-bold text-slate-300 sm:w-[150px] sm:shrink-0">
            {t('breedEditor.provider')}
          </span>
          <div className="min-w-0 flex-1">
            <code className="block w-full rounded-lg border border-transparent bg-slate-950 px-3 py-1.5 font-mono text-xs text-slate-400 select-all">
              {provider ? `${provider}（${clientLabel(form.clientId)}）` : t('breedEditor.providerHint')}
            </code>
          </div>
        </label>

        <SelectField
          label={t('breedEditor.boundAccount')}
          value={form.accountRef}
          options={[
            { value: '', label: loadingProfiles ? t('common.loading') : t('breedEditor.pleaseSelectAccount') },
            ...accountOptions
              .map((profile) => ({
                value: profile.id,
                label: profile.builtin
                  ? `${profile.displayName ?? profile.name}（内置）`
                  : profile.authType === 'oauth'
                    ? `${profile.displayName ?? profile.name}（OAuth）`
                    : `${profile.displayName ?? profile.name}（API Key）`,
              })),
          ]}
          onChange={(value) => onChange({ accountRef: value, defaultModel: '' })}
          disabled={loadingProfiles}
          required
        />
        <TextField
          label={t('breedEditor.defaultModel')}
          ariaLabel="Model"
          value={form.defaultModel}
          onChange={(value) => onChange({ defaultModel: value })}
          suggestions={modelOptions}
          required
          placeholder={t('settings.hubbreededitor.s11')}
        />
        <div className="flex items-center justify-between p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
          <span className="text-sm text-slate-300">{t('breedEditor.mcpSupport')}</span>
          <Toggle on={form.mcpSupport} onChange={(v) => onChange({ mcpSupport: v })} />
        </div>
        <p className="-mt-1 text-xs leading-4 text-slate-500">
          控制该成员是否接收协作与记忆工具（post_message、search_evidence 等）。关闭后该成员无法使用 MCP 工具。
        </p>
      </div>
    </SectionCard>
  );
}

export function RoutingSection({
  form,
  hasError,
  reservedPatterns,
  onChange,
}: {
  form: BreedEditorFormState;
  hasError?: boolean;
  /** Lowercase alias set already taken by other members. */
  reservedPatterns?: ReadonlySet<string>;
  onChange: (patch: FormPatch) => void;
}) {
  const { t } = useI18n();
  const aliases = currentAliasTags(form);
  const validateAlias = useMemo(() => {
    if (!reservedPatterns?.size) return undefined;
    return (tag: string) => {
      if (reservedPatterns.has(tag.toLowerCase())) {
        return `别名 "${tag}" 已被其他成员使用`;
      }
      return null;
    };
  }, [reservedPatterns]);
  return (
    <SectionCard title={t('breedEditor.routing')} description={t('breedEditor.routingHint')} tone={hasError ? 'error' : 'neutral'}>
      <TagEditor
        tags={aliases}
        onChange={(tags) => onChange({ mentionPatterns: joinTags(tags) })}
        addLabel="+ 添加"
        placeholder="@边牧"
        emptyLabel="(至少添加 1 个别名，否则无法 @)"
        validate={validateAlias}
        minCount={1}
      />
      <textarea
        aria-label="Aliases"
        value={form.mentionPatterns}
        onChange={(event) => onChange({ mentionPatterns: event.target.value })}
        placeholder="@边牧, @bianmu"
        className="sr-only"
      />
    </SectionCard>
  );
}
