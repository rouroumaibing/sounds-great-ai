package config

import "testing"

func TestDefaultLeaderConfig(t *testing.T) {
	cfg := DefaultLeaderConfig()
	if cfg.Name != "You" {
		t.Errorf("Name = %q, want %q", cfg.Name, "You")
	}
	if len(cfg.MentionPatterns) != 3 {
		t.Errorf("MentionPatterns len = %d, want 3", len(cfg.MentionPatterns))
	}
	if cfg.TimeZone != "Asia/Shanghai" {
		t.Errorf("TimeZone = %q, want %q", cfg.TimeZone, "Asia/Shanghai")
	}
}

func TestLeaderConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     LeaderConfig
		wantErr bool
	}{
		{
			name:    "valid",
			cfg:     DefaultLeaderConfig(),
			wantErr: false,
		},
		{
			name: "empty name",
			cfg: LeaderConfig{
				Name:            "",
				MentionPatterns: []string{"@leader"},
			},
			wantErr: true,
		},
		{
			name: "no mention patterns",
			cfg: LeaderConfig{
				Name:            "ME",
				MentionPatterns: []string{},
			},
			wantErr: true,
		},
		{
			name: "nil mention patterns",
			cfg: LeaderConfig{
				Name:            "ME",
				MentionPatterns: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
