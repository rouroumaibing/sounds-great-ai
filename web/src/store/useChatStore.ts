import { create } from 'zustand';
import { useShallow } from 'zustand/react/shallow';
import { WsManager } from '../services/ws';
import { API_BASE } from '../services/http';
import { useAppStore } from './useAppStore';
import { useNoticeStore } from './useNoticeStore';
import type { StreamEvent } from '../types';
import { useI18n } from './useI18n';
import type {
  WsEvent,
  BarkStartPayload,
  ThinkingPayload,
  ToolCallPayload,
  CodeDiffPayload,
  TerminalOutputPayload,
  HitlApprovalPayload,
  BarkResultPayload,
  BarkErrorPayload,
  ErrorPayload,
  SystemNoticePayload,
} from '../types/api';

// Re-export useShallow for components that select objects/arrays from this store
export { useShallow };

interface ChatStore {
  wsManager: WsManager | null;
  wsReadyState: number;
  events: Record<string, StreamEvent[]>;
  isGenerating: Record<string, boolean>;
  lastSeq: Record<string, number>;

  initWebSocket: () => void;
  sendPrompt: () => void;
  sendHitlResponse: (requestId: string, approved: boolean, reason: string) => void;
  loadThreadEvents: (threadId: string, events: StreamEvent[]) => void;
  resolveEscalation: () => void;
  handleWsEvent: (event: WsEvent) => void;
}

function resolveThreadId(event: WsEvent): string | null {
  const tid = event.session_id || useAppStore.getState().activeThreadId;
  if (!tid) {
    console.warn('[WS] Dropping event without valid threadId:', event);
    return null;
  }
  return tid;
}

function appendEvent(
  events: Record<string, StreamEvent[]>,
  threadId: string,
  newEvent: StreamEvent
): Record<string, StreamEvent[]> {
  const threadEvents = events[threadId];
  // Functional update: only create new array for the changed thread,
  // reuse existing object structure to minimize allocations.
  const updatedEvents = Object.assign({}, events);
  updatedEvents[threadId] = threadEvents ? [...threadEvents, newEvent] : [newEvent];
  return updatedEvents;
}

