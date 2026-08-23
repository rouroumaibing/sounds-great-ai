import { useEffect, useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { getConcierge, patchConcierge } from '../../services/panelsService';
import type { ConciergeConfig } from '../../types/panels';
import {
  SettingsSection,
  SettingsText,
  SettingsPrimaryButton,
  SettingsStatusStrip,
} from './primitives';

const inputCls =
  'w-full rounded-[10px] border border-slate-800 bg-slate-950 px-3 py-1.5 text-compact leading-5 text-slate-200 placeholder:text-slate-600 outline-none transition focus:border-amber-500 focus:ring-1 focus:ring-amber-500/30';

// ConciergePanel (panels-roadmap P1): floating-ball appearance, persona and
// proactive strategy — persisted via PATCH /api/config/concierge.
export function ConciergePanel() {
  const { t } = useI18n();
  const [cfg, setCfg] = useState<ConciergeConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    getConcierge()
      .then(setCfg)
      .catch((e) => setError(String(e)));
  }, []);

  if (!cfg) {
    return error ? <SettingsStatusStrip tone="error">{error}</SettingsStatusStrip> : <SettingsText variant="xs" tone="muted">…</SettingsText>;
  }

  const save = async () => {
    setSaving(true);
    setError('');
    setSaved(false);
    try {
      const next = await patchConcierge(cfg);
      setCfg(next);
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      {error && <SettingsStatusStrip tone="error">{error}</SettingsStatusStrip>}

      <SettingsSection title={t('concierge.appearance')} description={t('concierge.desc')}>
        <div className="mt-2 flex items-start gap-6">
          {/* Live preview of the floating ball */}
          <div className="shrink-0 flex flex-col items-center gap-2 pt-1">
            <div
              className="rounded-full flex items-center justify-center shadow-lg border-2 border-white/10 transition-all"
              style={{ width: cfg.size, height: cfg.size, backgroundColor: cfg.color, fontSize: cfg.size * 0.45 }}
            >
              <span>{cfg.avatar}</span>
            </div>
            <SettingsText variant="micro" tone="muted">{cfg.size}px</SettingsText>
          </div>
          <div className="grid flex-1 gap-3 sm:grid-cols-2">
            <label className="space-y-1">
              <SettingsText variant="xs" tone="muted">{t('concierge.avatar')}</SettingsText>
              <input
                value={cfg.avatar}
                onChange={(e) => setCfg({ ...cfg, avatar: e.target.value })}
                maxLength={16}
                className={inputCls}
              />
            </label>
            <label className="space-y-1">
              <SettingsText variant="xs" tone="muted">{t('concierge.color')}</SettingsText>
              <div className="flex gap-2">
                <input
                  type="color"
                  value={cfg.color}
                  onChange={(e) => setCfg({ ...cfg, color: e.target.value })}
                  className="h-8 w-12 rounded border border-slate-800 bg-slate-950 cursor-pointer"
                />
                <input
                  value={cfg.color}
                  onChange={(e) => setCfg({ ...cfg, color: e.target.value })}
                  className={inputCls + ' font-mono'}
                />
              </div>
            </label>
            <label className="space-y-1 sm:col-span-2">
              <SettingsText variant="xs" tone="muted">
                {t('concierge.size')} {cfg.size}px
              </SettingsText>
              <input
                type="range"
                min={16}
                max={256}
                value={cfg.size}
                onChange={(e) => setCfg({ ...cfg, size: Number(e.target.value) })}
                className="w-full accent-amber-500"
              />
            </label>
          </div>
        </div>
      </SettingsSection>

      <SettingsSection title={t('concierge.personalityConfig')} description={t('concierge.personalityDefault')}>
        <div className="mt-2 space-y-3">
          <label className="block space-y-1">
            <SettingsText variant="xs" tone="muted">{t('concierge.personalityDesc')}</SettingsText>
            <textarea
              value={cfg.personality}
              onChange={(e) => setCfg({ ...cfg, personality: e.target.value })}
              rows={3}
              maxLength={2000}
              className={inputCls + ' resize-y'}
            />
          </label>
          <label className="block space-y-1">
            <SettingsText variant="xs" tone="muted">{t('concierge.greeting')}</SettingsText>
            <input
              value={cfg.greeting}
              onChange={(e) => setCfg({ ...cfg, greeting: e.target.value })}
              placeholder={t('concierge.greetingDefault')}
              maxLength={2000}
              className={inputCls}
            />
          </label>
          <label className="block space-y-1">
            <SettingsText variant="xs" tone="muted">{t('concierge.dutyDog')}</SettingsText>
            <input
              value={cfg.dutyBreed}
              onChange={(e) => setCfg({ ...cfg, dutyBreed: e.target.value })}
              placeholder="@bianmu"
              maxLength={64}
              className={inputCls + ' font-mono'}
            />
          </label>
        </div>
      </SettingsSection>

      <SettingsSection title={t('concierge.proactiveStrategy')} description="">
        <div className="mt-2 grid gap-3 sm:grid-cols-2">
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">
              {t('concierge.autoSuggestThreshold')} {cfg.autoSuggestThreshold} {t('concierge.autoSuggestSuffix')}
            </SettingsText>
            <input
              type="range"
              min={0}
              max={20}
              value={cfg.autoSuggestThreshold}
              onChange={(e) => setCfg({ ...cfg, autoSuggestThreshold: Number(e.target.value) })}
              className="w-full accent-amber-500"
            />
          </label>
          <div className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('concierge.proactiveLevel')}</SettingsText>
            <div className="flex gap-1">
              {(['low', 'medium', 'high'] as const).map((lvl) => (
                <button
                  key={lvl}
                  onClick={() => setCfg({ ...cfg, proactivityLevel: lvl })}
                  className={
                    'flex-1 px-2 py-1.5 rounded-lg text-xs transition ' +
                    (cfg.proactivityLevel === lvl
                      ? 'bg-amber-500/20 border border-amber-500/40 text-amber-300'
                      : 'border border-slate-800 text-slate-400 hover:text-slate-200')
                  }
                >
                  {t(`concierge.level${lvl[0].toUpperCase()}${lvl.slice(1)}`)}
                </button>
              ))}
            </div>
          </div>
        </div>
      </SettingsSection>

      <div className="flex items-center gap-3">
        <SettingsPrimaryButton onClick={() => void save()} disabled={saving}>
          {saving ? '…' : t('common.save')}
        </SettingsPrimaryButton>
        {saved && <SettingsText variant="xs" tone="muted">✓ {t('concierge.saved')}</SettingsText>}
      </div>
    </div>
  );
}
