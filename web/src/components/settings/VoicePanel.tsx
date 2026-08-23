import { useEffect, useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { getVoice, patchVoice } from '../../services/panelsService';
import type { VoiceConfig } from '../../types/panels';
import {
  SettingsSection,
  SettingsText,
  SettingsBadge,
  SettingsPrimaryButton,
  SettingsSecondaryButton,
  SettingsStatusStrip,
} from './primitives';

const inputCls =
  'w-full rounded-[10px] border border-slate-800 bg-slate-950 px-3 py-1.5 text-compact leading-5 text-slate-200 placeholder:text-slate-600 outline-none transition focus:border-amber-500 focus:ring-1 focus:ring-amber-500/30';

// VoicePanel (panels-roadmap P1): TTS/STT configuration + glossary. SG stores
// configuration only — no inference runs here.
export function VoicePanel() {
  const { t } = useI18n();
  const [cfg, setCfg] = useState<VoiceConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    getVoice()
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
      const next = await patchVoice({
        ...cfg,
        glossary: cfg.glossary.filter((g) => g.source.trim() || g.target.trim()),
      });
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

      <SettingsSection title={t('voice.service')} description={t('voice.desc')}>
        <div className="mt-2 flex items-center gap-3">
          <button
            onClick={() => setCfg({ ...cfg, enabled: !cfg.enabled })}
            className={
              'relative w-11 h-6 rounded-full transition ' + (cfg.enabled ? 'bg-emerald-500/80' : 'bg-slate-700')
            }
            aria-pressed={cfg.enabled}
          >
            <span
              className={
                'absolute top-0.5 w-5 h-5 rounded-full bg-white shadow transition-all ' +
                (cfg.enabled ? 'left-[22px]' : 'left-0.5')
              }
            />
          </button>
          <SettingsBadge tone={cfg.enabled ? 'emerald' : 'slate'}>
            {cfg.enabled ? t('voice.running') : t('voice.disabled')}
          </SettingsBadge>
          <SettingsBadge tone={cfg.enabled ? 'emerald' : 'amber'}>
            {cfg.enabled ? t('voice.healthy') : t('voice.unknown')}
          </SettingsBadge>
        </div>
      </SettingsSection>

      <SettingsSection title={t('voice.ttsConfig')} description="">
        <div className="mt-2 grid gap-3 sm:grid-cols-2">
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('voice.voice')}</SettingsText>
            <input
              value={cfg.ttsVoice}
              onChange={(e) => setCfg({ ...cfg, ttsVoice: e.target.value })}
              maxLength={64}
              placeholder="zh-CN-Yunxi"
              className={inputCls + ' font-mono'}
            />
          </label>
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('voice.langCode')}</SettingsText>
            <input
              value={cfg.ttsLang}
              onChange={(e) => setCfg({ ...cfg, ttsLang: e.target.value })}
              maxLength={16}
              placeholder="zh-CN"
              className={inputCls + ' font-mono'}
            />
          </label>
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('voice.speed')} {cfg.ttsSpeed.toFixed(2)}x</SettingsText>
            <input
              type="range"
              min={0.25}
              max={4}
              step={0.05}
              value={cfg.ttsSpeed}
              onChange={(e) => setCfg({ ...cfg, ttsSpeed: Number(e.target.value) })}
              className="w-full accent-amber-500"
            />
          </label>
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('voice.refAudio')}</SettingsText>
            <input
              value={cfg.ttsRefAudio}
              onChange={(e) => setCfg({ ...cfg, ttsRefAudio: e.target.value })}
              maxLength={512}
              placeholder="https://…"
              className={inputCls + ' font-mono'}
            />
          </label>
        </div>
      </SettingsSection>

      <SettingsSection title={t('voice.sttConfig')} description="">
        <div className="mt-2 grid gap-3 sm:grid-cols-2">
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('voice.model')}</SettingsText>
            <input
              value={cfg.sttModel}
              onChange={(e) => setCfg({ ...cfg, sttModel: e.target.value })}
              maxLength={64}
              placeholder="whisper-1"
              className={inputCls + ' font-mono'}
            />
          </label>
          <label className="space-y-1">
            <SettingsText variant="xs" tone="muted">{t('voice.language')}</SettingsText>
            <input
              value={cfg.sttLanguage}
              onChange={(e) => setCfg({ ...cfg, sttLanguage: e.target.value })}
              maxLength={16}
              placeholder="zh"
              className={inputCls + ' font-mono'}
            />
          </label>
          <label className="flex items-center gap-2 sm:col-span-2">
            <input
              type="checkbox"
              checked={cfg.sttAutoTranscribe}
              onChange={(e) => setCfg({ ...cfg, sttAutoTranscribe: e.target.checked })}
              className="accent-amber-500"
            />
            <SettingsText variant="xs" tone="muted">{t('voice.autoTranscribe')}</SettingsText>
          </label>
        </div>
      </SettingsSection>

      <SettingsSection title={t('voice.glossary')} description="">
        <div className="mt-2 space-y-2">
          {cfg.glossary.map((entry, i) => (
            <div key={i} className="flex items-center gap-2">
              <input
                value={entry.source}
                onChange={(e) => {
                  const glossary = [...cfg.glossary];
                  glossary[i] = { ...entry, source: e.target.value };
                  setCfg({ ...cfg, glossary });
                }}
                placeholder={t('voice.termPlaceholder')}
                maxLength={200}
                className={inputCls + ' font-mono'}
              />
              <span className="text-slate-600">→</span>
              <input
                value={entry.target}
                onChange={(e) => {
                  const glossary = [...cfg.glossary];
                  glossary[i] = { ...entry, target: e.target.value };
                  setCfg({ ...cfg, glossary });
                }}
                placeholder={t('voice.defPlaceholder')}
                maxLength={200}
                className={inputCls + ' font-mono'}
              />
              <button
                onClick={() => setCfg({ ...cfg, glossary: cfg.glossary.filter((_, j) => j !== i) })}
                className="p-1 rounded text-slate-600 hover:text-rose-400 transition"
                title={t('common.delete')}
              >
                <i className="fa-regular fa-trash-can text-xs"></i>
              </button>
            </div>
          ))}
          <SettingsSecondaryButton onClick={() => setCfg({ ...cfg, glossary: [...cfg.glossary, { source: '', target: '' }] })}>
            <i className="fa-solid fa-plus mr-1 text-[10px]"></i>{t('common.add')}
          </SettingsSecondaryButton>
        </div>
      </SettingsSection>

      <div className="flex items-center gap-3">
        <SettingsPrimaryButton onClick={() => void save()} disabled={saving}>
          {saving ? '…' : t('common.save')}
        </SettingsPrimaryButton>
        {saved && <SettingsText variant="xs" tone="muted">✓ {t('voice.saved')}</SettingsText>}
      </div>
    </div>
  );
}
