import { useRef, useState } from 'react';
import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';
import { MentionPopover } from './MentionPopover';

export function CommandBar() {
  const userPromptInput = useAppStore((s) => s.userPromptInput);
  const setUserPromptInput = useAppStore((s) => s.setUserPromptInput);
  const mentionOpen = useAppStore((s) => s.mentionOpen);
  const setMentionOpen = useAppStore((s) => s.setMentionOpen);
  const setMentionQuery = useAppStore((s) => s.setMentionQuery);
  const activeThreadId = useAppStore((s) => s.activeThreadId);

  const sendPrompt = useChatStore((s) => s.sendPrompt);
  const wsReadyState = useChatStore((s) => s.wsReadyState);
  const isGenerating = useChatStore((s) => s.isGenerating[activeThreadId] ?? false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [isComposing, setIsComposing] = useState(false);

  const wsConnected = wsReadyState === WebSocket.OPEN;

  const autoResize = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 144) + 'px';
  };

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value;
    setUserPromptInput(value);
    autoResize();

    const lastChar = value[value.length - 1];
    if (lastChar === '@') {
      setMentionOpen(true);
      setMentionQuery('');
    } else if (mentionOpen && lastChar !== ' ') {
      const atIdx = value.lastIndexOf('@');
      if (atIdx !== -1) {
        setMentionQuery(value.slice(atIdx + 1));
      }
    } else if (lastChar === ' ' || lastChar === '\n') {
      setMentionOpen(false);
    }
  };

  const handleMentionSelect = (insertText: string) => {
    const value = userPromptInput;
    const atIdx = value.lastIndexOf('@');
    if (atIdx !== -1) {
      const newValue = value.slice(0, atIdx) + insertText + ' ';
      setUserPromptInput(newValue);
      setTimeout(() => {
        textareaRef.current?.focus();
        autoResize();
      }, 0);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (mentionOpen) {
      if (e.key === 'Enter' || e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        e.preventDefault();
        return;
      }
      if (e.key === 'Escape') {
        setMentionOpen(false);
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey && !isComposing) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleSend = () => {
    if (!userPromptInput.trim()) return;
    sendPrompt();
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  return (
    <div className="p-3 bg-slate-900/90 border-t border-slate-800 flex-shrink-0 relative">
      <div className="max-w-4xl mx-auto bg-slate-950 rounded-xl border border-slate-800 p-2 focus-within:border-indigo-500/60 transition shadow-inner relative">
        {mentionOpen && <MentionPopover onSelect={handleMentionSelect} />}
        <div className="flex items-end space-x-2">
          <div className="pl-2 pb-1.5">
            <span className={clsx(
              'inline-block w-2 h-2 rounded-full',
              wsConnected ? 'bg-emerald-500' : wsReadyState === WebSocket.CONNECTING ? 'bg-amber-500 animate-pulse' : 'bg-rose-500'
            )} title={wsConnected ? '已连接' : '未连接'}></span>
          </div>
          <textarea
            ref={textareaRef}
            value={userPromptInput}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            onCompositionStart={() => setIsComposing(true)}
            onCompositionEnd={() => setIsComposing(false)}
            rows={1}
            placeholder={wsConnected ? '向犬种特工队下发 CVO 指令... (Shift+Enter 换行, @ 唤起上下文)' : '正在连接 WebSocket...'}
            className="flex-1 bg-transparent border-none text-xs text-slate-100 placeholder-slate-500 focus:outline-none font-mono resize-none max-h-36 overflow-y-auto"
          />
          <button
            onClick={handleSend}
            disabled={!userPromptInput.trim() || isGenerating || !wsConnected}
            className={clsx(
              'px-3.5 py-1.5 rounded-lg text-xs font-medium transition flex items-center gap-1 shadow-md',
              userPromptInput.trim() && !isGenerating && wsConnected
                ? 'bg-indigo-600 hover:bg-indigo-500 text-white shadow-indigo-600/30'
                : 'bg-slate-800 text-slate-500 cursor-not-allowed'
            )}
          >
            {isGenerating ? (
              <>
                <i className="fa-solid fa-spinner fa-spin text-[10px]"></i>
                <span>生成中</span>
              </>
            ) : (
              <>
                <span>发送</span>
                <i className="fa-solid fa-paper-plane text-[10px]"></i>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
