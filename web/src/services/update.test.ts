import { describe, it, expect, vi, afterEach } from 'vitest';
import { isStaleChunkError, tryAutoReload } from './update';

describe('isStaleChunkError', () => {
  it('matches dynamic-import failures across browsers', () => {
    expect(
      isStaleChunkError(new TypeError('Failed to fetch dynamically imported module: http://localhost:8080/assets/M-1.js')),
    ).toBe(true);
    expect(isStaleChunkError(new Error('Importing a module script failed.'))).toBe(true);
    expect(isStaleChunkError(new Error('error loading dynamically imported module'))).toBe(true);
  });

  it('matches the module-script MIME refusal', () => {
    expect(
      isStaleChunkError(
        new Error(
          "Failed to load module script: Expected a JavaScript-or-Wasm module script but the server responded with a MIME type of 'text/html'",
        ),
      ),
    ).toBe(true);
  });

  it('rejects unrelated errors', () => {
    expect(isStaleChunkError(new Error('test error'))).toBe(false);
    expect(isStaleChunkError(undefined)).toBe(false);
    expect(isStaleChunkError(42)).toBe(false);
  });
});

describe('tryAutoReload', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    sessionStorage.clear();
  });

  it('reloads once, then throttles repeat attempts', () => {
    const reload = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { ...window.location, reload },
      writable: true,
    });

    expect(tryAutoReload()).toBe(true);
    expect(reload).toHaveBeenCalledTimes(1);
    // Within the throttle window recovery must not loop even if the
    // boundary error fires again on the next render.
    expect(tryAutoReload()).toBe(false);
    expect(reload).toHaveBeenCalledTimes(1);
  });
});
