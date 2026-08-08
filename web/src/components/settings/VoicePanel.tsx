import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { apiGet, apiPatch } from '../../services/http';

interface GlossaryEntry {
  key: string;
  value: string;
}

export function VoicePanel() {
  const [enabled, setEnabled] = useState(false);
  const [serviceHealth, setServiceHealth] = useState<'ok' | 'down' | 'unknown'>('unknown');

  // TTS config
  const [ttsVoice, setTtsVoice] = useState('alloy');
  const [ttsLang, setTtsLang] = useState('zh-CN');
  const [ttsSpeed, setTtsSpeed] = useState(1.0);
  const [ttsRefAudio, setTtsRefAudio] = useState('');

  // STT config
  const [sttModel, setSttModel] = useState('whisper-1');
  const [sttLanguage, setSttLanguage] = useState('zh');
  const [sttAutoTranscribe, setSttAutoTranscribe] = useState(true);

  // Glossary
  const [glossary, setGlossary] = useState<GlossaryEntry[]>([]);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  const [testResult, setTestResult] = useState('');

  useEffect(() => {
    apiGet<Record<string, unknown>>('/api/config/voice').then((data) => {
      if (typeof data.enabled === 'boolean') setEnabled(data.enabled);
      if (typeof data.ttsVoice === 'string') setTtsVoice(data.ttsVoice);
      if (typeof data.ttsLang === 'string') setTtsLang(data.ttsLang);
      if (typeof data.ttsSpeed === 'number') setTtsSpeed(data.ttsSpeed);
      if (typeof data.ttsRefAudio === 'string') setTtsRefAudio(data.ttsRefAudio);
      if (typeof data.sttModel === 'string') setSttModel(data.sttModel);
      if (typeof data.sttLanguage === 'string') setSttLanguage(data.sttLanguage);
      if (typeof data.sttAutoTranscribe === 'boolean') setSttAutoTranscribe(data.sttAutoTranscribe);
      if (Array.isArray(data.glossary)) setGlossary(data.glossary as GlossaryEntry[]);
    }).catch(() => {});
  }, []);

  const handleTestTTS = () => {
    setTestResult('TTS 测试: "你好，犬队已就绪。" 🔊');
    setTimeout(() => setTestResult(''), 3000);
  };

  const handleTestSTT = () => {
    setTestResult('STT 测试: 请对着麦克风说话... 🎤');
    setTimeout(() => setTestResult(''), 3000);
  };

  const addGlossary = () => {
    if (!newKey || !newValue) return;
    setGlossary((prev) => [...prev, { key: newKey, value: newValue }]);
    setNewKey('');
    setNewValue('');
  };

  const removeGlossary = (idx: number) => {
    setGlossary((prev) => prev.filter((_, i) => i !== idx));
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">语音管理</h2>
        <p className="text-xs text-slate-400 mt-1">TTS/STT 配置与术语表管理。</p>
      </div>

      {/* 语音服务状态 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className={clsx('w-10 h-10 rounded-xl flex items-center justify-center', enabled ? 'bg-emerald-500/20' : 'bg-slate-800')}>
              <i className={clsx('fa-solid fa-microphone text-sm', enabled ? 'text-emerald-400' : 'text-slate-500')}></i>
            </div>
            <div>
              <div className="text-xs font-bold text-slate-200">语音服务</div>
              <div className="text-[11px] text-slate-500 mt-0.5">
                {enabled ? `运行中 · ${serviceHealth === 'ok' ? '健康' : serviceHealth === 'down' ? '异常' : '未知'}` : '已禁用'}
              </div>
            </div>
          </div>
          <button onClick={() => { setEnabled(!enabled); setServiceHealth(!enabled ? 'ok' : 'unknown'); }} className={clsx('relative w-10 h-5 rounded-full transition', enabled ? 'bg-indigo-500' : 'bg-slate-700')}>
            <span className={clsx('absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform', enabled ? 'translate-x-5' : 'translate-x-0.5')} />
          </button>
        </div>
      </div>

      {/* TTS 配置 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-2">
            <i className="fa-solid fa-volume-high text-cyan-400 text-xs"></i>
            <h4 className="text-xs font-bold text-slate-200">TTS 配置 (文本转语音)</h4>
          </div>
          <button onClick={handleTestTTS} disabled={!enabled} className="px-3 py-1 rounded-lg bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[11px] font-semibold hover:bg-cyan-500/30 transition disabled:opacity-50">
            测试
          </button>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="语音" value={ttsVoice} onChange={setTtsVoice} options={['alloy', 'echo', 'fable', 'onyx', 'nova', 'shimmer']} />
          <Field label="语言代码" value={ttsLang} onChange={setTtsLang} options={['zh-CN', 'en-US', 'ja-JP']} />
        </div>
        <div>
          <label className="text-[11px] text-slate-400">语速: {ttsSpeed.toFixed(1)}x</label>
          <input type="range" min={0.5} max={2.0} step={0.1} value={ttsSpeed} onChange={(e) => setTtsSpeed(Number(e.target.value))} className="w-full mt-1.5 accent-cyan-500" />
        </div>
        <div>
          <label className="text-[11px] text-slate-400">参考音频 URL (可选)</label>
          <input type="text" value={ttsRefAudio} onChange={(e) => setTtsRefAudio(e.target.value)} placeholder="https://..." className="w-full mt-1 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] font-mono text-slate-200 focus:border-cyan-500/50 transition" />
        </div>
      </div>

      {/* STT 配置 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-2">
            <i className="fa-solid fa-microphone-lines text-amber-400 text-xs"></i>
            <h4 className="text-xs font-bold text-slate-200">STT 配置 (语音转文本)</h4>
          </div>
          <button onClick={handleTestSTT} disabled={!enabled} className="px-3 py-1 rounded-lg bg-amber-500/20 text-amber-300 border border-amber-500/30 text-[11px] font-semibold hover:bg-amber-500/30 transition disabled:opacity-50">
            测试
          </button>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="模型" value={sttModel} onChange={setSttModel} options={['whisper-1', 'whisper-large-v3']} />
          <Field label="语言" value={sttLanguage} onChange={setSttLanguage} options={['zh', 'en', 'ja', 'auto']} />
        </div>
        <div className="flex items-center justify-between">
          <span className="text-[11px] text-slate-400">自动转写</span>
          <button onClick={() => setSttAutoTranscribe(!sttAutoTranscribe)} className={clsx('relative w-10 h-5 rounded-full transition', sttAutoTranscribe ? 'bg-indigo-500' : 'bg-slate-700')}>
            <span className={clsx('absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform', sttAutoTranscribe ? 'translate-x-5' : 'translate-x-0.5')} />
          </button>
        </div>
      </div>

      {/* 术语表 */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4 space-y-3">
        <div className="flex items-center space-x-2">
          <i className="fa-solid fa-book text-purple-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">术语表</h4>
        </div>
        <div className="divide-y divide-slate-800/40">
          {glossary.map((entry, idx) => (
            <div key={idx} className="py-2 flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <span className="text-[11px] font-mono text-indigo-300">{entry.key}</span>
                <i className="fa-solid fa-arrow-right text-slate-600 text-[9px]"></i>
                <span className="text-[11px] text-slate-300">{entry.value}</span>
              </div>
              <button onClick={() => removeGlossary(idx)} className="text-rose-400 text-[11px] hover:text-rose-300"><i className="fa-solid fa-xmark"></i></button>
            </div>
          ))}
        </div>
        <div className="flex items-center space-x-2 pt-2 border-t border-slate-800/40">
          <input type="text" value={newKey} onChange={(e) => setNewKey(e.target.value)} placeholder="术语" className="flex-1 px-2 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] font-mono text-slate-200 focus:border-purple-500/50 transition" />
          <input type="text" value={newValue} onChange={(e) => setNewValue(e.target.value)} placeholder="释义" className="flex-1 px-2 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] text-slate-200 focus:border-purple-500/50 transition" />
          <button onClick={addGlossary} disabled={!newKey || !newValue} className="px-3 py-1.5 rounded-lg bg-purple-500 text-white text-[11px] font-semibold hover:bg-purple-400 transition disabled:opacity-50">
            添加
          </button>
        </div>
      </div>

      {testResult && (
        <div className="text-xs font-mono p-3 rounded-xl border border-cyan-500/30 bg-cyan-500/10 text-cyan-300">
          {testResult}
        </div>
      )}

      <div className="flex justify-end">
        <button onClick={() => apiPatch('/api/config/voice', { enabled, ttsVoice, ttsLang, ttsSpeed, ttsRefAudio, sttModel, sttLanguage, sttAutoTranscribe, glossary }).catch(() => {})} className="px-4 py-2 rounded-xl bg-indigo-500 text-white text-xs font-semibold hover:bg-indigo-400 transition">
          保存配置
        </button>
      </div>
    </div>
  );
}

function Field({ label, value, onChange, options }: { label: string; value: string; onChange: (v: string) => void; options: string[] }) {
  return (
    <div>
      <label className="text-[11px] text-slate-400">{label}</label>
      <select value={value} onChange={(e) => onChange(e.target.value)} className="w-full mt-1 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] text-slate-200 focus:border-indigo-500/50 transition">
        {options.map((o) => <option key={o} value={o}>{o}</option>)}
      </select>
    </div>
  );
}
