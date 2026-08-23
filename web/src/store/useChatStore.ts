import { create } from 'zustand';
import { useShallow } from 'zustand/react/shallow';
import { WsManager } from '../services/ws';
import { API_BASE, apiGet } from '../services/http';
import { useAppStore } from './useAppStore';
import type { StreamEvent, BreedResponseLiveEvent, BreedResponseCompleteEvent, SopGateEvent } from '../types';
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
  AgentMessagePayload,
  ErrorPayload,
  SystemNoticePayload,
  LivenessPayload,
  CarrierHealthPayload,
  SopGatePayload,
  CvoEscalationPayload,
} from '../types/api';

// Re-export useShallow for components that select objects/arrays from this store
export { useShallow };

// Per-carrier health (T25 / R6): keyed by carrier id (e.g. "claude"). The
// backend pushes CARRIER_HEALTH whenever a carrier degrades, recovers, or a
// transport tier is skipped during fallback. `updatedAt` lets the UI show
// staleness and lets us pick the freshest signal per carrier.
export interface CarrierHealthState {
  level: 'online' | 'degraded' | 'offline';
  transport?: string;
  reason?: string;
  remainingMs?: number;
  updatedAt: number;
}

interface ChatStore {
  wsManager: WsManager | null;
  wsReadyState: number;
  events: Record<string, StreamEvent[]>;
  isGenerating: Record<string, boolean>;
  lastSeq: Record<string, number>;
  // T25 / R6: structured per-carrier upstream health from CARRIER_HEALTH events.
  carrierHealth: Record<string, CarrierHealthState>;
  // G4: threads with an unresolved CVO escalation. Keyed by thread id; true
  // while an escalation card awaits the operator's decision.
  escalations: Record<string, boolean>;

  initWebSocket: () => void;
  sendPrompt: () => void;
  sendHitlResponse: (requestId: string, approved: boolean, reason: string) => void;
  loadThreadEvents: (threadId: string, events: StreamEvent[]) => void;
  prependHistory: (threadId: string, events: StreamEvent[]) => void;
  resolveEscalation: (threadId: string, decision: string, escalationId?: string) => void;
  restoreEscalations: () => Promise<void>;
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
  carrierHealth: {},
  escalations: {},

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

  prependHistory: (threadId, incoming) => {
    set((state) => {
      const existing = state.events[threadId] ?? [];
      // Dedup by content+timestamp so pagination boundaries don't double-render.
      const keyOf = (e: StreamEvent) =>
        `${e.type}:${(e as { timestamp?: string }).timestamp ?? ''}:${
          'content' in e ? (e as { content?: string }).content : ''
        }`;
      const seen = new Set(existing.map(keyOf));
      const merged = [...incoming, ...existing.filter((e) => !seen.has(keyOf(e)))];
      return { events: { ...state.events, [threadId]: merged } };
    });
  },

  resolveEscalation: (threadId, decision, escalationId) => {
    const manager = get().wsManager;
    if (manager && escalationId) {
      manager.sendEscalationResponse(escalationId, decision, threadId);
    }
    set((state) => {
      const threadEvents = state.events[threadId] ?? [];
      const remaining = threadEvents.filter(
        (e) => !(e.type === 'cvo_escalation' && (!escalationId || e.escalationId === escalationId || !e.escalationId))
      );
      const stillEscalated = remaining.some((e) => e.type === 'cvo_escalation');
      return {
        events: { ...state.events, [threadId]: remaining },
        escalations: { ...state.escalations, [threadId]: stillEscalated },
      };
    });
  },

  // restoreEscalations re-hydrates unresolved CVO escalation cards after a
  // page reload (the registry survives reloads server-side, not restarts).
  restoreEscalations: async () => {
    try {
      const data = await apiGet<
        Array<{ escalation_id: string; session_id: string; created_at: string; payload: CvoEscalationPayload | null }>
      >('/api/escalations');
      if (!Array.isArray(data) || data.length === 0) return;
      set((state) => {
        const events = { ...state.events };
        const escalations = { ...state.escalations };
        for (const esc of data) {
          const tid = esc.session_id;
          if (!tid) continue;
          const existing = events[tid] ?? [];
          if (existing.some((e) => e.type === 'cvo_escalation' && e.escalationId === esc.escalation_id)) continue;
          events[tid] = [
            ...existing,
            {
              type: 'cvo_escalation',
              threadId: tid,
              escalationId: esc.escalation_id,
              reason: esc.payload?.reason,
              maxDepth: esc.payload?.max_depth,
              options: (esc.payload?.options ?? []).map((o) => ({ id: o.id, prompt: o.prompt })),
            },
          ];
          escalations[tid] = true;
        }
        return { events, escalations };
      });
    } catch {
      // Older backend without the endpoint: escalate nothing, stay quiet.
    }
  },

