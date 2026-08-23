import { apiGet, apiPatch, apiDelete, apiPost, API_BASE, authHeaders } from './http';
import type { PluginView, InstallResult } from '../types/plugins';

// Plugins client (panels-roadmap P3 + P4 marketplace). Local install posts
// multipart form data; marketplace browse/install go through the server-side
// proxy (index cache + ed25519 signature verification).

export async function listPlugins(): Promise<PluginView[]> {
  const data = await apiGet<PluginView[]>('/api/plugins');
  return Array.isArray(data) ? data : [];
}

export async function installPlugin(file: File): Promise<InstallResult> {
  const form = new FormData();
  form.append('package', file);
  const res = await fetch(`${API_BASE}/api/plugins/install`, {
    method: 'POST',
    headers: { ...authHeaders() },
    body: form,
  });
  if (!res.ok) {
    const text = await res.text();
    let msg = text;
    try {
      msg = JSON.parse(text).error ?? text;
    } catch { /* raw body */ }
    throw new Error(`${res.status}: ${msg}`);
  }
  return res.json() as Promise<InstallResult>;
}

export async function setPluginEnabled(id: string, enabled: boolean): Promise<PluginView> {
  return apiPatch<PluginView>(`/api/plugins/${encodeURIComponent(id)}`, { enabled });
}

export async function uninstallPlugin(id: string): Promise<void> {
  await apiDelete(`/api/plugins/${encodeURIComponent(id)}`);
}

// --- Marketplace (P4) ---

export interface MarketplaceItem {
  id: string;
  name: string;
  version: string;
  description?: string;
  publisher?: string;
  homepage?: string;
  installs?: number;
  installed: boolean;
}

export interface MarketplaceListing {
  plugins: MarketplaceItem[];
  note?: string;
}

export async function browseMarketplace(query = ''): Promise<MarketplaceListing> {
  const q = query ? `?query=${encodeURIComponent(query)}` : '';
  return apiGet<MarketplaceListing>(`/api/marketplace${q}`);
}

// installFromMarketplace downloads + verifies server-side, then installs
// through the same P3 path (disabled by default pending skill approval).
export async function installFromMarketplace(id: string): Promise<InstallResult> {
  return apiPost<InstallResult>('/api/marketplace/install', { id });
}
