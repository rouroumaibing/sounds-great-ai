import { useState } from 'react';
import clsx from 'clsx';
import type { SettingsAccount } from '../../types';
import { CLIENT_IDS } from '../../constants/clientIds';
import { TagEditor, KeyValueEditor } from './TagEditor';
import { useI18n } from '../../store/useI18n';

interface AccountEditModalProps {
  account?: SettingsAccount;
  onSave: (account: Omit<SettingsAccount, 'id'> & { id?: string; apiKey?: string }) => void;
  onClose: () => void;
}

function emptyAccount(): Omit<SettingsAccount, 'id'> {
  return { name: '', details: '', type: 'api_key', authType: 'api_key', mode: 'api_key', models: [], modelAliases: {}, envVars: {}, builtin: false };
}

export function AccountEditModal({ account, onSave, onClose }: AccountEditModalProps) {
  const { t } = useI18n();
  const [form, setForm] = useState<Omit<SettingsAccount, 'id'> & { id?: string; apiKey?: string }>(
    account ? { ...account, apiKey: '' } : { ...emptyAccount(), apiKey: '' }
  );

  const update = (patch: Partial<typeof form>) => setForm(f => ({ ...f, ...patch }));
  const isEdit = !!account;

  const handleSave = () => { onSave(form); };

  return (
    <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg p-6 space-y-4 shadow-2xl max-h-[90vh] overflow-y-auto" role="dialog" aria-modal="true">
        <div className="flex items-center justify-between border-b border-slate-800 pb-3">
          <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
            <i className="fa-solid fa-key text-amber-400"></i>
            <span>{isEdit ? t('accounts.editAccount') : t('accounts.createAccount')}</span>
            {form.builtin && <span className="px-2 py-0.5 rounded-lg bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 text-[10px] font-mono">builtin</span>}
          </h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>

        <div className="space-y-3 text-xs">
          <div className="grid grid-cols-2 gap-3">
            <Field label={t('accounts.accountId')}><input value={form.name} onChange={e => update({ name: e.target.value })} className={inputCls} /></Field>
            <Field label="displayName"><input value={form.displayName ?? ''} onChange={e => update({ displayName: e.target.value })} className={inputCls} /></Field>
          </div>

          <Field label="Provider (clientId)">
            <select value={form.clientId ?? ''} onChange={e => update({ clientId: e.target.value })} className={inputCls}>
              <option value="">—</option>
              {CLIENT_IDS.map(c => <option key={c.id} value={c.id}>{c.label}</option>)}
            </select>
          </Field>

          <div className="flex items-center justify-between p-3 rounded-xl bg-slate-950/60 border border-slate-800">
            <span className="text-slate-300">{t('accounts.authMethod')}</span>
            <div className="flex gap-1">
              <button onClick={() => update({ authType: 'api_key', type: 'api_key' })} className={clsx('px-3 py-1 rounded-lg font-medium', form.authType === 'api_key' ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400')}>api_key</button>
              <button onClick={() => update({ authType: 'oauth', type: 'oauth' })} className={clsx('px-3 py-1 rounded-lg font-medium', form.authType === 'oauth' ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400')}>oauth</button>
            </div>
          </div>

          {form.authType === 'api_key' && (
            <Field label="API Key"><input type="password" value={form.apiKey ?? ''} onChange={e => update({ apiKey: e.target.value })} placeholder="sk-..." className={clsx(inputCls, 'font-mono')} /></Field>
          )}

          <Field label={t('accounts.baseUrl')}><input value={form.baseUrl ?? ''} onChange={e => update({ baseUrl: e.target.value })} placeholder="https://..." className={inputCls} /></Field>

          <Field label="Mode">
            <select value={form.mode ?? 'api_key'} onChange={e => update({ mode: e.target.value })} className={inputCls}>
              <option value="api_key">api_key</option><option value="subscription">subscription</option>
            </select>
          </Field>

          <Field label={t('accounts.availableModels')}><TagEditor tags={form.models ?? []} onChange={t => update({ models: t })} placeholder="model name" /></Field>

          <Field label={t('accounts.modelAliases')}><KeyValueEditor entries={form.modelAliases ?? {}} onChange={e => update({ modelAliases: e })} keyPlaceholder="alias" valuePlaceholder="real model" /></Field>

          <Field label={t('accounts.envVars')}><KeyValueEditor entries={form.envVars ?? {}} onChange={e => update({ envVars: e })} keyPlaceholder="ENV_VAR" valuePlaceholder="value" /></Field>
        </div>

        <div className="flex justify-end space-x-2 pt-2 border-t border-slate-800">
          <button onClick={onClose} className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium">{t('common.cancel')}</button>
          <button onClick={handleSave} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-lg shadow-amber-600/20">{t('common.save')}</button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <div><label className="block text-slate-400 mb-1">{label}</label>{children}</div>;
}

const inputCls = 'w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-slate-200 focus:outline-none focus:border-amber-500';
