import { apiGet, apiPatch, apiPost, apiDelete } from './http';
import type {
  ConciergeConfig,
  VoiceConfig,
  Connector,
  ConnectorProbeResult,
  ConnectorType,
} from '../types/panels';

// Panels config client (panels-roadmap P1+P2): concierge / voice are
// single-doc GET+PATCH; connectors are a masked CRUD registry with probe.

export async function getConcierge(): Promise<ConciergeConfig> {
  return apiGet<ConciergeConfig>('/api/config/concierge');
}

export async function patchConcierge(patch: Partial<ConciergeConfig>): Promise<ConciergeConfig> {
  return apiPatch<ConciergeConfig>('/api/config/concierge', patch);
}

export async function getVoice(): Promise<VoiceConfig> {
  return apiGet<VoiceConfig>('/api/config/voice');
}

export async function patchVoice(patch: Partial<VoiceConfig>): Promise<VoiceConfig> {
  return apiPatch<VoiceConfig>('/api/config/voice', patch);
}

export async function listConnectors(): Promise<Connector[]> {
  const data = await apiGet<Connector[]>('/api/config/connectors');
  return Array.isArray(data) ? data : [];
}

export async function createConnector(input: {
  name: string;
  type: ConnectorType;
  endpoint: string;
  auth_key?: string;
  enabled?: boolean;
}): Promise<Connector> {
  return apiPost<Connector>('/api/config/connectors', input);
}

export async function updateConnector(
  id: string,
  patch: { name?: string; type?: ConnectorType; endpoint?: string; auth_key?: string; enabled?: boolean },
): Promise<Connector> {
  return apiPatch<Connector>(`/api/config/connectors/${encodeURIComponent(id)}`, patch);
}

export async function deleteConnector(id: string): Promise<void> {
  await apiDelete(`/api/config/connectors/${encodeURIComponent(id)}`);
}

export async function testConnector(id: string): Promise<ConnectorProbeResult> {
  return apiPost<ConnectorProbeResult>(`/api/config/connectors/${encodeURIComponent(id)}/test`, {});
}
