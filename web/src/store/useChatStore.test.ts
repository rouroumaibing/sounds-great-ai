import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useChatStore } from './useChatStore';
import { useAppStore } from './useAppStore';
import type { WsManager } from '../services/ws';
import type { WsEvent } from '../types/api';

// The WS event machine is the heart of the app: every backend stream event
// flows through handleWsEvent into per-thread StreamEvents. These tests pin
// the state transitions the UI depends on.

function makeManager(overrides: Partial<WsManager> = {}): WsManager {
  return {
    readyState: 1, // WebSocket.OPEN
    connect: vi.fn(),
    disconnect: vi.fn(),
    send: vi.fn(),
    sendHitlResponse: vi.fn(),
    sendWakeHold: vi.fn(),
    sendEscalationResponse: vi.fn(),
    onEvent: vi.fn(),
    ...overrides,
  } as unknown as WsManager;
}

function ev(type: string, payload: unknown, sessionId = 't1', seq?: number): WsEvent {
  return { type, session_id: sessionId, timestamp: 0, seq, payload };
}

function resetStores(manager: WsManager | null = makeManager()) {
  useAppStore.setState({
    activeThreadId: 't1',
    userPromptInput: '',
    notifications: [],
    toasts: [],
    contextMenu: { show: false, x: 0, y: 0, file: null },
  });
  useChatStore.setState({
    wsManager: manager,
    wsReadyState: 1,
    events: {},
    isGenerating: {},
    lastSeq: {},
    carrierHealth: {},
    escalations: {},
  });
}

let handleWsEvent: (e: WsEvent) => void;

beforeEach(() => {
  resetStores();
  handleWsEvent = useChatStore.getState().handleWsEvent;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useChatStore WS event machine', () => {
  it('BARK_START appends breed_response_start and marks generating', () => {
    handleWsEvent(ev('BARK_START', { breed: 'bianmu', session_id: 't1', query: 'q' }));
    const s = useChatStore.getState();
    expect(s.isGenerating.t1).toBe(true);
    expect(s.events.t1).toHaveLength(1);
    expect(s.events.t1[0].type).toBe('breed_response_start');
  });

  it('consecutive THINKING deltas with the same step merge into one block', () => {
    handleWsEvent(ev('THINKING', { step: 1, content: 'abc' }));
    handleWsEvent(ev('THINKING', { step: 1, content: 'def' }));
    const s = useChatStore.getState();
    expect(s.events.t1).toHaveLength(1);
    expect(s.events.t1[0]).toMatchObject({ type: 'thinking', content: 'abcdef', status: 'running' });
  });

  it('AGENT_MESSAGE deltas accumulate and BARK_RESULT converts live → complete', () => {
    handleWsEvent(ev('BARK_START', { breed: 'bianmu' }));
    handleWsEvent(ev('AGENT_MESSAGE', { breed: 'bianmu', content: 'hello ' }));
    handleWsEvent(ev('AGENT_MESSAGE', { breed: 'bianmu', content: 'world' }));

    // Before the terminal event the live block carries the accumulated text.
    let live = useChatStore.getState().events.t1.find((e) => e.type === 'breed_response_live');
    expect(live).toMatchObject({ breed: 'bianmu', content: 'hello world' });

    handleWsEvent(ev('BARK_RESULT', { breed: 'bianmu', success: true, steps: [] }));

    const s = useChatStore.getState();
    expect(s.isGenerating.t1).toBe(false);
    // BARK_RESULT replaces the live block in place with the complete event.
    expect(s.events.t1.some((e) => e.type === 'breed_response_live')).toBe(false);
    const complete = s.events.t1.find((e) => e.type === 'breed_response_complete');
    expect(complete).toBeDefined();
    expect(complete).toMatchObject({ breed: 'bianmu', content: 'hello world' });
  });

  it('BARK_RESULT without a live block appends a fresh complete event', () => {
    handleWsEvent(ev('BARK_RESULT', { breed: 'xigou', success: true, steps: [], content: 'final' }));
    const complete = useChatStore.getState().events.t1.find((e) => e.type === 'breed_response_complete');
    expect(complete).toMatchObject({ breed: 'xigou', content: 'final' });
  });

  it('BARK_ERROR appends an error event and stops generating', () => {
    handleWsEvent(ev('BARK_START', { breed: 'jinmao' }));
    handleWsEvent(ev('BARK_ERROR', { breed: 'jinmao', error: 'boom' }));
    const s = useChatStore.getState();
    expect(s.isGenerating.t1).toBe(false);
    expect(s.events.t1.some((e) => e.type === 'error')).toBe(true);
  });

  it('CARRIER_HEALTH updates the per-carrier map without a session_id', () => {
    handleWsEvent({ type: 'CARRIER_HEALTH', session_id: '', timestamp: 0, payload: { carrier: 'claude', level: 'degraded' } });
    const health = useChatStore.getState().carrierHealth;
    expect(health.claude).toMatchObject({ level: 'degraded' });
  });

  it('SYSTEM_NOTICE surfaces as a toast and lands in the notification center', () => {
    handleWsEvent(ev('SYSTEM_NOTICE', { severity: 'warning', title: '交接终止', message: '深度超限', timestamp: '2026-01-01T00:00:00Z' }));
    const app = useAppStore.getState();
    expect(app.toasts).toHaveLength(1);
    expect(app.toasts[0].type).toBe('warning');
    expect(app.notifications).toHaveLength(1);
    expect(app.notifications[0]).toMatchObject({ title: '交接终止', message: '深度超限', read: false });
  });

  it('SOP_GATE appends a sop_gate event to the thread', () => {
    handleWsEvent(ev('SOP_GATE', { reason: 'cross review', blocked: true }));
    const e = useChatStore.getState().events.t1.find((x) => x.type === 'sop_gate');
    expect(e).toMatchObject({ type: 'sop_gate', reason: 'cross review' });
  });

  it('tracks the high-water seq per thread', () => {
    handleWsEvent(ev('BARK_START', {}, 't1', 1));
    handleWsEvent(ev('BARK_START', {}, 't1', 5));
    expect(useChatStore.getState().lastSeq.t1).toBe(5);
  });

  it('events without session_id route to the active thread', () => {
    useAppStore.setState({ activeThreadId: 'active-thread' });
    handleWsEvent(ev('BARK_START', { breed: 'b' }, ''));
    expect(useChatStore.getState().events['active-thread']).toHaveLength(1);
  });
});

