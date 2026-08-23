import { afterEach, describe, expect, it, vi } from 'vitest';
import { streamSSE } from './sse';
import { API_BASE } from './http';

// SSE chunks arrive as raw bytes; emulate a ReadableStream pushing frames
// incrementally (including a split frame across chunks).
function sseBody(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  let i = 0;
  return new ReadableStream({
    pull(controller) {
      if (i >= chunks.length) {
        controller.close();
        return;
      }
      controller.enqueue(encoder.encode(chunks[i++]));
    },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('streamSSE (fetch-based SSE with auth)', () => {
  it('sends Authorization + Accept headers to API_BASE and delivers data frames', async () => {
    localStorage.setItem('sounds-great-ai:auth-token', 'tok-1');
    const fetchMock = vi.fn().mockImplementation(async () =>
      new Response(sseBody([': connected\n\n', 'data: {"ev":1}\n\n', 'data: line1\ndata: line2\n\n']), { status: 200 })
    );
    vi.stubGlobal('fetch', fetchMock);

    const got: string[] = [];
    const controller = new AbortController();
    // Stream closes after chunks are exhausted; retry loop would re-connect —
    // abort right after the last expected message to end the test.
    const done = streamSSE('/api/people-memory/events?operator=leader', {
      onMessage: (d) => {
        got.push(d);
        if (got.length === 2) controller.abort();
      },
    }, controller.signal);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`${API_BASE}/api/people-memory/events?operator=leader`);
    expect(init.headers).toMatchObject({ Accept: 'text/event-stream', Authorization: 'Bearer tok-1' });

    await done;
    expect(got).toEqual(['{"ev":1}', 'line1\nline2']);
  });

  it('reassembles frames split across chunk boundaries and skips comment/heartbeat frames', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () =>
      new Response(sseBody([': ping\n\nda', 'ta: split\n', '\ndata: after\n\n']), { status: 200 })
    ));
    const got: string[] = [];
    const controller = new AbortController();
    await streamSSE('/x', {
      onMessage: (d) => {
        got.push(d);
        controller.abort();
      },
    }, controller.signal);
    expect(got).toEqual(['split', 'after']);
  });

  it('reports error and reconnects on non-2xx', async () => {
    let calls = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => {
      calls++;
      return new Response('nope', { status: 401 });
    }));
    const errors: number[] = [];
    const controller = new AbortController();
    const p = streamSSE('/x', {
      onMessage: () => {},
      onError: () => {
        errors.push(calls);
        if (calls >= 2) controller.abort();
      },
      retryDelayMs: 10,
    }, controller.signal);
    await p;
    expect(errors.length).toBeGreaterThanOrEqual(2);
    expect(calls).toBeGreaterThanOrEqual(2);
  });
});
