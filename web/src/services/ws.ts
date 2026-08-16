import type { WsEvent } from '../types/api';

type EventHandler = (event: WsEvent) => void;

export class WsManager {
  private ws: WebSocket | null = null;
  private listeners = new Set<EventHandler>();
  private reconnectAttempts = 0;
  private base: string;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(base: string) {
    this.base = base.replace(/^http/, 'ws');
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) {
      return;
    }
    try {
      this.ws = new WebSocket(`${this.base}/ws`);
    } catch (e) {
      console.error('[WS] Failed to create WebSocket:', e);
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
    };

    this.ws.onmessage = (e: MessageEvent) => {
      try {
        const parsed = JSON.parse(e.data);
        // Handle both single events and batched arrays (micro-batching from server)
        if (Array.isArray(parsed)) {
          for (const item of parsed) {
            if (item && typeof item.type === 'string') {
              this.listeners.forEach((fn) => fn(item as WsEvent));
            }
          }
          return;
        }
        const event = parsed as WsEvent;
        if (!event || typeof event.type !== 'string') {
          console.warn('[WS] Dropping malformed event:', e.data);
          return;
        }
        this.listeners.forEach((fn) => fn(event));
      } catch (err) {
        console.warn('[WS] Failed to parse message:', err);
      }
    };

    this.ws.onerror = (e: Event) => {
      console.warn('[WS] WebSocket error:', e);
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.scheduleReconnect();
    };
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= 5) {
      console.warn('[WS] Max reconnect attempts reached (5). Giving up.');
      return;
    }
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectAttempts++;
    const delay = 1000 * this.reconnectAttempts;
    this.reconnectTimer = setTimeout(() => this.connect(), delay);
  }

  send(message: string, sessionId: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('[WS] Cannot send — WebSocket not open');
      return;
    }
    this.ws.send(JSON.stringify({
      type: 'USER_INPUT',
      session_id: sessionId,
      timestamp: 0,
      payload: { message, session_id: sessionId, thread_id: sessionId },
    }));
  }

  sendHitlResponse(requestId: string, approved: boolean, reason: string, sessionId: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('[WS] Cannot send HITL_RESPONSE — WebSocket not open');
      return;
    }
    this.ws.send(JSON.stringify({
      type: 'HITL_RESPONSE',
      session_id: sessionId,
      timestamp: 0,
      payload: { request_id: requestId, approved, reason },
    }));
  }

  // sendWakeHold releases a parked hold_ball manually (human click). The server
  // re-dispatches the holder via ResumeHeldThread (transport/ws_handler.go).
  sendWakeHold(sessionId: string, kind: string, token?: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('[WS] Cannot send WAKE_HOLD — WebSocket not open');
      return;
    }
    this.ws.send(JSON.stringify({
      type: 'WAKE_HOLD',
      session_id: sessionId,
      timestamp: 0,
      payload: { session_id: sessionId, kind, token: token ?? '' },
    }));
  }

  onEvent(fn: EventHandler): () => void {
    this.listeners.add(fn);
    return () => { this.listeners.delete(fn); };
  }

  get readyState(): number {
    return this.ws?.readyState ?? WebSocket.CLOSED;
  }

  disconnect(): void {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectAttempts = 5; // prevent reconnect
    this.ws?.close();
    this.ws = null;
  }
}
