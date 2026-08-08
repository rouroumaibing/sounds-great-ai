/**
 * Breed display name helpers — consistent formatting across human-facing surfaces.
 *
 * API data is primary (displayName/name), static fallback only when breed is unknown.
 */

export interface BreedDisplayNameData {
  displayName?: string;
  name?: string;
  variantLabel?: string;
}

export type GetBreedDisplayNameData = (id: string) => BreedDisplayNameData | undefined;

/** Format one breed consistently across human-facing surfaces. */
export function formatBreedDisplayName(breed: BreedDisplayNameData): string {
  const base = breed.displayName ?? breed.name ?? '';
  return breed.variantLabel ? `${base}（${breed.variantLabel}）` : base;
}

/** Resolve a breedId to a friendly label, retaining the id as the unknown-breed fallback. */
export function resolveBreedDisplayName(breedId: string, getBreedById: GetBreedDisplayNameData): string {
  const breed = getBreedById(breedId);
  return breed ? formatBreedDisplayName(breed) : breedId;
}
