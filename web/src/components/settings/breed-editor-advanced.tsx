// 添加成员弹窗的「高级运行时」分区，适配 SG 数据层与深色主题。
// 裁剪项：
//  - Session 策略独立端点（/api/config/session-strategy）→ SG 的策略存于 variant.strategy，用普通下拉
//  - Codex 运行参数分组（sandbox / approval / auth mode）→ SG 无对应
//  - CLI Effort 的 provider preset 提示 → 用固定 low/medium/high 下拉

'use client';

import type { BreedEditorFormState } from './breed-editor.model';
import {
  CLI_EFFORT_OPTIONS,
  OUTPUT_FORMAT_OPTIONS,
  SESSION_CHAIN_OPTIONS,
  SESSION_STRATEGY_OPTIONS,
} from './breed-editor.model';
import { SectionCard, SelectField, TextField, Toggle } from './breed-editor-fields';
import { TagEditor } from './TagEditor';
import { useI18n } from '../../store/useI18n';

type FormPatch = Partial<BreedEditorFormState>;

export function AdvancedRuntimeSection({
  form,
  onChange,
}: {
  form: BreedEditorFormState;
  onChange: (patch: FormPatch) => void;
}) {
  const { t } = useI18n();
  return (
    <SectionCard
      title={t('breedEditor.advancedTitle')}
      description={t('breedEditor.advancedHint')}
      tone="success"
    >
      <p className="text-xs leading-5 text-amber-200/80">
        上下文预算会随成员配置一起持久化到运行时 catalog。4 项要么全部留空，要么全部填写。
      </p>
      <div className="space-y-2">
        <TextField
          label={t('breedEditor.maxPromptTokens')}
          value={form.maxPromptTokens}
          onChange={(value) => onChange({ maxPromptTokens: value })}
          inputMode="numeric"
          tone="success"
          placeholder={t('settings.hubbreededitor.s16')}
        />
        <TextField
          label={t('breedEditor.maxContextTokens')}
          value={form.maxContextTokens}
          onChange={(value) => onChange({ maxContextTokens: value })}
          inputMode="numeric"
          tone="success"
          placeholder={t('settings.hubbreededitor.s14')}
        />
        <TextField
          label={t('breedEditor.maxMessages')}
          value={form.maxMessages}
          onChange={(value) => onChange({ maxMessages: value })}
          inputMode="numeric"
          tone="success"
          placeholder={t('settings.hubbreededitor.s17')}
        />
        <TextField
          label={t('breedEditor.maxContentLength')}
          ariaLabel="Max Content Length Per Msg"
          value={form.maxContentLengthPerMsg}
          onChange={(value) => onChange({ maxContentLengthPerMsg: value })}
          inputMode="numeric"
          tone="success"
          placeholder={t('settings.hubbreededitor.s18')}
        />
        <TextField
          label={t('breedEditor.autoCompact')}
          value={form.autoCompact}
          onChange={(value) => onChange({ autoCompact: value })}
          inputMode="numeric"
          tone="success"
          placeholder={t('settings.hubbreededitor.s15')}
        />
        <SelectField
          label={t('breedEditor.sessionChain')}
          value={form.sessionChain}
          options={SESSION_CHAIN_OPTIONS}
          onChange={(value) => onChange({ sessionChain: value as BreedEditorFormState['sessionChain'] })}
          tone="success"
        />

        {/* CLI 配置 */}
        <div className="space-y-2 rounded-[14px] border border-amber-700/30 bg-slate-950/40 p-[14px]">
          <p className="text-sm font-medium text-slate-200">{t('breedEditor.cliConfig')}</p>
          <p className="text-xs leading-4 text-slate-500">{t('breedEditor.cliConfigHint')}</p>
          <TextField
            label={t('breedEditor.command')}
            value={form.commandArgs}
            onChange={(value) => onChange({ commandArgs: value })}
            tone="success"
            placeholder={t('settings.hubbreededitor.s12')}
          />
          <SelectField
            label={t('breedEditor.outputFormat')}
            value={form.outputFormat}
            options={OUTPUT_FORMAT_OPTIONS}
            onChange={(value) => onChange({ outputFormat: value })}
            tone="success"
          />
          <SelectField
            label={t('breedEditor.effort')}
            value={form.cliEffort}
            options={CLI_EFFORT_OPTIONS}
            onChange={(value) => onChange({ cliEffort: value })}
            tone="success"
          />
          <div className="space-y-1">
            <p className="text-sm font-medium text-slate-200">额外 CLI 参数</p>
            <TagEditor
              tags={form.cliConfigArgs}
              onChange={(nextTags) => onChange({ cliConfigArgs: nextTags })}
              addLabel="+ 添加参数"
              placeholder="例如 --model xxx 或 --flag value"
              emptyLabel="无额外参数"
            />
            <p className="text-xs leading-4 text-slate-500">
              每条直接追加到 CLI 命令。`命令`、`输出格式`、`推理力度` 等结构化项始终以上方字段为准。
            </p>
          </div>
        </div>

        {/* 会话策略 */}
        <SelectField
          label={t('breedEditor.sessionStrategyLabel')}
          value={form.strategy}
          options={SESSION_STRATEGY_OPTIONS}
          onChange={(value) => onChange({ strategy: value as BreedEditorFormState['strategy'] })}
          tone="success"
        />

        {/* 启用开关 */}
        <div className="flex items-center justify-between p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
          <span className="text-sm text-slate-300">{t('breedEditor.enabled')}</span>
          <Toggle on={form.enabled} onChange={(v) => onChange({ enabled: v })} />
        </div>
      </div>
    </SectionCard>
  );
}