export const useChatStore = create<ChatStore>((set, get) => ({
  wsManager: null,
  wsReadyState: WebSocket.CLOSED,
  events: {},
  isGenerating: {},
  lastSeq: {},

  initWebSocket: () => {
    const existing = get().wsManager;
    if (existing) return;

    const manager = new WsManager(API_BASE);
    manager.onEvent((event) => {
      get().handleWsEvent(event);
    });

    // Poll readyState for connection status updates
    const interval = setInterval(() => {
      const newState = manager.readyState;
      const current = get().wsReadyState;
      if (newState !== current) {
        set({ wsReadyState: newState });
      }
    }, 500);

    manager.connect();
    set({ wsManager: manager, wsReadyState: manager.readyState });

    // Store interval on the manager for cleanup (hack: attach to window for now)
    (window as unknown as Record<string, unknown>).__wsReadyStateInterval = interval;
  },

  sendPrompt: () => {
    const { userPromptInput, activeThreadId } = useAppStore.getState();
    if (!userPromptInput.trim()) return;
    if (!activeThreadId) return;

    const manager = get().wsManager;
    if (!manager || manager.readyState !== WebSocket.OPEN) {
      useAppStore.getState().showToast({
        message: useI18n.getState().t('store.usechatstore.s1'),
        type: 'error',
      });
      return;
    }

    // Optimistic: append cvo_message to thread events
    const cvoEvent: StreamEvent = {
      type: 'cvo_message',
      timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
      content: userPromptInput,
    };

    set((state) => ({
      events: appendEvent(state.events, activeThreadId, cvoEvent),
      isGenerating: { ...state.isGenerating, [activeThreadId]: true },
    }));

    manager.send(userPromptInput, activeThreadId);
    useAppStore.getState().setUserPromptInput('');
  },

  sendHitlResponse: (requestId, approved, reason) => {
    const { activeThreadId } = useAppStore.getState();
    const manager = get().wsManager;
    if (!manager || !activeThreadId) return;
    manager.sendHitlResponse(requestId, approved, reason, activeThreadId);
  },

  loadThreadEvents: (threadId, events) => {
    set((state) => {
      if (state.events[threadId]) return state; // already loaded
      return { events: { ...state.events, [threadId]: events } };
    });
  },

  resolveEscalation: () => {
    const { activeThreadId } = useAppStore.getState();
    if (!activeThreadId) return;
    set((state) => {
      const threadEvents = state.events[activeThreadId] ?? [];
      return {
        events: {
          ...state.events,
          [activeThreadId]: threadEvents.filter((e) => e.type !== 'cvo_escalation'),
        },
      };
    });
  },

  handleWsEvent: (event) => {
    const threadId = resolveThreadId(event);
    if (!threadId) return;

    // Gap detection: check if seq numbers are contiguous
    if (event.seq != null && event.seq > 0) {
      const prevSeq = get().lastSeq[threadId] ?? 0;
      if (prevSeq > 0 && event.seq > prevSeq + 1) {
        const gap = event.seq - prevSeq - 1;
        console.warn(`[WS] Gap detected: session=${threadId} prevSeq=${prevSeq} curSeq=${event.seq} missing=${gap}`);
      }
      set((state) => ({
        lastSeq: { ...state.lastSeq, [threadId]: event.seq! },
      }));
    }

    const payload = event.payload;
    const now = new Date().toLocaleTimeString('zh-CN', { hour12: false });

    switch (event.type) {
      case 'BARK_START': {
        const p = payload as BarkStartPayload;
        set((state) => ({
          isGenerating: { ...state.isGenerating, [threadId]: true },
          events: appendEvent(state.events, threadId, {
            type: 'breed_response_start',
            breed: p.breed,
            timestamp: now,
          }),
        }));
        break;
      }

      case 'THINKING': {
        const p = payload as ThinkingPayload;
        set((state) => {
          const threadEvents = state.events[threadId] ?? [];
          const lastEvent = threadEvents[threadEvents.length - 1];
          if (
            lastEvent &&
            lastEvent.type === 'thinking' &&
            lastEvent.status === 'running' &&
            lastEvent.step === p.step
          ) {
            const updated = [...threadEvents];
            updated[updated.length - 1] = {
              ...lastEvent,
              content: lastEvent.content + p.content,
            };
            return { events: { ...state.events, [threadId]: updated } };
          }
          return {
            events: appendEvent(state.events, threadId, {
              type: 'thinking',
              content: p.content,
              step: p.step,
              status: 'running',
            }),
          };
        });
        break;
      }

      case 'TOOL_CALL': {
        const p = payload as ToolCallPayload;
        set((state) => ({
          events: appendEvent(state.events, threadId, {
            type: 'tool_call',
            tool: p.tool,
            params: p.params,
            result: p.result,
            status: p.status,
          }),
        }));
        break;
      }

      case 'CODE_DIFF': {
        const p = payload as CodeDiffPayload;
        set((state) => ({
          events: appendEvent(state.events, threadId, {
            type: 'code_diff',
            file: p.file,
            diff: p.diff,
            action: p.action,
          }),
        }));
        break;
      }

      case 'TERMINAL_OUTPUT': {
        const p = payload as TerminalOutputPayload;
        set((state) => {
          const threadEvents = state.events[threadId] ?? [];
          const lastEvent = threadEvents[threadEvents.length - 1];
          if (
            lastEvent &&
            lastEvent.type === 'terminal_output' &&
            lastEvent.stream === p.stream
          ) {
            const updated = [...threadEvents];
            updated[updated.length - 1] = {
              ...lastEvent,
              data: lastEvent.data + p.data,
            };
            return { events: { ...state.events, [threadId]: updated } };
          }
          return {
            events: appendEvent(state.events, threadId, {
              type: 'terminal_output',
              stream: p.stream,
              data: p.data,
            }),
          };
        });
        break;
      }

      case 'HITL_APPROVAL': {
        const p = payload as HitlApprovalPayload;
        set((state) => ({
          events: appendEvent(state.events, threadId, {
            type: 'approval_request',
            action: p.action,
            request_id: p.request_id,
            impact: p.impact,
          }),
        }));
        break;
      }

      case 'BARK_RESULT': {
        const p = payload as BarkResultPayload;
        set((state) => {
          const threadEvents = state.events[threadId] ?? [];
          const updated = threadEvents.map((e) =>
            e.type === 'thinking' && e.status === 'running'
              ? { ...e, status: 'success' as const }
              : e
          );
          return {
            isGenerating: { ...state.isGenerating, [threadId]: false },
            events: {
              ...state.events,
              [threadId]: [
                ...updated,
                { type: 'breed_response_complete', breed: p.breed, steps: p.steps },
              ],
            },
          };
        });
        break;
      }

      case 'BARK_ERROR': {
        const p = payload as BarkErrorPayload;
        set((state) => {
          const threadEvents = state.events[threadId] ?? [];
          const updated = threadEvents.map((e) =>
            e.type === 'thinking' && e.status === 'running'
              ? { ...e, status: 'error' as const }
              : e
          );
          return {
            isGenerating: { ...state.isGenerating, [threadId]: false },
            events: {
              ...state.events,
              [threadId]: [
                ...updated,
                { type: 'error', breed: p.breed, error: p.error },
              ],
            },
          };
        });
        break;
      }

      case 'BARK_REJECTED': {
        set((state) => ({
          isGenerating: { ...state.isGenerating, [threadId]: false },
        }));
        useAppStore.getState().showToast({
          message: useI18n.getState().t('store.usechatstore.s2'),
          type: 'warning',
        });
        break;
      }

      case 'ERROR': {
        const p = payload as ErrorPayload;
        useAppStore.getState().showToast({
          message: useI18n.getState().t('store.usechatstore.s3').replace('{p.error}', String(p.error)),
          type: 'error',
        });
        break;
      }

      case 'SYSTEM_NOTICE': {
        const p = payload as SystemNoticePayload;
        useNoticeStore.getState().addNotice(p);
        break;
      }

      default:
        console.warn('[WS] Unhandled event type:', event.type);
    }
  },
}));
