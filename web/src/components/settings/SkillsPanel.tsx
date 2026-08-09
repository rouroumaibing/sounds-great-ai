import clsx from 'clsx';
import { useEffect, useState } from 'react';
import { apiGet, ApiError } from '../../services/http';
import { useI18n } from '../../store/useI18n';

type LoadedSkill = { name: string; source: string };

export function SkillsPanel() {
  const { t } = useI18n();
  const [loadedSkills, setLoadedSkills] = useState<LoadedSkill[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [disabledSkills, setDisabledSkills] = useState<Set<string>>(new Set());

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    apiGet<LoadedSkill[]>('/api/skills')
      .then((data) => {
        if (!cancelled) setLoadedSkills(data);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : t('common.error'));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const toggleSkill = (name: string) => {
    setDisabledSkills((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">{t('skills.title')}</h2>
        <p className="text-xs text-slate-400 mt-1">{t('skills.desc')}</p>
      </div>

      {loading ? (
        <div className="text-center py-12 text-slate-400 text-sm">{t('common.loading')}</div>
      ) : error ? (
        <div className="text-center py-12 text-rose-400 text-sm">{error}</div>
      ) : (
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-slate-800/80 text-slate-400">
                <th className="text-left px-4 py-3 font-semibold">{t('skills.name')}</th>
                <th className="text-left px-4 py-3 font-semibold">{t('skills.sourcePath')}</th>
                <th className="text-left px-4 py-3 font-semibold">{t('skills.status')}</th>
                <th className="text-right px-4 py-3 font-semibold">{t('skills.enabled')}</th>
              </tr>
            </thead>
            <tbody>
              {loadedSkills.map((skill) => {
                const isDisabled = disabledSkills.has(skill.name);
                return (
                  <tr key={skill.name} className="border-b border-slate-800/40 hover:bg-slate-800/20 transition">
                    <td className="px-4 py-3">
                      <div className="flex items-center space-x-2">
                        <i className="fa-solid fa-bolt text-amber-400 text-[10px]"></i>
                        <span className="font-mono text-slate-200">{skill.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 font-mono text-slate-400 text-[11px]">{skill.source}</td>
                    <td className="px-4 py-3">
                      <span className={clsx('px-2 py-0.5 rounded-lg border text-[10px] font-bold font-mono', isDisabled ? 'bg-slate-800 text-slate-500 border-slate-700' : 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30')}>
                        {isDisabled ? t('skills.disabledStatus') : 'loaded'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => toggleSkill(skill.name)}
                        className={clsx('w-11 h-6 rounded-full p-0.5 transition-colors relative focus:outline-none inline-block', !isDisabled ? 'bg-amber-600' : 'bg-slate-800')}
                      >
                        <div className={clsx('w-5 h-5 rounded-full bg-white shadow-md transform transition-transform', !isDisabled ? 'translate-x-5' : 'translate-x-0')}></div>
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
