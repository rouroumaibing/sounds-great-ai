import { useEffect, useState } from 'react';
import clsx from 'clsx';
import type { BreedConfig, Variant, BreedColor } from '../../types/api';
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
  const [expanded, setExpanded] = useState<number>(0);
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

  return (
    <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl p-6 space-y-4 shadow-2xl max-h-[90vh] overflow-y-auto" role="dialog" aria-modal="true">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
            <i className="fa-solid fa-dog text-amber-400"></i>
            <span>{isEdit ? t('personas.editDog') : t('personas.create')}</span>
          </h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>

        {!isEdit && templates.length > 0 && (
          <div className="space-y-2">
            <h4 className="text-xs font-bold text-slate-300">{t('breedEditor.memberTemplate')}</h4>
            <div className="flex flex-wrap gap-2">
              <button onClick={() => handleTemplateSelect(null)} className={clsx('px-3 py-1.5 rounded-xl text-xs font-medium transition', selectedTemplateId === 'custom' ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}>
                {t('accounts.customTag')}
              </button>
              {templates.map(t => (
                <button key={t.id} onClick={() => handleTemplateSelect(selectedTemplateId === t.id ? null : t)} className={clsx('px-3 py-1.5 rounded-xl text-xs font-medium transition', selectedTemplateId === t.id ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}>
                  {t.name}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="space-y-2 text-xs">
          <CollapsibleSection title={t('breedEditor.identityInfo')} isOpen={expanded === 0} onToggle={() => setExpanded(expanded === 0 ? -1 : 0)}>
            <IdentitySection form={form} color={color} update={update} updateColor={updateColor} />
          </CollapsibleSection>
          <CollapsibleSection title={t('breedEditor.accountBinding')} isOpen={expanded === 1} onToggle={() => setExpanded(expanded === 1 ? -1 : 1)}>
            <AccountSection variant={variant} updateVariant={updateVariant} accounts={filteredAccounts} loadingAccounts={loadingAccounts} onAccountChange={handleAccountChange} />
          </CollapsibleSection>
          <CollapsibleSection title={t('breedEditor.cliConfig')} isOpen={expanded === 2} onToggle={() => setExpanded(expanded === 2 ? -1 : 2)}>
            <CliSection variant={variant} updateVariant={updateVariant} updateCli={updateCli} />
          </CollapsibleSection>
          <CollapsibleSection title={t('breedEditor.contextBudget')} isOpen={expanded === 3} onToggle={() => setExpanded(expanded === 3 ? -1 : 3)}>
            <BudgetSection variant={variant} updateBudget={updateBudget} />
          </CollapsibleSection>
          <CollapsibleSection title={t('breedEditor.sessionStrategy')} isOpen={expanded === 4} onToggle={() => setExpanded(expanded === 4 ? -1 : 4)}>
            <StrategySection form={form} variant={variant} update={update} updateVariant={updateVariant} />
          </CollapsibleSection>
        </div>

        <div className="flex justify-end space-x-2 pt-2 border-t border-slate-800">
          <button onClick={onClose} className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium">{t('common.cancel')}</button>
          <button onClick={handleSave} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-lg shadow-amber-600/20">{t('common.save')}</button>
        </div>
      </div>
    </div>
  );
}

function CollapsibleSection({ title, isOpen, onToggle, children }: { title: string; isOpen: boolean; onToggle: () => void; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-slate-800 overflow-hidden">
      <button onClick={onToggle} className="w-full flex items-center justify-between px-4 py-2.5 bg-slate-950/60 hover:bg-slate-950 transition">
        <span className="text-xs font-bold text-slate-200">{title}</span>
        <i className={clsx('fa-solid fa-chevron-down text-[10px] text-slate-500 transition-transform', isOpen && 'rotate-180')}></i>
      </button>
      {isOpen && <div className="p-4 space-y-3">{children}</div>}
    </div>
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

function IdentitySection({ form, color, update, updateColor }: {
  form: BreedConfig; color: BreedColor;
  update: (p: Partial<BreedConfig>) => void; updateColor: (p: Partial<BreedColor>) => void;
}) {
  const { t } = useI18n();
  return (
    <>
      <div className="grid grid-cols-2 gap-3">
        <Field label={t('rules.name')}><input value={form.name} onChange={e => update({ name: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s1')} /></Field>
        <Field label={t('breedEditor.displayName')}><input value={form.display_name} onChange={e => update({ display_name: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s1')} /></Field>
        <Field label={t('breedEditor.nickname')}><input value={form.nickname ?? ''} onChange={e => update({ nickname: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s2')} /></Field>
        <Field label={t('concierge.avatar')}><input value={form.avatar} onChange={e => update({ avatar: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s3')} /></Field>
        <Field label={t('breedEditor.colorPrimary')}><input value={color.primary} onChange={e => updateColor({ primary: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s4')} /></Field>
        <Field label={t('breedEditor.colorSecondary')}><input value={color.secondary} onChange={e => updateColor({ secondary: e.target.value })} className={inputCls} placeholder={t('settings.hubbreededitor.s5')} /></Field>
      </div>
      <Field label={t('breedEditor.mentionPatterns')}><TagEditor tags={form.mention_patterns} onChange={t => update({ mention_patterns: t })} placeholder={t('settings.hubbreededitor.s6')} /></Field>
      <Field label={t('breedEditor.roleDesc')}><textarea value={form.role_description ?? ''} onChange={e => update({ role_description: e.target.value })} className={inputCls} rows={2} placeholder={t('settings.hubbreededitor.s7')} /></Field>
      <Field label={t('breedEditor.personality')}><textarea value={form.personality} onChange={e => update({ personality: e.target.value })} className={inputCls} rows={2} placeholder={t('settings.hubbreededitor.s8')} /></Field>
      <Field label={t('breedEditor.strengths')}><TagEditor tags={(form.team_strengths ?? '').split(',').filter(Boolean)} onChange={t => update({ team_strengths: t.join(',') })} placeholder={t('settings.hubbreededitor.s9')} /></Field>
      <Field label={t('breedEditor.caution')}><textarea value={form.caution ?? ''} onChange={e => update({ caution: e.target.value })} className={inputCls} rows={2} placeholder={t('settings.hubbreededitor.s10')} /></Field>
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
