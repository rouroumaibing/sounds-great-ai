import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { apiGet, apiPatch } from '../../services/http';
import { useBreeds } from '../../hooks/useBreeds';
import { useI18n } from '../../store/useI18n';

const AVATAR_OPTIONS = ['🐕', '🦮', '🐩', '🐕‍🦺', '🐺', '🦊'];
const COLOR_OPTIONS = ['#4A90D9', '#E84393', '#F39C12', '#27AE60', '#8E44AD', '#2C3E50'];

export function ConciergePanel() {
  const { t } = useI18n();
  const { dogs } = useBreeds();
  const [avatar, setAvatar] = useState('🐕');
  const [color, setColor] = useState('#4A90D9');
  const [size, setSize] = useState(56);
  const [personality, setPersonality] = useState(t('concierge.personalityDefault'));
  const [greeting, setGreeting] = useState(t('concierge.greetingDefault'));
  const [dutyBreed, setDutyBreed] = useState('bianmu');
  const [autoSuggestThreshold, setAutoSuggestThreshold] = useState(3);
  const [proactivityLevel, setProactivityLevel] = useState<'low' | 'medium' | 'high'>('medium');

  useEffect(() => {
    apiGet<Record<string, unknown>>('/api/config/concierge').then((data) => {
      if (typeof data.avatar === 'string') setAvatar(data.avatar);
      if (typeof data.color === 'string') setColor(data.color);
      if (typeof data.size === 'number') setSize(data.size);
      if (typeof data.personality === 'string') setPersonality(data.personality);
      if (typeof data.greeting === 'string') setGreeting(data.greeting);
      if (typeof data.dutyBreed === 'string') setDutyBreed(data.dutyBreed);
      if (typeof data.autoSuggestThreshold === 'number') setAutoSuggestThreshold(data.autoSuggestThreshold);
      if (typeof data.proactivityLevel === 'string') setProactivityLevel(data.proactivityLevel as 'low' | 'medium' | 'high');
    }).catch(() => {});
  }, []);

  const saveConfig = () => {
    apiPatch('/api/config/concierge', { avatar, color, size, personality, greeting, dutyBreed, autoSuggestThreshold, proactivityLevel }).catch(() => {});
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">{t('concierge.title')}</h2>
        <p className="text-xs text-slate-400 mt-1">{t('concierge.desc')}</p>
      </div>

      {/* 悬浮球形象配置 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-4">
        <div className="flex items-center space-x-2">
          <i className="fa-solid fa-circle-dot text-indigo-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('concierge.appearance')}</h4>
        </div>

        {/* Preview */}
        <div className="flex items-center justify-center py-4">
          <div
            className="rounded-full flex items-center justify-center shadow-lg"
            style={{ width: size, height: size, backgroundColor: color + '30', border: `2px solid ${color}` }}
          >
            <span style={{ fontSize: size * 0.5 }}>{avatar}</span>
          </div>
        </div>

        {/* Avatar picker */}
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.avatar')}</label>
          <div className="flex items-center space-x-2 mt-1.5">
            {AVATAR_OPTIONS.map((a) => (
              <button
                key={a}
                onClick={() => setAvatar(a)}
                className={clsx('w-9 h-9 rounded-xl flex items-center justify-center text-lg transition', avatar === a ? 'bg-indigo-500/20 border border-indigo-500/50' : 'bg-slate-800 border border-slate-700/50 hover:bg-slate-700')}
              >
                {a}
              </button>
            ))}
          </div>
        </div>

        {/* Color picker */}
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.color')}</label>
          <div className="flex items-center space-x-2 mt-1.5">
            {COLOR_OPTIONS.map((c) => (
              <button
                key={c}
                onClick={() => setColor(c)}
                className={clsx('w-7 h-7 rounded-full transition', color === c ? 'ring-2 ring-white ring-offset-2 ring-offset-slate-900' : '')}
                style={{ backgroundColor: c }}
              />
            ))}
          </div>
        </div>

        {/* Size slider */}
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.size')} {size}px</label>
          <input
            type="range"
            min={40}
            max={80}
            value={size}
            onChange={(e) => setSize(Number(e.target.value))}
            className="w-full mt-1.5 accent-indigo-500"
          />
        </div>
      </div>

      {/* 人设配置 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
        <div className="flex items-center space-x-2">
          <i className="fa-solid fa-user-pen text-amber-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('concierge.personalityConfig')}</h4>
        </div>
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.personalityDesc')}</label>
          <textarea
            value={personality}
            onChange={(e) => setPersonality(e.target.value)}
            rows={2}
            className="w-full mt-1 px-3 py-2 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] text-slate-200 focus:border-indigo-500/50 transition resize-none"
          />
        </div>
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.greeting')}</label>
          <input
            type="text"
            value={greeting}
            onChange={(e) => setGreeting(e.target.value)}
            className="w-full mt-1 px-3 py-2 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] text-slate-200 focus:border-indigo-500/50 transition"
          />
        </div>
      </div>

      {/* 值班犬选择 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
        <div className="flex items-center space-x-2">
          <i className="fa-solid fa-dog text-emerald-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('concierge.dutyDog')}</h4>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
          {dogs.map((b) => (
            <button
              key={b.id}
              onClick={() => setDutyBreed(b.id)}
              className={clsx('px-3 py-2 rounded-xl flex items-center space-x-2 transition', dutyBreed === b.id ? 'bg-emerald-500/20 border border-emerald-500/40' : 'bg-slate-800 border border-slate-700/50 hover:bg-slate-700')}
            >
              <i className={clsx(b.icon, 'text-xs', dutyBreed === b.id ? 'text-emerald-400' : 'text-slate-400')}></i>
              <span className={clsx('text-[11px] font-semibold', dutyBreed === b.id ? 'text-emerald-300' : 'text-slate-300')}>{b.name}</span>
            </button>
          ))}
        </div>
      </div>

      {/* 主动性策略 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
        <div className="flex items-center space-x-2">
          <i className="fa-solid fa-wand-magic-sparkles text-purple-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('concierge.proactiveStrategy')}</h4>
        </div>
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.autoSuggestThreshold')} {autoSuggestThreshold} {t('concierge.autoSuggestSuffix')}</label>
          <input
            type="range"
            min={1}
            max={10}
            value={autoSuggestThreshold}
            onChange={(e) => setAutoSuggestThreshold(Number(e.target.value))}
            className="w-full mt-1.5 accent-purple-500"
          />
        </div>
        <div>
          <label className="text-[11px] text-slate-400">{t('concierge.proactiveLevel')}</label>
          <div className="flex items-center space-x-2 mt-1.5">
            {(['low', 'medium', 'high'] as const).map((level) => (
              <button
                key={level}
                onClick={() => setProactivityLevel(level)}
                className={clsx('px-4 py-1.5 rounded-xl text-[11px] font-semibold transition', proactivityLevel === level ? 'bg-purple-500 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}
              >
                {level === 'low' ? t('concierge.levelLow') : level === 'medium' ? t('concierge.levelMedium') : t('concierge.levelHigh')}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="flex justify-end">
        <button onClick={saveConfig} className="px-4 py-2 rounded-xl bg-indigo-500 text-white text-xs font-semibold hover:bg-indigo-400 transition">
          {t('common.saveConfig')}
        </button>
      </div>
    </div>
  );
}
