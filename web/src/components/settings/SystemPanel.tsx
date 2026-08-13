import { useEffect, useState } from 'react';
import { apiGet, apiPut, apiPost, ApiError } from '../../services/http';
import {
  SettingsSection,
  SettingsBadge,
  SettingsText,
  SettingsPrimaryButton,
  SettingsSecondaryButton,
  SettingsStatusStrip,
} from './primitives';

interface RepoConfig {
  repo_url: string;
}

interface TestResult {
  ok: boolean;
  error?: string;
  branches_collected?: number;
}

export function SystemPanel() {
  const [repoURL, setRepoURL] = useState('');
  const [saved, setSaved] = useState('');
  const [saving, setSaving] = useState(false);
  const [test, setTest] = useState<TestResult | null>(null);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    apiGet<RepoConfig>('/api/config/repo')
      .then((cfg) => setRepoURL(cfg.repo_url ?? ''))
      .catch((e) => setError(String(e)));
  }, []);

  const onSave = async () => {
    setSaving(true);
    setError('');
    try {
      const res = await apiPut<RepoConfig>('/api/config/repo', { repo_url: repoURL.trim() });
      setSaved(res.repo_url);
      setTest(null);
    } catch (e) {
      setError(e instanceof ApiError ? `保存失败（${e.status}）` : String(e));
    } finally {
      setSaving(false);
    }
  };

  const onTest = async () => {
    setTesting(true);
    setTest(null);
    setError('');
    try {
      const res = await apiPost<TestResult>('/api/repo/test', {});
      setTest(res);
    } catch (e) {
      setTest({ ok: false, error: String(e) });
    } finally {
      setTesting(false);
    }
  };

  const repoActive = repoURL.trim().length > 0;

  return (
    <div className="space-y-6">
      {/* 项目归档源 */}
      <SettingsSection
        title="项目归档源"
        description="用于狗狗开发讨论功能回溯——把每一次多犬协作的讨论，与其关联的代码库活动拼成一条可回溯的时间轴。"
        badge={<SettingsBadge tone="amber">Project archive source</SettingsBadge>}
      >
        <div className="mt-1">
          <label className="block text-xs font-semibold text-slate-300">代码库地址</label>
          <SettingsText variant="xs" tone="muted" className="mt-1">
            可选。留空则不启用 git 轨迹；填入后将自动并入代码库 git 分支活动。
          </SettingsText>
          <div className="mt-3 flex items-center gap-2">
            <input
              type="text"
              value={repoURL}
              onChange={(e) => setRepoURL(e.target.value)}
              placeholder="https://github.com/you/your-repo.git（可选，留空则不启用 git 轨迹）"
              className="flex-1 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 outline-none placeholder:text-slate-500 focus:border-amber-500"
            />
            <SettingsPrimaryButton onClick={onSave} disabled={saving}>
              {saving ? '保存中…' : '保存'}
            </SettingsPrimaryButton>
            <SettingsSecondaryButton onClick={onTest} disabled={testing || !repoActive}>
              {testing ? '测试中…' : '测试连接'}
            </SettingsSecondaryButton>
          </div>

          <div className="mt-4">
            <SettingsText variant="sm" tone="secondary" className="mb-2">
              归档数据源
            </SettingsText>
            <div className="flex flex-wrap gap-2">
              <SettingsBadge tone="amber">球权事件流（custody event log）</SettingsBadge>
              <SettingsBadge tone="blue">聊天消息（MessageStore）</SettingsBadge>
              <SettingsBadge tone={repoActive ? 'purple' : 'slate'}>
                代码库 git 分支{repoActive ? '' : '（未配置）'}
              </SettingsBadge>
            </div>
          </div>

          {saved !== '' && (
            <div className="mt-4">
              <SettingsStatusStrip tone={saved ? 'success' : 'muted'}>
                {saved
                  ? `已保存代码库地址，git 轨迹已启用：${saved}`
                  : '已清空代码库地址，git 轨迹已禁用（仅球权事件流 + 聊天消息两源）。'}
              </SettingsStatusStrip>
            </div>
          )}
          {test && (
            <div className="mt-3">
              <SettingsStatusStrip tone={test.ok ? 'success' : 'error'}>
                {test.ok
                  ? `连接成功，采集到 ${test.branches_collected ?? 0} 个分支事件。`
                  : `连接失败：${test.error ?? '未知错误'}`}
              </SettingsStatusStrip>
            </div>
          )}
          {error && (
            <div className="mt-3">
              <SettingsStatusStrip tone="error">{error}</SettingsStatusStrip>
            </div>
          )}
        </div>
      </SettingsSection>

      {/* 运行时总开关 */}
      <SettingsSection
        title="运行时总开关"
        description="与 clowder「系统配置」分区同定位：默认行为与全局开关集中在此，不再混入账户分区。"
      >
        <div className="mt-1 space-y-2">
          <div className="flex items-center justify-between rounded-xl border border-slate-800/80 bg-slate-900/60 px-4 py-3">
            <div>
              <SettingsText variant="sm" tone="default" className="font-semibold">
                跨犬 handoff 深度
              </SettingsText>
              <SettingsText variant="xs" tone="muted" className="mt-0.5">
                默认 3 · 高于乒乓熔断阈值时 floor 到 8
              </SettingsText>
            </div>
            <SettingsBadge tone="slate">3</SettingsBadge>
          </div>
          <div className="flex items-center justify-between rounded-xl border border-slate-800/80 bg-slate-900/60 px-4 py-3">
            <div>
              <SettingsText variant="sm" tone="default" className="font-semibold">
                命令唤醒超时
              </SettingsText>
              <SettingsText variant="xs" tone="muted" className="mt-0.5">
                持球命令唤醒（wakeWhen）的超时回收
              </SettingsText>
            </div>
            <SettingsBadge tone="slate">5 min</SettingsBadge>
          </div>
        </div>
      </SettingsSection>
    </div>
  );
}
