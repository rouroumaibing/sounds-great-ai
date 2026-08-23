import { afterEach, describe, expect, it, vi } from 'vitest';
import { WsManager } from './ws';

// A minimal open-socket double: WsManager only touches readyState + send.
function openSocket() {
  return {
    readyState: 1,
    send: vi.fn(),
    close: vi.fn(),
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('WsManager outbound protocol', () => {
  it('send wraps the message in a USER_INPUT envelope', () => {
    const sock = openSocket();
    const mgr = new WsManager('http://localhost:8080');
    (mgr as unknown as { ws: unknown }).ws = sock;
    mgr.send('hello', 't1');
    const frame = JSON.parse(sock.send.mock.calls[0][0] as string);
    expect(frame).toMatchObject({
      type: 'USER_INPUT',
      session_id: 't1',
      payload: { message: 'hello', session_id: 't1', thread_id: 't1' },
    });
  });

  it('sendEscalationResponse frames CVO_ESCALATION_RESPONSE with the decision', () => {
    const sock = openSocket();
    const mgr = new WsManager('http://localhost:8080');
    (mgr as unknown as { ws: unknown }).ws = sock;
    mgr.sendEscalationResponse('esc-9', 'option_1', 't1');
    const frame = JSON.parse(sock.send.mock.calls[0][0] as string);
    expect(frame).toEqual({
      type: 'CVO_ESCALATION_RESPONSE',
      session_id: 't1',
      timestamp: 0,
      payload: { session_id: 't1', escalation_id: 'esc-9', decision: 'option_1' },
    });
  });

  it('send* are no-ops (no throw) when the socket is closed', () => {
    const mgr = new WsManager('http://localhost:8080');
    expect(() => mgr.send('x', 't1')).not.toThrow();
    expect(() => mgr.sendEscalationResponse('e', 'd', 't1')).not.toThrow();
  });
});
