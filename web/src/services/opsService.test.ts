import { afterEach, describe, expect, it, vi } from 'vitest';
import { getQcStatus, runQc, parseMetricLines } from './opsService';

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('opsService QC contract', () => {
  it('getQcStatus reads /api/qc/status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ passed: true, risk_tier: 'low', last_run: '2026-01-01T00:00:00Z' }), { status: 200 })
    ));
    const st = await getQcStatus();
    const [url, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/api/qc/status');
    expect(init.method).toBeUndefined();
    expect(st.passed).toBe(true);
    expect(st.risk_tier).toBe('low');
  });

  it('runQc(false) posts without heavy; runQc(true) adds ?heavy=1', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => new Response(JSON.stringify({}), { status: 200 })));
    await runQc(false);
    let [url, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url.endsWith('/api/qc/run')).toBe(true);
    expect(init.method).toBe('POST');

    await runQc(true);
    [url, init] = (fetch as ReturnType<typeof vi.fn>).mock.calls[1] as [string, RequestInit];
    expect(url.endsWith('/api/qc/run?heavy=1')).toBe(true);
  });
});

describe('parseMetricLines', () => {
  it('extracts plain and labeled prometheus samples', () => {
    const text = '# HELP x\nfoo_total 12\nbar{a="b"} 3.5\n';
    const out = parseMetricLines(text);
    expect(out).toEqual([
      { name: 'foo_total', value: '12' },
      { name: 'bar', value: '3.5' },
    ]);
  });
});
