import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiGet, apiPatch, apiPost, apiPut, apiDelete, ApiError, API_BASE } from './http';

function mockFetch(status = 200, body: unknown = {}): ReturnType<typeof vi.fn> {
  const fn = vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  );
  vi.stubGlobal('fetch', fn);
  return fn;
}

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe('http helpers', () => {
  it('apiGet hits API_BASE with the given path', async () => {
    const fn = mockFetch(200, { ok: true });
    const out = await apiGet<{ ok: boolean }>('/api/threads');
    expect(out).toEqual({ ok: true });
    expect(fn).toHaveBeenCalledTimes(1);
    const [url, init] = fn.mock.calls[0];
    expect(url).toBe(`${API_BASE}/api/threads`);
    expect(init.method).toBeUndefined(); // default GET
  });

  it('apiPost sends JSON body with POST method', async () => {
    const fn = mockFetch(201, { id: 't1' });
    await apiPost('/api/threads', { title: 'hi' });
    const [url, init] = fn.mock.calls[0];
    expect(url).toBe(`${API_BASE}/api/threads`);
    expect(init.method).toBe('POST');
    expect(init.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(init.body)).toEqual({ title: 'hi' });
  });

  it('apiPatch uses PATCH', async () => {
    const fn = mockFetch(200, { id: 't1', title: 'renamed' });
    await apiPatch('/api/threads/t1', { title: 'renamed' });
    const [, init] = fn.mock.calls[0];
    expect(init.method).toBe('PATCH');
  });

  it('apiPut uses PUT', async () => {
    const fn = mockFetch(200, {});
    await apiPut('/api/profiles/k', { body: 'x' });
    const [, init] = fn.mock.calls[0];
    expect(init.method).toBe('PUT');
  });

  it('apiDelete uses DELETE and returns void', async () => {
    const fn = mockFetch(200, {});
    await expect(apiDelete('/api/threads/t1')).resolves.toBeUndefined();
    const [, init] = fn.mock.calls[0];
    expect(init.method).toBe('DELETE');
  });

  it('throws ApiError with status on non-2xx', async () => {
    mockFetch(404, {});
    const err = await apiGet('/api/nope').catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(404);
  });

  it('attaches Authorization header when a token is stored', async () => {
    localStorage.setItem('sounds-great-ai:auth-token', 'sekrit');
    const fn = mockFetch(200, []);
    await apiGet('/api/threads');
    const [, init] = fn.mock.calls[0];
    expect(init.headers.Authorization).toBe('Bearer sekrit');
  });
});
