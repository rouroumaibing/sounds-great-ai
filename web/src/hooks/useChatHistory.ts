import { useCallback, useEffect, useRef, useState } from 'react';
import { apiGet } from '../services/http';
import { useChatStore } from '../store/useChatStore';
import type { StreamEvent } from '../types';

// ---------------------------------------------------------------------------
// 聊天历史水合（G9）。
//
// 与 clowder web/src/hooks/useChatHistory.ts 同位置（模块级常量）对齐分页与
// 合并窗口；端点语义对齐 SG 的 GET /api/threads/{id}/messages?limit=&before=。
// 刷新 / 切换线程时自动把落库的聊天消息水合进 useChatStore，scroll-up 按游标
// 增量加载更早消息，与 WS 实时事件按内容+时间戳去重合并。
// ---------------------------------------------------------------------------

// 历史分页大小（对齐后端 ListMessages 默认 limit 50 / max 200）。
export const HISTORY_PAGE_SIZE = 50;
// 导出时的最大消息数上限。
export const EXPORT_LIMIT = 10000;
// 草稿 / 实时事件合并窗口（对齐 clowder）。
export const DRAFT_LIVE_MERGE_ACTIVITY_WINDOW_MS = 5 * 60 * 1000;
// 滚动贴底阈值。
export const SCROLL_BOTTOM_THRESHOLD_PX = 24;
// 滚动恢复最大帧数。
export const MAX_RESTORE_FRAMES = 90;

// 模块级缓存（与 clowder 同位置）。
const scrollPositionsByThread = new Map<string, number>();
const taskCacheByThread = new Map<string, unknown>();

export { scrollPositionsByThread, taskCacheByThread };

interface ChatMessage {
  id: string;
  thread_id: string;
  role: string; // user | assistant
  content: string;
  sender: string;
  timestamp: string; // ISO8601
}

interface MessagesResponse {
  messages: ChatMessage[];
  has_more: boolean;
}

function formatTs(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleTimeString('zh-CN', { hour12: false });
}

// toStreamEvent maps a persisted chat message into a renderable StreamEvent.
// assistant 消息带 content（历史文本），user 消息为 cvo_message。
function toStreamEvent(m: ChatMessage): StreamEvent {
  const ts = formatTs(m.timestamp);
  if (m.role === 'user') {
    return { type: 'cvo_message', timestamp: ts, content: m.content };
  }
  return { type: 'breed_response_complete', breed: m.sender, steps: [], content: m.content } as StreamEvent;
}

function cursorOf(m: ChatMessage): string {
  const ts = new Date(m.timestamp).getTime();
  return `${ts}:${m.id}`;
}

export interface UseChatHistory {
  hasMore: boolean;
  loading: boolean;
  loadOlder: () => void;
}

// useChatHistory hydrates chat history for the active thread and exposes an
// incremental loader for scroll-up.
export function useChatHistory(threadId: string | undefined): UseChatHistory {
  const loadThreadEvents = useChatStore((s) => s.loadThreadEvents);
  const prependHistory = useChatStore((s) => s.prependHistory);

  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const oldestRef = useRef<ChatMessage | null>(null);
  const loadingRef = useRef(false);

  const fetchInitial = useCallback(
    async (tid: string) => {
      if (loadingRef.current) return;
      loadingRef.current = true;
      setLoading(true);
      try {
        const data = await apiGet<MessagesResponse>(
          `/api/threads/${tid}/messages?limit=${HISTORY_PAGE_SIZE}`,
        );
        const evts = (data.messages ?? []).map(toStreamEvent);
        loadThreadEvents(tid, evts);
        oldestRef.current = data.messages && data.messages.length > 0 ? data.messages[0] : null;
        setHasMore(Boolean(data.has_more));
      } catch (e) {
        // Hydration is best-effort: a missing/empty store must not break the UI.
        console.warn('[history] initial load failed:', e);
      } finally {
        loadingRef.current = false;
        setLoading(false);
      }
    },
    [loadThreadEvents],
  );

  const loadOlder = useCallback(() => {
    if (!threadId || !hasMore || loadingRef.current || !oldestRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    const oldest = oldestRef.current;
    apiGet<MessagesResponse>(
      `/api/threads/${threadId}/messages?limit=${HISTORY_PAGE_SIZE}&before=${encodeURIComponent(cursorOf(oldest))}`,
    )
      .then((data) => {
        const evts = (data.messages ?? []).map(toStreamEvent);
        prependHistory(threadId, evts);
        if (data.messages && data.messages.length > 0) {
          oldestRef.current = data.messages[0];
        }
        setHasMore(Boolean(data.has_more));
      })
      .catch((e) => console.warn('[history] loadOlder failed:', e))
      .finally(() => {
        loadingRef.current = false;
        setLoading(false);
      });
  }, [threadId, hasMore, prependHistory]);

  useEffect(() => {
    if (!threadId) return;
    oldestRef.current = null;
    setHasMore(false);
    fetchInitial(threadId);
  }, [threadId, fetchInitial]);

  return { hasMore, loading, loadOlder };
}
