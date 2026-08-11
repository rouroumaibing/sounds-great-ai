import { useEffect, useState } from 'react';
import clsx from 'clsx';
import type { BreedConfig, Variant, BreedColor, VoiceConfig } from '../../types/api';
import type { SettingsAccount } from '../../types';
import { CLIENT_IDS, providerFromClientId } from '../../constants/clientIds';
import { settingsService } from '../../services/settingsService';
import { apiGet } from '../../services/http';
import { TagEditor } from './TagEditor';
import { useI18n } from '../../store/useI18n';

interface HubBreedEditorProps {
  breed?: BreedConfig;
  onSave: (breed: BreedConfig) => void;
  onClose: () => void;
}

interface BreedTemplate {
  id: string;
  name: string;
  avatar?: string;
  color?: { primary: string; secondary: string };
  role_description?: string;
  personality?: string;
  default_roles?: string[];
}

function emptyBreed(): BreedConfig {
  return {
    id: '', name: '', display_name: '', avatar: '', personality: '',
    mention_patterns: [], roles: [], default_variant_id: 'default',
    variants: [{ id: 'default', client_id: '', default_model: '', mcp_support: false, cli: { command: '', output_format: '' } }],
    source: 'user', enabled: true,
  };
}

export function HubBreedEditor({ breed, onSave, onClose }: HubBreedEditorProps) {
  const { t } = useI18n();
  const [form, setForm] = useState<BreedConfig>(breed ?? emptyBreed());
  const [accounts, setAccounts] = useState<SettingsAccount[]>([]);
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [templates, setTemplates] = useState<BreedTemplate[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>('custom');

  const variant: Variant = form.variants[0] ?? form.variants.find(v => v.id === form.default_variant_id);
  const color: BreedColor = form.color ?? { primary: '', secondary: '' };
  const isEdit = !!breed;

  useEffect(() => {
    let cancelled = false;
    setLoadingAccounts(true);
    settingsService.getAccounts()
      .then((data) => { if (!cancelled) setAccounts(data); })
      .catch(() => { if (!cancelled) setAccounts([]); })
      .finally(() => { if (!cancelled) setLoadingAccounts(false); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (isEdit) return;
    let cancelled = false;
    apiGet<BreedTemplate[]>('/api/breeds/templates')
      .then((data) => { if (!cancelled) setTemplates(Array.isArray(data) ? data : []); })
      .catch(() => { if (!cancelled) setTemplates([]); });
    return () => { cancelled = true; };
  }, [isEdit]);

  const update = (patch: Partial<BreedConfig>) => setForm(f => ({ ...f, ...patch }));
  const updateVariant = (patch: Partial<Variant>) => setForm(f => {
    const vs = [...f.variants];
    vs[0] = { ...vs[0], ...patch };
    return { ...f, variants: vs };
  });
  const updateColor = (patch: Partial<BreedColor>) => setForm(f => ({ ...f, color: { ...f.color ?? { primary: '', secondary: '' }, ...patch } }));
  const updateBudget = (patch: Record<string, number | undefined>) => setForm(f => {
    const vs = [...f.variants];
    vs[0] = { ...vs[0], context_budget: { ...vs[0].context_budget ?? {}, ...patch } };
    return { ...f, variants: vs };
  });
  const updateCli = (patch: Record<string, string | string[] | undefined>) => setForm(f => {
    const vs = [...f.variants];
    vs[0] = { ...vs[0], cli: { ...vs[0].cli, ...patch } };
    return { ...f, variants: vs };
  });

  const handleTemplateSelect = (t: BreedTemplate | null) => {
    if (!t) {
      setSelectedTemplateId('custom');
      setForm(emptyBreed());
      return;
    }
    setSelectedTemplateId(t.id);
    setForm(f => ({
      ...f,
      name: t.name,
      display_name: t.name,
      avatar: t.avatar ?? f.avatar,
      color: t.color ?? f.color,
      role_description: t.role_description ?? f.role_description,
      personality: t.personality ?? f.personality,
      mention_patterns: t.default_roles?.map(r => `@${r}`) ?? f.mention_patterns,
    }));
  };

  const handleAccountChange = (accountId: string) => {
    updateVariant({ account_ref: accountId });
    const acct = accounts.find(a => a.id === accountId);
    if (acct?.models && acct.models.length > 0) {
      updateVariant({ default_model: acct.models[0] });
    }
  };

  const handleSave = () => {
    const v = form.variants[0];
    const finalVariant = { ...v, provider: providerFromClientId(v.client_id) };
    onSave({ ...form, variants: [finalVariant] });
  };

  const filteredAccounts = accounts.filter(a => !variant.client_id || a.clientId === variant.client_id);

  const title = isEdit ? (form.display_name || form.name || t('personas.editDog')) : t('members.add');

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4 backdrop-blur-sm"
      onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <div
        className="flex max-h-[88vh] w-full max-w-[720px] flex-col overflow-hidden rounded-2xl border border-slate-800 bg-slate-900 shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-labelledby="breed-editor-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex shrink-0 items-start justify-between px-7 py-5">
          <p id="breed-editor-title" className="text-base font-extrabold text-slate-100">{title}</p>
          <button
            type="button"
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-[10px] bg-slate-800 text-lg leading-none text-slate-300 transition hover:bg-slate-700"
            aria-label="关闭"
          >
            ×
          </button>
        </div>

        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-7 py-5">
          {!isEdit && (
            <p className="rounded-[14px] bg-slate-800/60 px-4 py-3 text-xs font-semibold text-slate-300">{t('breedEditor.createHint')}</p>
          )}
          {!isEdit && templates.length > 0 && (
            <SectionCard title={t('breedEditor.memberTemplate')}>
              <p className="text-xs text-slate-400">{t('breedEditor.templateHint')}</p>
              <div className="flex flex-wrap gap-2.5">
                <button
                  type="button"
                  onClick={() => handleTemplateSelect(null)}
                  className={clsx('h-8 rounded-2xl px-3.5 text-xs font-extrabold transition', selectedTemplateId === 'custom' ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}
                >
                  {t('breedEditor.custom')}
                </button>
                {templates.map((tpl) => (
                  <button
                    key={tpl.id}
                    type="button"
                    onClick={() => handleTemplateSelect(selectedTemplateId === tpl.id ? null : tpl)}
                    className={clsx('h-8 rounded-2xl px-3.5 text-xs font-extrabold transition', selectedTemplateId === tpl.id ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}
                >
                  {tpl.name}
                </button>
                ))}
              </div>
            </SectionCard>
          )}

          <SectionCard title={t('breedEditor.identityInfo')}>
            <IdentitySection form={form} color={color} update={update} updateColor={updateColor} updateVariant={updateVariant} dogId={form.id} />
          </SectionCard>
          <SectionCard title={t('breedEditor.accountBinding')}>
            <AccountSection variant={variant} updateVariant={updateVariant} accounts={filteredAccounts} loadingAccounts={loadingAccounts} onAccountChange={handleAccountChange} />
          </SectionCard>
          <SectionCard title={t('breedEditor.cliConfig')}>
            <CliSection variant={variant} updateVariant={updateVariant} updateCli={updateCli} />
          </SectionCard>
          <SectionCard title={t('breedEditor.contextBudget')}>
            <BudgetSection variant={variant} updateBudget={updateBudget} />
          </SectionCard>
          <SectionCard title={t('breedEditor.sessionStrategy')}>
            <StrategySection form={form} variant={variant} update={update} updateVariant={updateVariant} />
          </SectionCard>
        </div>

        <div className="flex items-center justify-end gap-2 px-7 pb-5 pt-4">
          <button onClick={onClose} className="h-8 rounded-[10px] bg-slate-800 px-4 text-xs font-semibold text-slate-200 transition hover:bg-slate-700">{t('common.cancel')}</button>
          <button onClick={handleSave} className="h-8 rounded-[10px] bg-amber-600 px-4 text-xs font-semibold text-white transition hover:bg-amber-500">{t('common.save')}</button>
        </div>
      </div>
    </div>
  );
}

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-xl border border-slate-800 bg-slate-950/30 p-4 space-y-3">
      <h4 className="text-sm font-bold text-slate-100">{title}</h4>
      {children}
    </section>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-slate-400 mb-1">{label}</label>
      {children}
    </div>
  );
}

const inputCls = 'w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-slate-200 focus:outline-none focus:border-amber-500';

function IdentitySection({ form, color, update, updateColor, updateVariant, dogId }: {
  form: BreedConfig; color: BreedColor;
  update: (p: Partial<BreedConfig>) => void; updateColor: (p: Partial<BreedColor>) => void;
  updateVariant: (p: Partial<Variant>) => void; dogId: string;
}) {
  const { t } = useI18n();
  const variant = form.variants[0];
  const vc: VoiceConfig = variant?.voice_config ?? {};
  const setVoice = (patch: Partial<VoiceConfig>) => updateVariant({ voice_config: { ...vc, ...patch } });
  return (
    <>
      {/* read-only Dog ID — mirrors clowder "Cat ID" shown above name for existing members */}
      {dogId && (
        <Field label={t('breedEditor.dogId')}>
          <code className="block w-full rounded-xl bg-slate-950 px-3 py-2 font-mono text-xs text-slate-400 select-all">{dogId}</code>
        </Field>
      )}
      <div className="grid grid-cols-2 gap-3">
        <Field label={t('rules.name')}><input value={form.name} onChange={e => update({ name: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s1')} /></Field>
        <Field label={t('breedEditor.displayName')}><input value={form.display_name} onChange={e => update({ display_name: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s1')} /></Field>
        <Field label={t('breedEditor.nickname')}><input value={form.nickname ?? ''} onChange={e => update({ nickname: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s2')} /></Field>
        <Field label={t('concierge.avatar')}><input value={form.avatar} onChange={e => update({ avatar: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s3')} /></Field>
        <Field label={t('breedEditor.colorPrimary')}><input value={color.primary} onChange={e => updateColor({ primary: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s4')} /></Field>
        <Field label={t('breedEditor.colorSecondary')}><input value={color.secondary} onChange={e => updateColor({ secondary: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s5')} /></Field>
      </div>
      <Field label={t('breedEditor.variantLabel')}><input value={variant?.variant_label ?? ''} onChange={e => updateVariant({ variant_label: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s4')} /></Field>
      <Field label={t('breedEditor.mentionPatterns')}><TagEditor tags={form.mention_patterns} onChange={t => update({ mention_patterns: t })} placeholder={t('settings.hubbreededitor.s6')} /></Field>
      <Field label={t('breedEditor.roleDesc')}><textarea value={form.role_description ?? ''} onChange={e => update({ role_description: e.target.value })} className={inputCls} rows={2} placeholder={t('settings.hubbreededitor.s7')} /></Field>
      <Field label={t('breedEditor.personality')}><textarea value={form.personality} onChange={e => update({ personality: e.target.value })} className={inputCls} rows={2} placeholder={t('settings.hubbreededitor.s8')} /></Field>
      <Field label={t('breedEditor.strengths')}><TagEditor tags={(form.team_strengths ?? '').split(',').filter(Boolean)} onChange={t => update({ team_strengths: t.join(',') })} placeholder={t('settings.hubbreededitor.s9')} /></Field>
      <Field label={t('breedEditor.caution')}><textarea value={form.caution ?? ''} onChange={e => update({ caution: e.target.value })} className={inputCls} rows={2} placeholder={t('settings.hubbreededitor.s10')} /></Field>

      {/* Voice config — mirrors clowder VoiceConfigSection */}
      <div className="rounded-xl border border-slate-800 p-3 space-y-3">
        <div className="text-[11px] font-bold text-slate-300">{t('breedEditor.voice')}</div>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t('breedEditor.voiceName')}><input value={vc.voice ?? ''} onChange={e => setVoice({ voice: e.target.value })} className={inputCls} placeholder="zh-CN-Yunxi" /></Field>
          <Field label={t('breedEditor.voiceLang')}><input value={vc.lang_code ?? ''} onChange={e => setVoice({ lang_code: e.target.value })} className={inputCls} placeholder="zh-CN" /></Field>
          <Field label={t('breedEditor.voiceSpeed')}><input type="number" step="0.1" value={vc.speed ?? ''} onChange={e => setVoice({ speed: Number(e.target.value) || undefined })} className={inputCls} placeholder="1" /></Field>
          <Field label={t('breedEditor.voiceRefAudio')}><input value={vc.ref_audio ?? ''} onChange={e => setVoice({ ref_audio: e.target.value })} className={inputCls} placeholder="https://..." /></Field>
        </div>
      </div>
    </>
  );
}

function AccountSection({ variant, updateVariant, accounts, loadingAccounts, onAccountChange }: {
  variant: Variant; updateVariant: (p: Partial<Variant>) => void;
  accounts: SettingsAccount[]; loadingAccounts: boolean; onAccountChange: (id: string) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="grid grid-cols-2 gap-3">
      <Field label={t('breedEditor.client')}>
        <select value={variant.client_id} onChange={e => updateVariant({ client_id: e.target.value })} className={inputCls}>
          <option value="">—</option>
          {CLIENT_IDS.map(c => <option key={c.id} value={c.id}>{c.label}</option>)}
        </select>
      </Field>
      <Field label={t('breedEditor.provider')}><input value={providerFromClientId(variant.client_id)} disabled className={clsx(inputCls, 'opacity-60')} placeholder={t('breedEditor.providerHint')} /></Field>
      <Field label={t('breedEditor.boundAccount')}>
        {loadingAccounts ? (
          <input value={t('common.loading')} disabled className={clsx(inputCls, 'opacity-60')} />
        ) : accounts.length === 0 ? (
          <p className="text-[11px] text-slate-500 italic">{t('breedEditor.noAccount')}</p>
        ) : (
          <select value={variant.account_ref ?? ''} onChange={e => onAccountChange(e.target.value)} className={inputCls}>
            <option value="">—</option>
            {accounts.map(a => <option key={a.id} value={a.id}>{a.displayName ?? a.name}</option>)}
          </select>
        )}
      </Field>
      <Field label={t('breedEditor.defaultModel')}><input value={variant.default_model} onChange={e => updateVariant({ default_model: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s11')} /></Field>
    </div>
  );
}

function CliSection({ variant, updateVariant, updateCli }: {
  variant: Variant; updateVariant: (p: Partial<Variant>) => void; updateCli: (p: Record<string, string | string[] | undefined>) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="grid grid-cols-2 gap-3">
      <Field label={t('breedEditor.command')}><input value={variant.cli.command} onChange={e => updateCli({ command: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s12')} /></Field>
      <Field label={t('breedEditor.outputFormat')}>
        <select value={variant.cli.output_format} onChange={e => updateCli({ output_format: e.target.value })} className={inputCls}>
          <option value="">—</option><option value="text">text</option><option value="json">json</option><option value="stream">stream</option>
        </select>
      </Field>
      <Field label={t('breedEditor.defaultArgs')}><input value={(variant.cli.default_args ?? []).join(' ')} onChange={e => updateCli({ default_args: e.target.value.split(' ').filter(Boolean) })} className={inputCls} placeholder={t('settings.hubbreededitor.s13')} /></Field>
      <Field label={t('breedEditor.effort')}>
        <select value={variant.cli.effort ?? ''} onChange={e => updateCli({ effort: e.target.value })} className={inputCls}>
          <option value="">—</option><option value="low">low</option><option value="medium">medium</option><option value="high">high</option>
        </select>
      </Field>
      <Field label={t('breedEditor.contextWindow')}><input type="number" value={variant.context_budget?.max_context_tokens ?? ''} onChange={e => updateVariant({ context_budget: { ...variant.context_budget ?? {}, max_context_tokens: Number(e.target.value) || undefined } })} className={inputCls} placeholder={t('settings.hubbreededitor.s14')} /></Field>
      <Field label={t('breedEditor.autoCompact')}><input type="number" value={variant.auto_compact_token_limit ?? ''} onChange={e => updateVariant({ auto_compact_token_limit: Number(e.target.value) || undefined })} className={inputCls} placeholder={t('settings.hubbreededitor.s15')} /></Field>
    </div>
  );
}

function BudgetSection({ variant, updateBudget }: {
  variant: Variant; updateBudget: (p: Record<string, number | undefined>) => void;
}) {
  const { t } = useI18n();
  const b = variant.context_budget ?? {};
  return (
    <div className="grid grid-cols-2 gap-3">
      <Field label={t('breedEditor.maxPromptTokens')}><input type="number" value={b.max_prompt_tokens ?? ''} onChange={e => updateBudget({ max_prompt_tokens: Number(e.target.value) || undefined })} className={inputCls} placeholder={t('settings.hubbreededitor.s16')} /></Field>
      <Field label={t('breedEditor.maxContextTokens')}><input type="number" value={b.max_context_tokens ?? ''} onChange={e => updateBudget({ max_context_tokens: Number(e.target.value) || undefined })} className={inputCls} placeholder={t('settings.hubbreededitor.s14')} /></Field>
      <Field label={t('breedEditor.maxMessages')}><input type="number" value={b.max_messages ?? ''} onChange={e => updateBudget({ max_messages: Number(e.target.value) || undefined })} className={inputCls} placeholder={t('settings.hubbreededitor.s17')} /></Field>
      <Field label={t('breedEditor.maxContentLength')}><input type="number" value={b.max_content_length_per_msg ?? ''} onChange={e => updateBudget({ max_content_length_per_msg: Number(e.target.value) || undefined })} className={inputCls} placeholder={t('settings.hubbreededitor.s18')} /></Field>
    </div>
  );
}

function StrategySection({ form, variant, update, updateVariant }: {
  form: BreedConfig; variant: Variant; update: (p: Partial<BreedConfig>) => void; updateVariant: (p: Partial<Variant>) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between p-3 rounded-xl bg-slate-950/60 border border-slate-800">
        <span className="text-slate-300">{t('breedEditor.mcpSupport')}</span>
        <Toggle on={variant.mcp_support} onChange={v => updateVariant({ mcp_support: v })} />
      </div>
      <div className="flex items-center justify-between p-3 rounded-xl bg-slate-950/60 border border-slate-800">
        <span className="text-slate-300">{t('breedEditor.sessionChain')}</span>
        <Toggle on={variant.session_chain ?? false} onChange={v => updateVariant({ session_chain: v })} />
      </div>
      <div className="flex items-center justify-between p-3 rounded-xl bg-slate-950/60 border border-slate-800">
        <span className="text-slate-300">{t('common.enabled')}</span>
        <Toggle on={form.enabled} onChange={v => update({ enabled: v })} />
      </div>
      <Field label={t('breedEditor.sessionStrategy')}>
        <select value={variant.strategy ?? ''} onChange={e => updateVariant({ strategy: e.target.value })} className={inputCls}>
          <option value="">—</option><option value="handoff">handoff</option><option value="compress">compress</option><option value="hybrid">hybrid</option>
        </select>
      </Field>
    </div>
  );
}

function Toggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  return (
    <button onClick={() => onChange(!on)} className={clsx('w-11 h-6 rounded-full p-0.5 transition-colors', on ? 'bg-amber-600' : 'bg-slate-800')}>
      <div className={clsx('w-5 h-5 rounded-full bg-white shadow-md transform transition-transform', on ? 'translate-x-5' : 'translate-x-0')} />
    </button>
  );
}
