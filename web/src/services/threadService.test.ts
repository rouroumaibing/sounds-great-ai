import { afterEach, describe, expect, it, vi } from 'vitest';
import { threadService } from './threadService';

function lastCall(): [string, RequestInit] {
  const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
  return calls[calls.length - 1] as [string, RequestInit];
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('threadService contract', () => {
  it('getThreads returns array', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify([{ id: 't1', title: 'a', created_at: 1 }]), { status: 200 })
    ));
    const threads = await threadService.getThreads();
    expect(threads).toHaveLength(1);
    expect(lastCall()[0]).toContain('/api/threads');
  });

  it('getThreads tolerates non-array payloads', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify(null), { status: 200 })
    ));
    await expect(threadService.getThreads()).resolves.toEqual([]);
  });

  it('createThread POSTs a title', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 't2', title: 'x', created_at: 2 }), { status: 201 })
    ));
    await threadService.createThread('x');
    const [url, init] = lastCall();
    expect(url).toContain('/api/threads');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual({ title: 'x' });
  });

  it('renameThread PATCHes the new title at /api/threads/{id}', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 't1', title: 'renamed' }), { status: 200 })
    ));
    const out = await threadService.renameThread('t1', 'renamed');
    const [url, init] = lastCall();
    expect(url).toContain('/api/threads/t1');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body as string)).toEqual({ title: 'renamed' });
    expect(out.title).toBe('renamed');
  });

  it('deleteThread issues DELETE', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 200 })));
    await threadService.deleteThread('t1');
    const [url, init] = lastCall();
    expect(url).toContain('/api/threads/t1');
    expect(init.method).toBe('DELETE');
  });

  it('getThreadEvents unwraps {events}', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ events: [{ type: 'cvo_message' }] }), { status: 200 })
    ));
    const events = await threadService.getThreadEvents('t1');
    expect(events).toEqual([{ type: 'cvo_message' }]);
  });
});
