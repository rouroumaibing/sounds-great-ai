import { useState, useEffect } from 'react';
import clsx from 'clsx';
import { apiGet, apiPost } from '../../services/http';

const FALLBACK_REPO_URL = 'https://github.com/rouroumaibing/sounds-great-ai';

export function AboutPanel() {
  const [mode, setMode] = useState<'source' | 'release' | ''>('');
  const [version, setVersion] = useState('v0.1.0');
  const [repoUrl, setRepoUrl] = useState(FALLBACK_REPO_URL);
  const [upgradeDialogOpen, setUpgradeDialogOpen] = useState(false);
  const [upgrading, setUpgrading] = useState(false);
  const [upgradeMsg, setUpgradeMsg] = useState('');

  useEffect(() => {
    apiGet<{mode: string, version: string, repo: string}>('/api/upgrade/info')
      .then((info) => {
        setMode(info.mode as 'source' | 'release');
        if (info.version) setVersion(info.version);
        if (info.repo) setRepoUrl(info.repo);
      })
      .catch(() => {
        // Fallback: assume source install (local dev without backend running)
        setMode('source');
      });
  }, []);

  const handleUpgrade = async (pull: boolean) => {
    setUpgradeDialogOpen(false);
    setUpgrading(true);
    setUpgradeMsg('');
    try {
      const result = await apiPost<{success: boolean, message: string, logs: string[]}>('/api/upgrade', { pull });
      setUpgradeMsg(result.message);
    } catch (e) {
      setUpgradeMsg(`升级失败: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      setUpgrading(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto w-full p-6 space-y-6">
      <h2 className="text-xl font-bold text-slate-100">关于</h2>

      {/* Project Info */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-4">
        <div className="flex items-center space-x-3">
          <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 via-purple-600 to-rose-500 flex items-center justify-center text-white font-bold shadow-lg">
            <i className="fa-solid fa-dog text-xl"></i>
          </div>
          <div>
            <h3 className="text-lg font-bold text-slate-100">sounds-great-ai</h3>
            <span className="text-xs font-mono px-2 py-0.5 rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">{version}</span>
          </div>
        </div>

        <div className="space-y-2 text-sm">
          <div className="flex items-center justify-between border-b border-slate-800/50 pb-2">
            <span className="text-slate-500">项目名称</span>
            <span className="text-slate-200 font-mono">sounds-great-ai</span>
          </div>
          <div className="flex items-center justify-between border-b border-slate-800/50 pb-2">
            <span className="text-slate-500">版本</span>
            <span className="text-slate-200 font-mono">{version}</span>
          </div>
          <div className="flex items-center justify-between border-b border-slate-800/50 pb-2">
            <span className="text-slate-500">开发者</span>
            <span className="text-slate-200 font-mono">rouroumaibing</span>
          </div>
          <div className="flex items-center justify-between border-b border-slate-800/50 pb-2">
            <span className="text-slate-500">安装方式</span>
            <span className={clsx('font-mono', mode === 'source' ? 'text-emerald-400' : mode === 'release' ? 'text-indigo-400' : 'text-slate-400')}>
              {mode === 'source' ? '源码安装' : mode === 'release' ? 'Release 包安装' : '检测中...'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-slate-500">仓库地址</span>
            <a href={repoUrl} target="_blank" rel="noopener noreferrer" className="text-indigo-400 hover:text-indigo-300 font-mono text-xs">
              {repoUrl.replace(/^https?:\/\//, '')}
            </a>
          </div>
        </div>
      </div>

      {/* Upgrade Section */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-4">
        <h3 className="text-base font-bold text-slate-100">升级</h3>
        <p className="text-sm text-slate-400">
          {mode === 'source'
            ? '拉取最新代码并重新构建前端和后端。'
            : mode === 'release'
            ? '下载最新 Release 包并替换当前二进制文件。'
            : '检测安装方式...'}
        </p>

        <button
          onClick={() => {
            if (mode === 'release') {
              handleUpgrade(false);
            } else {
              setUpgradeDialogOpen(true);
            }
          }}
          disabled={upgrading || !mode}
          className={clsx('px-4 py-2 rounded-lg text-sm font-medium transition', upgrading ? 'bg-amber-500/20 text-amber-400 border border-amber-500/50 animate-pulse' : 'bg-amber-500 text-white hover:bg-amber-400', !mode && 'opacity-50 cursor-not-allowed')}
        >
          {upgrading ? '升级中...' : '升级'}
        </button>

        {upgradeMsg && (
          <div className={clsx('text-sm font-mono p-3 rounded-lg border', upgradeMsg.includes('失败') ? 'text-rose-400 bg-rose-500/10 border-rose-500/30' : 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30')}>
            {upgradeMsg}
          </div>
        )}
      </div>

      {/* Upgrade Confirmation Dialog (source mode only) */}
      {upgradeDialogOpen && mode === 'source' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl p-6 w-96 space-y-4">
            <h3 className="text-lg font-bold text-slate-100">升级</h3>
            <p className="text-sm text-slate-400">是否需要拉取最新的代码？</p>
            <div className="flex justify-end space-x-2 pt-2">
              <button
                onClick={() => setUpgradeDialogOpen(false)}
                className="px-3 py-1.5 rounded-lg text-sm text-slate-400 hover:text-slate-200 border border-slate-700 hover:border-slate-500 transition"
              >
                取消
              </button>
              <button
                onClick={() => handleUpgrade(false)}
                className="px-3 py-1.5 rounded-lg text-sm text-slate-200 bg-slate-700 hover:bg-slate-600 transition"
              >
                否，仅重新构建
              </button>
              <button
                onClick={() => handleUpgrade(true)}
                className="px-3 py-1.5 rounded-lg text-sm text-white bg-amber-500 hover:bg-amber-400 transition"
              >
                是，拉取最新代码
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
