import type { OpsSpan } from '../../hooks/useOpsTraces';
import { useI18n } from '../../store/useI18n';

interface PromptXRayProps {
  span: OpsSpan | null;
  onClose?: () => void;
}

function attrStr(span: OpsSpan, key: string): string {
  const v = span.Attributes[key];
  if (v === undefined || v === null) return '';
  return String(v);
}

export function PromptXRay({ span, onClose }: PromptXRayProps) {
  const { t } = useI18n();
  if (!span) return null;

  const systemPrompt = attrStr(span, 'prompt.system');
  const userPrompt = attrStr(span, 'prompt.user');
  const systemTokens = parseInt(attrStr(span, 'prompt.system.tokens') || '0', 10);
  const userTokens = parseInt(attrStr(span, 'prompt.user.tokens') || '0', 10);
  const totalTokens = systemTokens + userTokens;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={onClose}>
      <div
        className="w-full max-w-3xl max-h-[80vh] bg-slate-900 rounded-2xl border border-slate-800 overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-3 border-b border-slate-800 flex-shrink-0">
          <div className="flex items-center gap-2">
            <span className="text-lg">🔍</span>
            <span className="text-sm font-bold text-slate-200">Prompt X-Ray Inspector</span>
            <span className="text-[10px] text-slate-500 font-mono">({span.SpanID})</span>
          </div>
          <button onClick={onClose} className="text-slate-500 hover:text-slate-300 text-lg">✕</button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {/* Token breakdown bar */}
          {totalTokens > 0 && (
            <div className="space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-slate-500 font-bold">Token Breakdown</div>
              <div className="flex h-4 rounded overflow-hidden bg-slate-950">
                {systemTokens > 0 && (
                  <div
                    className="bg-indigo-500/60 flex items-center justify-center text-[9px] text-white"
                    style={{ width: `${(systemTokens / totalTokens) * 100}%` }}
                  >
                    {systemTokens > totalTokens * 0.1 ? `Sys ${systemTokens}` : ''}
                  </div>
                )}
                {userTokens > 0 && (
                  <div
                    className="bg-emerald-500/60 flex items-center justify-center text-[9px] text-white"
                    style={{ width: `${(userTokens / totalTokens) * 100}%` }}
                  >
                    {userTokens > totalTokens * 0.1 ? `User ${userTokens}` : ''}
                  </div>
                )}
              </div>
              <div className="text-[10px] text-slate-500 font-mono">Total: {totalTokens} tokens</div>
            </div>
          )}

          {/* System prompt */}
          {systemPrompt && (
            <div className="space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-indigo-400 font-bold">System Prompt</div>
              <pre className="p-3 rounded bg-slate-950/60 border border-slate-800 text-[11px] text-slate-300 whitespace-pre-wrap font-mono max-h-48 overflow-y-auto">
                {systemPrompt}
              </pre>
            </div>
          )}

          {/* User prompt */}
          {userPrompt && (
            <div className="space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-emerald-400 font-bold">User Prompt</div>
              <pre className="p-3 rounded bg-slate-950/60 border border-slate-800 text-[11px] text-slate-300 whitespace-pre-wrap font-mono max-h-48 overflow-y-auto">
                {userPrompt}
              </pre>
            </div>
          )}

          {!systemPrompt && !userPrompt && (
            <div className="text-center text-slate-500 text-xs py-4">{t('auto.promptxray.7')}</div>
          )}
        </div>
      </div>
    </div>
  );
}
