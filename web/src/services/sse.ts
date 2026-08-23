import { API_BASE, authHeaders } from './http';

// fetchSSE consumes a server-sent-events stream over fetch instead of
// EventSource. Unlike EventSource it can carry the Authorization header (and
// the API_BASE origin), so SSE works behind AUTH_TOKEN and remote API
// deployments. Comment frames (`: ping`) are ignored; each `data:` frame is
// delivered as one message. Auto-reconnects with a small delay until aborted.
export interface SSEHandlers {
  onOpen?: () => void;
  onMessage: (data: string) => void;
  onError?: () => void;
  /** Reconnect delay; defaults to 3000ms. */
  retryDelayMs?: number;
}

export async function streamSSE(path: string, handlers: SSEHandlers, signal: AbortSignal): Promise<void> {
  const delay = handlers.retryDelayMs ?? 3000;
  // Loop until the caller aborts; each iteration is one connection lifetime.
  while (!signal.aborted) {
    try {
      const res = await fetch(`${API_BASE}${path}`, {
        headers: { Accept: 'text/event-stream', ...authHeaders() },
        signal,
      });
      if (!res.ok || !res.body) {
        handlers.onError?.();
        await sleepAbortable(delay, signal);
        continue;
      }
      handlers.onOpen?.();

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        // Frames are separated by a blank line; deliver complete data frames.
        let sep: number;
        while ((sep = buffer.indexOf('\n\n')) >= 0) {
          const frame = buffer.slice(0, sep);
          buffer = buffer.slice(sep + 2);
          const data = parseDataFrame(frame);
          if (data !== null) handlers.onMessage(data);
        }
      }
      // Server closed the stream cleanly: reconnect.
      handlers.onError?.();
      await sleepAbortable(delay, signal);
    } catch {
      if (signal.aborted) return;
      handlers.onError?.();
      await sleepAbortable(delay, signal);
    }
  }
}

// parseDataFrame extracts the joined data payload of one SSE frame; comment
// frames (starting with ':') return null.
function parseDataFrame(frame: string): string | null {
  const dataLines: string[] = [];
  for (const line of frame.split('\n')) {
    if (line.startsWith(':')) continue;
    if (line.startsWith('data:')) dataLines.push(line.slice(5).replace(/^ /, ''));
  }
  if (dataLines.length === 0) return null;
  return dataLines.join('\n');
}

function sleepAbortable(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const t = setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    const onAbort = () => {
      clearTimeout(t);
      resolve();
    };
    if (signal.aborted) onAbort();
    else signal.addEventListener('abort', onAbort, { once: true });
  });
}
