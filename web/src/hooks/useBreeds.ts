import { useCallback, useEffect, useState } from 'react';
import { breedService, breedConfigToDogAgent } from '../services/breedService';
import { useAppStore } from '../store/useAppStore';
import type { BreedConfig } from '../types/api';
import type { DogAgent } from '../types';
import { useI18n } from '../store/useI18n';

let breedCache: BreedConfig[] | null = null;

export function useBreeds() {
  const [breeds, setBreeds] = useState<BreedConfig[]>(breedCache ?? []);
  const [dogs, setDogs] = useState<DogAgent[]>([]);
  const [loading, setLoading] = useState(breedCache === null);
  const [error, setError] = useState<string | null>(null);
  const { t } = useI18n();
  const showToast = useAppStore((s) => s.showToast);

  const fetchBreeds = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await breedService.getBreeds();
      breedCache = data;
      setBreeds(data);
      setDogs(data.map(breedConfigToDogAgent));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: t('hooks.usebreeds.s1').replace('{msg}', msg), type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => {
    fetchBreeds();
  }, [fetchBreeds]);

  const toggleEnabled = useCallback(async (id: string, enabled: boolean) => {
    try {
      await breedService.updateBreedEnabled(id, enabled);
      if (breedCache) {
        breedCache = breedCache.map((b) => b.id === id ? { ...b, enabled } : b);
        setBreeds(breedCache);
        setDogs(breedCache.map(breedConfigToDogAgent));
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: t('hooks.usebreeds.s2').replace('{msg}', msg), type: 'error' });
    }
  }, [showToast]);

  const deleteBreed = useCallback(async (id: string) => {
    try {
      await breedService.deleteBreed(id);
      if (breedCache) {
        breedCache = breedCache.filter((b) => b.id !== id);
        setBreeds(breedCache);
        setDogs(breedCache.map(breedConfigToDogAgent));
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: t('hooks.usebreeds.s3').replace('{msg}', msg), type: 'error' });
    }
  }, [showToast]);

  const createBreed = useCallback(async (config: BreedConfig) => {
    const created = await breedService.createBreed(config);
    await fetchBreeds();
    return created;
  }, [fetchBreeds]);

  const updateBreed = useCallback(async (id: string, updates: Partial<BreedConfig>) => {
    const updated = await breedService.updateBreed(id, updates);
    await fetchBreeds();
    return updated;
  }, [fetchBreeds]);

  return { breeds, dogs, loading, error, toggleEnabled, deleteBreed, createBreed, updateBreed, refetch: fetchBreeds };
}
