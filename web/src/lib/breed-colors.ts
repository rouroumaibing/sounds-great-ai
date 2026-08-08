/**
 * Centralized breed fallback colors (single source of truth).
 *
 * These hex values are JS-level fallbacks for when breed config hasn't
 * loaded yet. They feed JS color computation (inline style derivation)
 * — CSS tokens can't serve this role because the values must be manipulable in JS.
 *
 * Breed persona CSS tokens (--color-{breedId}-*) are injected dynamically by
 * CatHueInjector from the live breed catalog. These fallbacks are the safety net
 * for pre-load, SSR, and edge cases where the catalog hasn't arrived.
 */

export interface BreedColorPair {
  primary: string;
  secondary: string;
}

/** Breed persona fallback colors keyed by breedId. */
export const BREED_COLOR_DEFAULTS: Record<string, BreedColorPair> = {
  bianmu: { primary: '#4A90D9', secondary: '#D4E6F7' },
  xigou: { primary: '#E84393', secondary: '#F8D7E8' },
  jinmao: { primary: '#F39C12', secondary: '#FDEBD0' },
  demu: { primary: '#2C3E50', secondary: '#D5DBDB' },
  zangao: { primary: '#8E44AD', secondary: '#E8DAEF' },
  zhonghuatianyuanquan: { primary: '#27AE60', secondary: '#D5F5E3' },
};

/** Fallback when breedId is unknown or color data is missing entirely. */
export const UNKNOWN_BREED_COLOR: BreedColorPair = { primary: '#9B7EBD', secondary: '#E8DFF5' };

/* ── Helpers ── */

/** Get breed color pair, preferring live API data, falling back to static defaults. */
export function getBreedColor(
  breedId: string,
  breedData?: { colorPrimary?: string; colorSecondary?: string },
): BreedColorPair {
  if (breedData?.colorPrimary) {
    return { primary: breedData.colorPrimary, secondary: breedData.colorSecondary ?? breedData.colorPrimary };
  }
  return BREED_COLOR_DEFAULTS[breedId] ?? UNKNOWN_BREED_COLOR;
}
