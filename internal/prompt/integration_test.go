package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/skills"

	"sounds-great-ai/pkg/pack"
)

// TestIntegrationPromptBuilderWithRealBreeds loads actual breed configs
// from packs/default/breeds/ and verifies the system prompt output.
func TestIntegrationPromptBuilderWithRealBreeds(t *testing.T) {
	breedsDir := "../../packs/default/breeds"
	skillsDir := "../../packs/default/skills"

	// Skip if running in environment without breed configs
	if _, err := os.Stat(breedsDir); err != nil {
		t.Skipf("breeds dir not found: %v", err)
	}

	// Load real breed configs from the single consolidated template file.
	loader := pack.NewLoader()
	breeds, err := loader.LoadFromFile(filepath.Join(breedsDir, "dog-template.json"))
	if err != nil {
		t.Fatalf("failed to load breeds: %v", err)
	}

	// Load skills (may be empty after cleanup)
	skillMgr := skills.NewManager(skillsDir)
	_ = skillMgr.LoadFromDir()

	// Create builder
	b := NewBuilder(breeds, skillMgr)

	// Test each breed
	for breedID, breed := range breeds {
		t.Run(breedID, func(t *testing.T) {
			result := b.Build(BuildRequest{BreedID: breedID})

			// 1. Must contain identity section
			if !strings.Contains(result, "# 身份") {
				t.Error("missing # 身份 section")
			}

			// 2. Must contain breed display name
			if !strings.Contains(result, breed.DisplayName) {
				t.Errorf("missing display name %q", breed.DisplayName)
			}

			// 3. Must contain restrictions section
			if !strings.Contains(result, "## 限制") {
				t.Error("missing ## 限制 section")
			}

			// 4. Must contain teammate roster (if >1 breed)
			if len(breeds) > 1 {
				if !strings.Contains(result, "## 队友名册") {
					t.Error("missing ## 队友名册 section")
				}
				// Must NOT contain self in roster
				lines := strings.Split(result, "\n")
				for _, line := range lines {
					if strings.Contains(line, "|") && strings.Contains(line, breed.DisplayName) {
						// This line has the breed name in a table row — could be roster
						// The roster header line is "| 狗狗 | @mention |" so skip that
						if !strings.Contains(line, "狗狗") && !strings.Contains(line, "---") {
							t.Errorf("self appears in roster: %s", line)
						}
					}
				}
			}

			// 5. Must contain variant system prompt if configured
			variant := breed.DefaultVariant()
			if variant != nil && variant.SystemPrompt != "" {
				if !strings.Contains(result, "## 角色提示") {
					t.Error("missing ## 角色提示 section")
				}
				if !strings.Contains(result, variant.SystemPrompt) {
					t.Errorf("missing variant system prompt: %q", variant.SystemPrompt)
				}
			}

			// 6. Output must be non-empty
			if len(result) == 0 {
				t.Error("expected non-empty system prompt")
			}

			// Log the actual output for manual inspection
			t.Logf("System prompt for %s (%d chars):\n%s", breedID, len(result), result)
		})
	}
}

// TestIntegrationPromptBuilderRosterContent verifies the roster table
// contains all other breeds with correct mention patterns.
func TestIntegrationPromptBuilderRosterContent(t *testing.T) {
	breedsDir := "../../packs/default/breeds"
	if _, err := os.Stat(breedsDir); err != nil {
		t.Skipf("breeds dir not found: %v", err)
	}

	loader := pack.NewLoader()
	breeds, err := loader.LoadFromFile(filepath.Join(breedsDir, "dog-template.json"))
	if err != nil {
		t.Fatalf("failed to load breeds: %v", err)
	}

	b := NewBuilder(breeds, nil)

	// Build for bianmu and check roster contains other breeds
	result := b.Build(BuildRequest{BreedID: "bianmu"})

	for breedID, breed := range breeds {
		if breedID == "bianmu" {
			continue
		}

		// Each other breed should appear in roster
		if !strings.Contains(result, breed.DisplayName) {
			t.Errorf("roster missing breed %s (%s)", breedID, breed.DisplayName)
		}

		// Check mention pattern if configured
		if len(breed.MentionPatterns) > 0 {
			mention := breed.MentionPatterns[0]
			if !strings.Contains(result, mention) {
				t.Errorf("roster missing mention pattern %s for breed %s", mention, breedID)
			}
		}
	}
}
