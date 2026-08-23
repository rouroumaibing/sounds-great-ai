import { afterEach, describe, expect, it, vi } from 'vitest';
import { listOpportunities, dismissOpportunity, convertOpportunity } from './dossierService';

function lastCall(): [string, RequestInit] {
  const calls = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls;
  return calls[calls.length - 1] as [string, RequestInit];
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('dossierService opportunities contract', () => {
  it('listOpportunities unwraps {opportunities}', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({ opportunities: [{ opportunityId: 'o1', status: 'pending' }] }),
        { status: 200 }
      )
    ));
    const opps = await listOpportunities();
    expect(opps).toHaveLength(1);
    expect(lastCall()[0]).toContain('/api/dossier/distillation-opportunities');
  });

  it('listOpportunities tolerates missing field', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({}), { status: 200 })));
    await expect(listOpportunities()).resolves.toEqual([]);
  });

  it('dismissOpportunity POSTs to the dismiss endpoint', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 })));
    await dismissOpportunity('o1');
    const [url, init] = lastCall();
    expect(url).toContain('/api/dossier/distillation-opportunities/o1/dismiss');
    expect(init.method).toBe('POST');
  });

  it('convertOpportunity POSTs the linked proposalId', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 })));
    await convertOpportunity('o1', 'p9');
    const [url, init] = lastCall();
    expect(url).toContain('/api/dossier/distillation-opportunities/o1/convert');
    expect(JSON.parse(init.body as string)).toEqual({ proposalId: 'p9' });
  });
});
