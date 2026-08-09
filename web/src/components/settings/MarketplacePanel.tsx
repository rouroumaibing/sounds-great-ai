import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { apiGet } from '../../services/http';
import { useI18n } from '../../store/useI18n';

interface Package {
  id: string;
  name: string;
  author: string;
  description: string;
  category: 'MCP' | 'Skill' | 'Plugin' | 'Breed';
  installs: number;
  rating: number;
  installed: boolean;
}

const CATEGORIES = ['all', 'MCP', 'Skill', 'Plugin', 'Breed'] as const;

export function MarketplacePanel() {
  const { t } = useI18n();
  const [packages, setPackages] = useState<Package[]>([]);
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState<string>('all');
  const [tab, setTab] = useState<'browse' | 'installed'>('browse');

  useEffect(() => {
    apiGet<Package[]>('/api/marketplace').then((data) => setPackages(Array.isArray(data) ? data : [])).catch(() => setPackages([]));
  }, []);

  const filtered = packages.filter((p) => {
    if (tab === 'installed' && !p.installed) return false;
    if (category !== 'all' && p.category !== category) return false;
    if (search && !p.name.toLowerCase().includes(search.toLowerCase()) && !p.description.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const handleInstall = (id: string) => {
    setPackages((prev) => prev.map((p) => p.id === id ? { ...p, installed: true } : p));
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">{t('settings.marketplace')}</h2>
        <p className="text-xs text-slate-400 mt-1">{t('marketplace.desc')}</p>
      </div>

      {/* Tabs */}
      <div className="flex items-center space-x-1">
        <button onClick={() => setTab('browse')} className={clsx('px-4 py-1.5 rounded-xl text-[11px] font-semibold transition', tab === 'browse' ? 'bg-indigo-500 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}>
          {t('common.browse')}
        </button>
        <button onClick={() => setTab('installed')} className={clsx('px-4 py-1.5 rounded-xl text-[11px] font-semibold transition', tab === 'installed' ? 'bg-indigo-500 text-white' : 'bg-slate-800 text-slate-400 hover:text-slate-200')}>
          {t('marketplace.installed').replace('{count}', String(packages.filter(p => p.installed).length))}
        </button>
      </div>

      {/* Search + filter */}
      <div className="flex items-center space-x-3">
        <div className="flex-1 relative">
          <i className="fa-solid fa-search absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 text-xs"></i>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('marketplace.searchPlaceholder')}
            className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-900/60 border border-slate-800/80 text-xs text-slate-200 focus:border-indigo-500/50 transition"
          />
        </div>
        <div className="flex items-center space-x-1">
          {CATEGORIES.map((cat) => (
            <button
              key={cat}
              onClick={() => setCategory(cat)}
              className={clsx('px-3 py-1.5 rounded-xl text-[11px] font-semibold transition', category === cat ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30' : 'text-slate-400 hover:text-slate-200')}
            >
              {cat === 'all' ? t('common.all') : cat}
            </button>
          ))}
        </div>
      </div>

      {/* Package cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {filtered.map((pkg) => (
          <div key={pkg.id} className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-2">
            <div className="flex items-start justify-between">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold text-slate-200">{pkg.name}</span>
                  <span className={clsx('text-[9px] font-mono px-1.5 py-0.5 rounded border', categoryColors[pkg.category])}>
                    {pkg.category}
                  </span>
                </div>
                <div className="text-[11px] text-slate-500 mt-0.5">by {pkg.author}</div>
              </div>
              <div className="flex items-center space-x-1 shrink-0">
                <span className="text-[10px] text-amber-400"><i className="fa-solid fa-star"></i> {pkg.rating}</span>
              </div>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">{pkg.description}</p>
            <div className="flex items-center justify-between pt-1">
              <span className="text-[10px] text-slate-500"><i className="fa-solid fa-download mr-1"></i>{pkg.installs.toLocaleString()} {t('marketplace.installs')}</span>
              {pkg.installed ? (
                <span className="px-3 py-1 rounded-lg bg-emerald-500/20 text-emerald-300 text-[11px] font-semibold">
                  <i className="fa-solid fa-check mr-1"></i>{t('marketplace.alreadyInstalled')}
                </span>
              ) : (
                <button onClick={() => handleInstall(pkg.id)} className="px-3 py-1 rounded-lg bg-indigo-500 text-white text-[11px] font-semibold hover:bg-indigo-400 transition">
                  {t('common.install')}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {filtered.length === 0 && (
        <div className="p-8 rounded-2xl bg-slate-900/40 border border-slate-800 text-center">
          <i className="fa-solid fa-store text-2xl text-slate-600"></i>
          <p className="text-xs text-slate-500 mt-2">{t('marketplace.notFound')}</p>
        </div>
      )}
    </div>
  );
}

const categoryColors: Record<string, string> = {
  MCP: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30',
  Skill: 'bg-amber-500/20 text-amber-300 border-amber-500/30',
  Plugin: 'bg-purple-500/20 text-purple-300 border-purple-500/30',
  Breed: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30',
};
