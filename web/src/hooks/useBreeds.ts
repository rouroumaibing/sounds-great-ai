import { useCallback, useEffect, useState } from 'react';
import { breedService, breedConfigToDogAgent } from '../services/breedService';
import { useAppStore } from '../store/useAppStore';
import type { BreedConfig } from '../types/api';
import type { DogAgent } from '../types';

let breedCache: BreedConfig[] | null = null;

export function useBreeds() {
  const [breeds, setBreeds] = useState<BreedConfig[]>(breedCache ?? []);
  const [dogs, setDogs] = useState<DogAgent[]>([]);
  const [loading, setLoading] = useState(breedCache === null);
  const [error, setError] = useState<string | null>(null);
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
      showToast({ message: `加载犬种失败: ${msg}`, type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => {
    if (breedCache) {
      setDogs(breedCache.map(breedConfigToDogAgent));
      setLoading(false);
      return;
    }
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
      showToast({ message: `更新犬种失败: ${msg}`, type: 'error' });
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
      showToast({ message: `删除犬种失败: ${msg}`, type: 'error' });
    }
  }, [showToast]);

  return { breeds, dogs, loading, error, toggleEnabled, deleteBreed, refetch: fetchBreeds };
}
