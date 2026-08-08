import { apiGet, apiPost, apiPatch, apiDelete } from './http';
import { getBreedColor } from '../lib/breed-colors';
import type { BreedConfig } from '../types/api';
import type { DogAgent } from '../types';

export function breedConfigToDogAgent(config: BreedConfig): DogAgent {
  const colorPair = getBreedColor(config.id, {
    colorPrimary: config.color?.primary,
    colorSecondary: config.color?.secondary,
  });
  const color = colorPair.primary;
  const icon = config.avatar || '';
  const variant = config.variants?.find(v => v.id === config.default_variant_id) ?? config.variants?.[0];
  return {
    id: config.id,
    name: `${config.name} (${config.display_name})`,
    title: config.display_name,
    role: config.display_name,
    color,
    icon,
    adapter: variant?.client_id ?? 'unknown',
    model: variant?.default_model ?? 'unknown',
    latency: '—',
    statusText: config.enabled ? 'READY' : 'DISABLED',
    statusBadgeClass: config.enabled
      ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30'
      : 'bg-slate-800 text-slate-500 border-slate-700',
  };
}

export const breedService = {
  async getBreeds(): Promise<BreedConfig[]> {
    const data = await apiGet<BreedConfig[]>('/api/breeds');
    return Array.isArray(data) ? data : [];
  },

  async createBreed(config: BreedConfig): Promise<BreedConfig> {
    return apiPost<BreedConfig>('/api/breeds', config);
  },

  async deleteBreed(id: string): Promise<void> {
    await apiDelete(`/api/breeds/${id}`);
  },

  async updateBreedEnabled(id: string, enabled: boolean): Promise<BreedConfig> {
    return apiPost<BreedConfig>('/api/breeds', { id, enabled });
  },

  async updateBreed(id: string, updates: Partial<BreedConfig>): Promise<BreedConfig> {
    return apiPatch<BreedConfig>(`/api/breeds/${id}`, updates);
  },
};
