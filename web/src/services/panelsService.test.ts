import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  getConcierge,
  patchConcierge,
  getVoice,
  patchVoice,
  listConnectors,
  createConnector,
  updateConnector,
  deleteConnector,
  testConnector,
} from './panelsService';

function lastCall(): [string, RequestInit] {
  const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
  return calls[calls.length - 1] as [string, RequestInit];
}

function mockOK(body: unknown, status = 200) {
  vi.stubGlobal('fetch', vi.fn().mockImplementation(async () =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  ));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('panelsService contract (panels-roadmap P1+P2)', () => {
  it('getConcierge / patchConcierge hit the concierge doc endpoint', async () => {
    mockOK({ avatar: '🐕', color: '#4A90D9' });
    await getConcierge();
    let [url, init] = lastCall();
    expect(url).toContain('/api/config/concierge');
    expect(init.method).toBeUndefined();

    await patchConcierge({ greeting: 'hi' });
    [url, init] = lastCall();
    expect(url).toContain('/api/config/concierge');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body as string)).toEqual({ greeting: 'hi' });
  });

  it('getVoice / patchVoice hit the voice doc endpoint', async () => {
    mockOK({ enabled: true, ttsVoice: 'alloy' });
    await getVoice();
    await patchVoice({ ttsSpeed: 1.5 });
    const [url, init] = lastCall();
    expect(url).toContain('/api/config/voice');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body as string)).toEqual({ ttsSpeed: 1.5 });
  });

  it('connector CRUD hits the registry endpoints with correct methods', async () => {
    mockOK([]);
    await listConnectors();
    expect(lastCall()[0]).toContain('/api/config/connectors');

    mockOK({ id: 'c1', name: 'ops', type: 'webhook' }, 201);
    await createConnector({ name: 'ops', type: 'webhook', endpoint: 'https://x.io' });
    let [url, init] = lastCall();
    expect(url).toContain('/api/config/connectors');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toMatchObject({ name: 'ops', type: 'webhook' });

    await updateConnector('c1', { enabled: false });
    [url, init] = lastCall();
    expect(url).toContain('/api/config/connectors/c1');
    expect(init.method).toBe('PATCH');

    mockOK(null, 200);
    await deleteConnector('c1');
    [url, init] = lastCall();
    expect(url).toContain('/api/config/connectors/c1');
    expect(init.method).toBe('DELETE');

    mockOK({ ok: true, latency_ms: 12, status: 'reachable' });
    await testConnector('c1');
    [url, init] = lastCall();
    expect(url).toContain('/api/config/connectors/c1/test');
    expect(init.method).toBe('POST');
  });

  it('connector ids are URL-encoded', async () => {
    mockOK(null, 200);
    await deleteConnector('a b/c');
    expect(lastCall()[0]).toContain('/api/config/connectors/a%20b%2Fc');
  });

  it('listConnectors tolerates non-array payloads', async () => {
    mockOK(null);
    await expect(listConnectors()).resolves.toEqual([]);
  });
});
