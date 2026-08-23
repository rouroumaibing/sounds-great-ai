import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  proposeCapsule,
  deleteCapsule,
  upsertCapsule,
  getProposal,
} from './profilesService';

function lastCall(): [string, RequestInit] {
  const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
  return calls[calls.length - 1] as [string, RequestInit];
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('profilesService contract', () => {
  it('proposeCapsule POSTs to the propose endpoint', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ status: 'proposed', relationship_key: 'family-x' }), { status: 201 })
    ));
    const out = await proposeCapsule('family-x', 'body text');
    const [url, init] = lastCall();
    expect(url).toContain('/api/profiles/family-x/propose');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual({ body: 'body text', owner_dog: 'operator' });
    expect(out.status).toBe('proposed');
  });

  it('proposeCapsule URL-encodes keys with special characters', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ status: 'proposed', relationship_key: 'a b/c' }), { status: 201 })
    ));
    await proposeCapsule('a b/c', 'x');
    expect(lastCall()[0]).toContain('/api/profiles/a%20b%2Fc/propose');
  });

  it('upsertCapsule PUTs the capsule body', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ relationship_key: 'k', body: 'b' }), { status: 200 })
    ));
    await upsertCapsule('k', 'b');
    const [url, init] = lastCall();
    expect(url).toContain('/api/profiles/k');
    expect(init.method).toBe('PUT');
  });

  it('deleteCapsule issues DELETE', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 200 })));
    await deleteCapsule('k');
    const [url, init] = lastCall();
    expect(url).toContain('/api/profiles/k');
    expect(init.method).toBe('DELETE');
  });

  it('getProposal maps 404 to null (no pending proposal)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: 'no pending proposal' }), { status: 404 })
    ));
    await expect(getProposal('k')).resolves.toBeNull();
  });
});
