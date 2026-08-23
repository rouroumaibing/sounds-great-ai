package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// PanelConfigStore persists the UI-panel configuration documents (concierge,
// voice, IM connectors) that previously lived as hardcoded stubs in the
// panels HTTP handler. Each document is a JSON file under
// <ConfigRoot>/panels/, written atomically. Defaults are applied on read so
// a fresh install (or a deleted file) degrades to the documented defaults
// rather than an error.
type PanelConfigStore struct {
	mu  sync.Mutex
	dir string
}

// NewPanelConfigStore creates (or opens) the panel config store under
// configRoot. The directory is created lazily on first write.
func NewPanelConfigStore(configRoot string) *PanelConfigStore {
	return &PanelConfigStore{dir: filepath.Join(configRoot, "panels")}
}

// --- Concierge ---

// ConciergeConfig is the concierge (迎宾犬) panel document.
type ConciergeConfig struct {
	Avatar               string `json:"avatar"`
	Color                string `json:"color"`
	Size                 int    `json:"size"`
	Personality          string `json:"personality"`
	Greeting             string `json:"greeting"`
	DutyBreed            string `json:"dutyBreed"`
	AutoSuggestThreshold int    `json:"autoSuggestThreshold"`
	ProactivityLevel     string `json:"proactivityLevel"`
}

func defaultConcierge() ConciergeConfig {
	return ConciergeConfig{
		Avatar:               "🐕",
		Color:                "#4A90D9",
		Size:                 56,
		AutoSuggestThreshold: 3,
		ProactivityLevel:     "medium",
	}
}

// LoadConcierge returns the persisted concierge config with defaults filled
// in for absent fields.
func (s *PanelConfigStore) LoadConcierge() (ConciergeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := defaultConcierge()
	if err := readDoc(s.dir, "concierge.json", &cfg); err != nil {
		return defaultConcierge(), err
	}
	if cfg.Avatar == "" {
		cfg.Avatar = "🐕"
	}
	if cfg.Color == "" {
		cfg.Color = "#4A90D9"
	}
	if cfg.Size <= 0 {
		cfg.Size = 56
	}
	if cfg.AutoSuggestThreshold <= 0 {
		cfg.AutoSuggestThreshold = 3
	}
	if cfg.ProactivityLevel == "" {
		cfg.ProactivityLevel = "medium"
	}
	return cfg, nil
}

// SaveConcierge persists the concierge config atomically.
func (s *PanelConfigStore) SaveConcierge(cfg ConciergeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeDoc(s.dir, "concierge.json", cfg)
}

// --- Voice ---

// VoiceGlossaryEntry is one TTS pronunciation override (source → target).
type VoiceGlossaryEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// VoiceConfig is the voice (TTS/STT) panel document. SG stores configuration
// only — actual TTS/STT inference is out of scope for this store.
type VoiceConfig struct {
	Enabled          bool                 `json:"enabled"`
	TTSVoice         string               `json:"ttsVoice"`
	TTSLang          string               `json:"ttsLang"`
	TTSSpeed         float64              `json:"ttsSpeed"`
	TTSRefAudio      string               `json:"ttsRefAudio"`
	STTModel         string               `json:"sttModel"`
	STTLanguage      string               `json:"sttLanguage"`
	STTAutoTranscrib bool                 `json:"sttAutoTranscribe"`
	Glossary         []VoiceGlossaryEntry `json:"glossary"`
}

func defaultVoice() VoiceConfig {
	return VoiceConfig{
		Enabled:          false,
		TTSVoice:         "alloy",
		TTSLang:          "zh-CN",
		TTSSpeed:         1.0,
		STTModel:         "whisper-1",
		STTLanguage:      "zh",
		STTAutoTranscrib: true,
		Glossary:         []VoiceGlossaryEntry{},
	}
}

// LoadVoice returns the persisted voice config with defaults filled in.
func (s *PanelConfigStore) LoadVoice() (VoiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := defaultVoice()
	if err := readDoc(s.dir, "voice.json", &cfg); err != nil {
		return defaultVoice(), err
	}
	if cfg.TTSVoice == "" {
		cfg.TTSVoice = "alloy"
	}
	if cfg.TTSLang == "" {
		cfg.TTSLang = "zh-CN"
	}
	if cfg.TTSSpeed <= 0 {
		cfg.TTSSpeed = 1.0
	}
	if cfg.STTModel == "" {
		cfg.STTModel = "whisper-1"
	}
	if cfg.STTLanguage == "" {
		cfg.STTLanguage = "zh"
	}
	if cfg.Glossary == nil {
		cfg.Glossary = []VoiceGlossaryEntry{}
	}
	return cfg, nil
}

// SaveVoice persists the voice config atomically.
func (s *PanelConfigStore) SaveVoice(cfg VoiceConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeDoc(s.dir, "voice.json", cfg)
}

// --- Connectors ---

// Connector is one registered IM/webhook connector. AuthKey is the raw
// credential — it is persisted to disk (file mode 0600) but NEVER returned
// over HTTP; transport layers expose only AuthKeySet / AuthKeyPreview
// (same discipline as accounts credentials).
type Connector struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // slack | telegram | webhook | generic
	Endpoint  string `json:"endpoint"`
	AuthKey   string `json:"auth_key,omitempty"` // persisted, masked on HTTP read
	Enabled   bool   `json:"enabled"`
	LastCheck string `json:"last_check,omitempty"` // last probe result summary
}

// ConnectorsDoc is the persisted connectors registry.
type ConnectorsDoc struct {
	Connectors []Connector `json:"connectors"`
}

// ListConnectors returns the connector registry (secrets intact, for
// internal use only — HTTP layers must mask before responding).
func (s *PanelConfigStore) ListConnectors() ([]Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := ConnectorsDoc{Connectors: []Connector{}}
	if err := readDoc(s.dir, "connectors.json", &doc); err != nil {
		return []Connector{}, nil // missing file = empty registry, not an error
	}
	if doc.Connectors == nil {
		doc.Connectors = []Connector{}
	}
	return doc.Connectors, nil
}

// SaveConnectors replaces the whole connector registry atomically.
func (s *PanelConfigStore) SaveConnectors(list []Connector) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if list == nil {
		list = []Connector{}
	}
	return writeDoc(s.dir, "connectors.json", ConnectorsDoc{Connectors: list})
}

// --- shared doc IO ---

func readDoc(dir, name string, into any) error {
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // keep defaults
		}
		return fmt.Errorf("read panel doc %s: %w", name, err)
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("parse panel doc %s: %w", name, err)
	}
	return nil
}

func writeDoc(dir, name string, doc any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create panel dir: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal panel doc %s: %w", name, err)
	}
	return writeAtomicRaw(filepath.Join(dir, name), data, 0o600)
}
