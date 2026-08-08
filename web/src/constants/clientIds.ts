export const CLIENT_IDS = [
  { id: 'claude', label: 'Claude (Anthropic)', provider: 'anthropic' },
  { id: 'codex', label: 'Codex (OpenAI)', provider: 'openai' },
  { id: 'gemini', label: 'Gemini (Google)', provider: 'google' },
  { id: 'opencode', label: 'OpenCode', provider: 'opencode' },
  { id: 'kimi', label: 'Kimi (Moonshot)', provider: 'moonshot' },
] as const;

export const CLIENT_ID_VALUES = CLIENT_IDS.map((c) => c.id);

export function providerFromClientId(clientId: string): string {
  const found = CLIENT_IDS.find((c) => c.id === clientId);
  return found?.provider ?? '';
}
