import { afterEach, describe, expect, it, vi } from 'vitest';
import { listPlugins, installPlugin, setPluginEnabled, uninstallPlugin } from './pluginsService';

afterEach(() => {
  vi.unstubAllGlobals();
});

function lastCall(): [string, RequestInit] {
  const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
  return calls[calls.length - 1] as [string, RequestInit];
}

describe('pluginsService contract (P3)', () => {
  it('listPlugins GETs /api/plugins and tolerates non-array payloads', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () =>
      new Response(JSON.stringify([{ id: 'acme', enabled: false }]), { status: 200 })
    ));
    const items = await listPlugins();
    expect(items).toHaveLength(1);
    expect(lastCall()[0]).toContain('/api/plugins');

    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => new Response('null', { status: 200 })));
    await expect(listPlugins()).resolves.toEqual([]);
  });

  it('installPlugin posts multipart with the auth header and unwraps errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () =>
      new Response(JSON.stringify({ plugin: { id: 'acme' }, notes: [] }), { status: 201 })
    ));
    localStorage.setItem('sounds-great-ai:auth-token', 'tok');
    const file = new File(['zip-bytes'], 'plugin.zip', { type: 'application/zip' });
    const result = await installPlugin(file);

    const [url, init] = lastCall();
    expect(url).toContain('/api/plugins/install');
    expect(init.method).toBe('POST');
    expect(init.headers).toMatchObject({ Authorization: 'Bearer tok' });
    expect(init.body).toBeInstanceOf(FormData);
    expect((init.body as FormData).get('package')).toBeInstanceOf(File);
    expect(result.plugin.id).toBe('acme');

    // Non-2xx: the JSON error body is surfaced as message text.
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () =>
      new Response(JSON.stringify({ error: 'plugin.json not found in archive' }), { status: 400 })
    ));
    const err = await installPlugin(file).catch((e) => e);
    expect(err).toBeInstanceOf(Error);
    expect(err.message).toContain('plugin.json not found');
  });

  it('setPluginEnabled PATCHes and uninstallPlugin DELETEs with encoded ids', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () =>
      new Response(JSON.stringify({ id: 'a b', enabled: true }), { status: 200 })
    ));
    await setPluginEnabled('a b', true);
    let [url, init] = lastCall();
    expect(url).toContain('/api/plugins/a%20b');
    expect(init.method).toBe('PATCH');

    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => new Response(null, { status: 200 })));
    await uninstallPlugin('a b');
    [url, init] = lastCall();
    expect(url).toContain('/api/plugins/a%20b');
    expect(init.method).toBe('DELETE');
  });
});
