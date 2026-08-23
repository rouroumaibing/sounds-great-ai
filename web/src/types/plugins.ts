// Plugin lifecycle types (mirrors internal/plugins wire format, P3).

export interface PluginView {
  id: string;
  name: string;
  version: string;
  description?: string;
  permissions?: string[];
  enabled: boolean;
  installed_at: string;
  breeds: string[];
  skills: string[];
}

export interface InstallResult {
  plugin: PluginView;
  notes: string[];
}

export interface MarketplaceStub {
  plugins: unknown[];
  note: string;
}