  handleWsEvent: (event) => {
    // T25 / R6: CARRIER_HEALTH is a global event (no session_id) that reports
    // per-carrier upstream health. Handle it before the thread-scoped
    // resolution below, which would otherwise drop an event without a valid
    // threadId. Stored by carrier id so ConnectionStatusBar can render it.
    if (event.type === 'CARRIER_HEALTH') {
      const p = event.payload as CarrierHealthPayload;
      if (!p || !p.carrier) {
        console.warn('[WS] Ignoring CARRIER_HEALTH without carrier:', event);
        return;
      }
      set((state) => ({
        carrierHealth: {
          ...state.carrierHealth,
          [p.carrier]: {
            level: p.level,
            transport: p.transport,
            reason: p.reason,
            remainingMs: p.remaining_ms,
            updatedAt: Date.now(),
          },
        },
      }));
      return;
    }

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

      case 'AGENT_MESSAGE': {
        const p = payload as AgentMessagePayload;
        if (!p.content) break;
        set((state) => {
          const threadEvents = state.events[threadId] ?? [];
          const lastEvent = threadEvents[threadEvents.length - 1];
          if (
            lastEvent &&
            lastEvent.type === 'breed_response_live' &&
            lastEvent.breed === p.breed
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
              type: 'breed_response_live',
              breed: p.breed,
              content: p.content,
            } as BreedResponseLiveEvent),
          };
        });
        break;
      }

      case 'AGENT_LIVENESS': {
        const p = payload as LivenessPayload;
        set((state) => ({
          events: appendEvent(state.events, threadId, {
            type: 'breed_stall_warning',
            breed: p.breed,
            state: p.state,
            hard: p.hard,
            message: p.message,
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
          // Convert an in-flight live text block to the terminal complete
          // event (G1+G9); otherwise append a fresh complete event carrying any
          // final content supplied by the server.
          let events2: StreamEvent[];
          let liveIdx = -1;
          for (let i = 0; i < updated.length; i++) {
            const e = updated[i];
            if (e.type === 'breed_response_live' && e.breed === p.breed) {
              liveIdx = i;
              break;
            }
          }
          if (liveIdx >= 0) {
            const live = updated[liveIdx] as BreedResponseLiveEvent;
            const finalContent =
              p.content && p.content.length > 0 ? p.content : live.content;
            events2 = [...updated];
            events2[liveIdx] = {
              type: 'breed_response_complete',
              breed: p.breed,
              steps: p.steps ?? [],
              content: finalContent,
            } as BreedResponseCompleteEvent;
          } else {
            events2 = [
              ...updated,
              {
                type: 'breed_response_complete',
                breed: p.breed,
                steps: p.steps ?? [],
                content: p.content,
              } as BreedResponseCompleteEvent,
            ];
          }
          return {
            isGenerating: { ...state.isGenerating, [threadId]: false },
            events: { ...state.events, [threadId]: events2 },
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
                {
                  type: 'error',
                  breed: p.breed,
                  error: p.error,
                  reason: p.reason,
                  summary: p.summary,
                  hint: p.hint,
                  excerpt: p.excerpt,
                  source: p.source,
                  meta: p.meta,
                },
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
        if (!p) break;
        // Surface live in the toast stack (critical/error notices are loud)
        // and persist into the notification center so the bell has data.
        const toastType =
          p.severity === 'critical' ? 'error'
          : p.severity === 'warning' ? 'warning'
          : p.severity === 'recovery' ? 'success'
          : 'info';
        useAppStore.getState().showToast({ message: `${p.title}：${p.message}`, type: toastType });
        useAppStore.getState().pushLiveNotice({
          severity: toastType === 'success' ? 'info' : toastType,
          title: p.title,
          message: p.message,
          source: 'system',
          timestamp: p.timestamp,
        });
        break;
      }

      case 'CVO_ESCALATION': {
        const p = payload as CvoEscalationPayload;
        set((state) => ({
          escalations: { ...state.escalations, [threadId]: true },
          events: appendEvent(state.events, threadId, {
            type: 'cvo_escalation',
            threadId,
            escalationId: p?.escalation_id,
            reason: p?.reason,
            maxDepth: p?.max_depth,
            options: (p?.options ?? []).map((o) => ({ id: o.id, prompt: o.prompt })),
          }),
        }));
        break;
      }

      case 'SOP_GATE': {
        const p = payload as SopGatePayload;
        set((state) => {
          const threadEvents = state.events[threadId] ?? [];
          const evt: SopGateEvent = { type: 'sop_gate', reason: p.reason };
          return {
            events: {
              ...state.events,
              [threadId]: [...threadEvents, evt],
            },
          };
        });
        break;
      }

      default:
        console.warn('[WS] Unhandled event type:', event.type);
    }
  },
}));