describe('useChatStore CVO escalation (G4)', () => {
  it('CVO_ESCALATION appends the card event and flags the thread', () => {
    handleWsEvent(ev('CVO_ESCALATION', {
      escalation_id: 'esc-1',
      reason: '深度超限',
      max_depth: 3,
      options: [{ id: 'option_1', prompt: '接手' }, { id: 'option_2', prompt: '收尾' }],
    }));
    const s = useChatStore.getState();
    expect(s.escalations.t1).toBe(true);
    const card = s.events.t1.find((e) => e.type === 'cvo_escalation');
    expect(card).toMatchObject({
      escalationId: 'esc-1',
      threadId: 't1',
      reason: '深度超限',
      maxDepth: 3,
    });
  });

  it('resolveEscalation sends the decision over WS and clears card + flag', () => {
    const manager = useChatStore.getState().wsManager!;
    handleWsEvent(ev('CVO_ESCALATION', {
      escalation_id: 'esc-1',
      reason: 'r',
      options: [{ id: 'option_1', prompt: 'p' }],
    }));
    useChatStore.getState().resolveEscalation('t1', 'option_1', 'esc-1');

    expect(manager.sendEscalationResponse).toHaveBeenCalledWith('esc-1', 'option_1', 't1');
    const s = useChatStore.getState();
    expect(s.events.t1.some((e) => e.type === 'cvo_escalation')).toBe(false);
    expect(s.escalations.t1).toBe(false);
  });

  it('a second live escalation keeps the thread flagged when one is resolved', () => {
    handleWsEvent(ev('CVO_ESCALATION', { escalation_id: 'e1', reason: 'r', options: [] }));
    handleWsEvent(ev('CVO_ESCALATION', { escalation_id: 'e2', reason: 'r', options: [] }));
    useChatStore.getState().resolveEscalation('t1', 'option_1', 'e1');
    const s = useChatStore.getState();
    expect(s.escalations.t1).toBe(true);
    expect(s.events.t1.filter((e) => e.type === 'cvo_escalation')).toHaveLength(1);
  });
});

describe('useChatStore restoreEscalations (reload re-hydration)', () => {
  it('re-hydrates pending escalation cards and flags from GET /api/escalations', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify([
        {
          escalation_id: 'esc-r1',
          session_id: 't1',
          created_at: '2026-08-22T00:00:00Z',
          payload: { escalation_id: 'esc-r1', reason: '深度超限', max_depth: 3, options: [{ id: 'option_1', prompt: 'p' }] },
        },
      ]), { status: 200 })
    ));
    await useChatStore.getState().restoreEscalations();
    const s = useChatStore.getState();
    expect(s.escalations.t1).toBe(true);
    const card = s.events.t1.find((e) => e.type === 'cvo_escalation');
    expect(card).toMatchObject({ escalationId: 'esc-r1', reason: '深度超限', maxDepth: 3 });

    // Restoring again must not duplicate the card.
    await useChatStore.getState().restoreEscalations();
    expect(useChatStore.getState().events.t1.filter((e) => e.type === 'cvo_escalation')).toHaveLength(1);
  });

  it('is a quiet no-op when the backend lacks the endpoint or returns empty', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"error":"not found"}', { status: 404 })));
    await expect(useChatStore.getState().restoreEscalations()).resolves.toBeUndefined();
    expect(useChatStore.getState().escalations).toEqual({});
  });
});

describe('useChatStore sendPrompt', () => {
  it('optimistically appends the user message, sends over WS, clears input', () => {
    useAppStore.setState({ userPromptInput: '  @bianmu fix it  ' });
    const manager = useChatStore.getState().wsManager!;
    useChatStore.getState().sendPrompt();

    expect(manager.send).toHaveBeenCalledWith('  @bianmu fix it  ', 't1');
    const s = useChatStore.getState();
    expect(s.events.t1.some((e) => e.type === 'cvo_message')).toBe(true);
    expect(useAppStore.getState().userPromptInput).toBe('');
  });

  it('refuses to send empty input', () => {
    useAppStore.setState({ userPromptInput: '   ' });
    const manager = useChatStore.getState().wsManager!;
    useChatStore.getState().sendPrompt();
    expect(manager.send).not.toHaveBeenCalled();
  });

  it('shows an error toast when the socket is not open', () => {
    resetStores(makeManager({ readyState: 3 } as Partial<WsManager>));
    useAppStore.setState({ userPromptInput: 'hi' });
    useChatStore.getState().sendPrompt();
    const app = useAppStore.getState();
    expect(app.toasts).toHaveLength(1);
    expect(app.toasts[0].type).toBe('error');
    // No optimistic event when the send path is unavailable.
    expect(useChatStore.getState().events.t1 ?? []).toHaveLength(0);
  });
});
