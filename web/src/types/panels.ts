// Panel config types (mirrors internal/settings/panel_config.go wire format).

export interface ConciergeConfig {
  avatar: string;
  color: string;
  size: number;
  personality: string;
  greeting: string;
  dutyBreed: string;
  autoSuggestThreshold: number;
  proactivityLevel: 'low' | 'medium' | 'high';
}

export interface VoiceGlossaryEntry {
  source: string;
  target: string;
}

export interface VoiceConfig {
  enabled: boolean;
  ttsVoice: string;
  ttsLang: string;
  ttsSpeed: number;
  ttsRefAudio: string;
  sttModel: string;
  sttLanguage: string;
  sttAutoTranscribe: boolean;
  glossary: VoiceGlossaryEntry[];
}

export type ConnectorType = 'slack' | 'telegram' | 'webhook' | 'generic';

// Masked read model: the raw auth key never crosses the wire — only
// auth_key_set / auth_key_preview (accounts-credentials discipline).
export interface Connector {
  id: string;
  name: string;
  type: ConnectorType;
  endpoint: string;
  enabled: boolean;
  last_check?: string;
  auth_key_set: boolean;
  auth_key_preview?: string;
}

export interface ConnectorProbeResult {
  ok: boolean;
  latency_ms: number;
  status: string;
  error?: string;
}
