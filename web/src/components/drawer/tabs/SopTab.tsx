import { useState } from 'react';
import { WorkflowSopPanel } from '../../workspace/WorkflowSopPanel';
import { useI18n } from '../../../store/useI18n';

/**
 * SopTab hosts the SOP bulletin board in the right drawer. SG has no backlog
 * list UI, so the feature id is entered directly; the panel then renders the
 * stage / baton holder / resume capsule / checks for that feature.
 */
export function SopTab() {
  const { t } = useI18n();
  const [input, setInput] = useState('');
  const [featureId, setFeatureId] = useState('');

  return (
    <div className="space-y-3">
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setFeatureId(input.trim());
        }}
      >
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={t('workspace.sopPanel.featurePlaceholder')}
          className="flex-1 rounded-lg bg-slate-900 border border-slate-700/70 px-2.5 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-amber-500/60"
          spellCheck={false}
        />
        <button
          type="submit"
          className="rounded-lg bg-amber-500/20 border border-amber-500/40 px-3 py-1.5 text-xs font-medium text-amber-300 hover:bg-amber-500/30 transition-colors"
        >
          {t('workspace.sopPanel.load')}
        </button>
      </form>
      <WorkflowSopPanel featureId={featureId} />
    </div>
  );
}
