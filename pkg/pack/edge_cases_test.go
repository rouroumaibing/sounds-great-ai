package pack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/pkg/pack/orchestrator"
)

// --- BreedConfig / DefaultVariant edge cases ---

func TestDefaultVariant_Found(t *testing.T) {
	breed := &BreedConfig{
		DefaultVariantID: "v2",
		Variants: []Variant{
			{ID: "v1", ClientID: "a", DefaultModel: "m1"},
			{ID: "v2", ClientID: "b", DefaultModel: "m2"},
		},
	}
	v := breed.DefaultVariant()
	if v == nil {
		t.Fatal("expected non-nil variant")
	}
	if v.ID != "v2" {
		t.Errorf("ID = %q, want %q", v.ID, "v2")
	}
}

func TestDefaultVariant_FallbackFirst(t *testing.T) {
	breed := &BreedConfig{
		DefaultVariantID: "nonexistent",
		Variants: []Variant{
			{ID: "v1", ClientID: "a", DefaultModel: "m1"},
			{ID: "v2", ClientID: "b", DefaultModel: "m2"},
		},
	}
	v := breed.DefaultVariant()
	if v == nil {
		t.Fatal("expected non-nil variant")
	}
	if v.ID != "v1" {
		t.Errorf("ID = %q, want %q (first variant)", v.ID, "v1")
	}
}

func TestDefaultVariant_EmptyVariants(t *testing.T) {
	breed := &BreedConfig{
		DefaultVariantID: "v1",
		Variants:         nil,
	}
	v := breed.DefaultVariant()
	if v != nil {
		t.Fatalf("expected nil for empty variants, got %+v", v)
	}
}

func TestDefaultVariant_SingleVariant(t *testing.T) {
	breed := &BreedConfig{
		DefaultVariantID: "v1",
		Variants: []Variant{
			{ID: "v1", ClientID: "a", DefaultModel: "m1"},
		},
	}
	v := breed.DefaultVariant()
	if v == nil || v.ID != "v1" {
		t.Errorf("expected v1, got %+v", v)
	}
}

// --- Color / CLIConfig / ContextBudget / Variant JSON ---

func TestColorJSONRoundTrip(t *testing.T) {
	in := Color{Primary: "#ff0000", Secondary: "#00ff00"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Color
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("mismatch: %+v vs %+v", out, in)
	}
}

func TestCLIConfigJSONRoundTrip(t *testing.T) {
	in := CLIConfig{
		Command:      "claude",
		OutputFormat: "json",
		DefaultArgs:  []string{"--verbose", "--model", "sonnet"},
		Effort:       "high",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out CLIConfig
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Command != in.Command || out.OutputFormat != in.OutputFormat || out.Effort != in.Effort {
		t.Errorf("mismatch: %+v vs %+v", out, in)
	}
	if len(out.DefaultArgs) != len(in.DefaultArgs) {
		t.Errorf("DefaultArgs len mismatch: %d vs %d", len(out.DefaultArgs), len(in.DefaultArgs))
	}
}

func TestContextBudgetJSONRoundTrip(t *testing.T) {
	in := ContextBudget{MaxPromptTokens: 4096, MaxContextTokens: 128000, MaxMessages: 50}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ContextBudget
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("mismatch: %+v vs %+v", out, in)
	}
}

func TestVariantFullFieldsJSONRoundTrip(t *testing.T) {
	in := Variant{
		ID:           "v1",
		ClientID:     "openai",
		DefaultModel: "gpt-4o",
		MCPSupport:   true,
		CLI: CLIConfig{
			Command:      "claude",
			OutputFormat: "json",
			DefaultArgs:  []string{"--foo"},
			Effort:       "medium",
		},
		SystemPrompt: "You are a helpful assistant.",
		Strengths:    []string{"coding", "analysis"},
		ContextBudget: ContextBudget{
			MaxPromptTokens:  8000,
			MaxContextTokens: 200000,
			MaxMessages:      100,
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Variant
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID != in.ID || out.ClientID != in.ClientID || out.DefaultModel != in.DefaultModel {
		t.Errorf("basic fields mismatch: %+v", out)
	}
	if !out.MCPSupport {
		t.Error("MCPSupport should be true")
	}
	if out.SystemPrompt != in.SystemPrompt {
		t.Errorf("SystemPrompt mismatch: %q", out.SystemPrompt)
	}
	if len(out.Strengths) != 2 {
		t.Errorf("Strengths len = %d, want 2", len(out.Strengths))
	}
	if out.ContextBudget.MaxContextTokens != 200000 {
		t.Errorf("MaxContextTokens = %d", out.ContextBudget.MaxContextTokens)
	}
}

func TestBreedConfigWithAllOptionalFields(t *testing.T) {
	breed := &BreedConfig{
		ID:               "test-breed",
		Name:             "test",
		DisplayName:      "Test Breed",
		Avatar:           "🐕",
		Color:            &Color{Primary: "#abc", Secondary: "#def"},
		Personality:      "diligent",
		RoleDescription:  "code reviewer",
		TeamStrengths:    "fast review",
		MentionPatterns:  []string{"@test"},
		Roles:            []string{"reviewer", "checker"},
		DefaultVariantID: "v1",
		Variants: []Variant{{
			ID:           "v1",
			ClientID:     "test",
			DefaultModel: "m",
		}},
		Source:  BreedSourceUser,
		Enabled: true,
	}
	data, err := json.Marshal(breed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded BreedConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Avatar != "🐕" {
		t.Errorf("Avatar = %q", decoded.Avatar)
	}
	if decoded.Color == nil || decoded.Color.Primary != "#abc" {
		t.Errorf("Color mismatch: %+v", decoded.Color)
	}
	if decoded.RoleDescription != "code reviewer" {
		t.Errorf("RoleDescription = %q", decoded.RoleDescription)
	}
	if len(decoded.Roles) != 2 {
		t.Errorf("Roles len = %d", len(decoded.Roles))
	}
	if !decoded.Enabled {
		t.Error("Enabled should be true")
	}
}

// --- Pack edge cases ---

func TestGetBreedNonexistent(t *testing.T) {
	p := New("test")
	if got := p.GetBreed("nonexistent"); got != nil {
		t.Errorf("expected nil for nonexistent breed, got %+v", got)
	}
}

func TestGetBreedExisting(t *testing.T) {
	p := New("test")
	breed := &BreedConfig{
		ID:               "mybreed",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	p.Register(breed)
	got := p.GetBreed("mybreed")
	if got == nil || got.ID != "mybreed" {
		t.Errorf("GetBreed returned wrong breed: %+v", got)
	}
}

func TestRegisterOverwriteUserBreed(t *testing.T) {
	p := New("test")
	breed1 := &BreedConfig{
		ID:               "breed1",
		Name:             "original",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	p.Register(breed1)

	breed2 := &BreedConfig{
		ID:               "breed1",
		Name:             "updated",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	if err := p.Register(breed2); err != nil {
		t.Fatalf("overwriting user breed should succeed: %v", err)
	}
	got := p.GetBreed("breed1")
	if got.Name != "updated" {
		t.Errorf("Name = %q, want %q", got.Name, "updated")
	}
}

func TestRegisterSystemOverwriteSystem(t *testing.T) {
	p := New("test")
	breed1 := &BreedConfig{
		ID:               "sys1",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceSystem,
	}
	p.Register(breed1)

	breed2 := &BreedConfig{
		ID:               "sys1",
		Name:             "updated",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceSystem,
	}
	if err := p.Register(breed2); err != nil {
		t.Fatalf("system overwriting system should succeed: %v", err)
	}
}

func TestValidateSystemBreedProtection(t *testing.T) {
	p := New("test")
	systemBreed := &BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceSystem,
	}
	p.Register(systemBreed)

	userBreed := &BreedConfig{
		ID:               "sys-breed",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	err := p.Validate(userBreed)
	if err == nil {
		t.Error("expected error validating user overwrite of system breed")
	}
}

func TestCloseNoCapabilities(t *testing.T) {
	p := New("test")
	if err := p.Close(); err != nil {
		t.Fatalf("Close with no capabilities: %v", err)
	}
}

func TestRegisterCapabilityInitFailure(t *testing.T) {
	p := New("test")
	failCap := &initFailingCapability{name: "bad", version: "v1"}
	err := p.RegisterCapability(failCap)
	if err == nil {
		t.Fatal("expected error for init failure, got nil")
	}
}

// initFailingCapability is a mock whose Init always returns an error.
type initFailingCapability struct {
	name    string
	version string
}

func (m *initFailingCapability) Name() string                   { return m.name }
func (m *initFailingCapability) Version() string                { return m.version }
func (m *initFailingCapability) Init(ctx context.Context) error { return context.DeadlineExceeded }
func (m *initFailingCapability) Run(ctx context.Context, input *TaskInput) (*TaskOutput, error) {
	return &TaskOutput{Approved: true}, nil
}
func (m *initFailingCapability) Health() error { return nil }
func (m *initFailingCapability) Close() error  { return nil }

// --- Loader edge cases ---

func TestLoadFromDirSkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	p := New("test")

	// Create a subdirectory with a .json file — should be skipped
	subdir := filepath.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.json"), []byte(`{invalid}`), 0644)

	// Write a valid breed file at top level
	validJSON := `{"id": "good", "name": "good", "source": "user"}`
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(validJSON), 0644)

	if err := p.LoadFromDir(dir, LoadPolicyFailFast); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(p.List()) != 1 {
		t.Errorf("List len = %d, want 1", len(p.List()))
	}
}

func TestLoadFromDirEmptyJSONObject(t *testing.T) {
	dir := t.TempDir()
	p := New("test")

	// Empty JSON object {} is valid JSON, should load as empty BreedConfig
	os.WriteFile(filepath.Join(dir, "empty.json"), []byte(`{}`), 0644)

	if err := p.LoadFromDir(dir, LoadPolicyFailFast); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(p.List()) != 1 {
		t.Errorf("List len = %d, want 1", len(p.List()))
	}
}

func TestLoadFromDirMultipleValidFiles(t *testing.T) {
	dir := t.TempDir()
	p := New("test")

	for i := 0; i < 5; i++ {
		json := `{"id": "breed-` + string(rune('a'+i)) + `", "source": "user"}`
		filename := filepath.Join(dir, "breed-"+string(rune('a'+i))+".json")
		os.WriteFile(filename, []byte(json), 0644)
	}

	if err := p.LoadFromDir(dir, LoadPolicyFailFast); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(p.List()) != 5 {
		t.Errorf("List len = %d, want 5", len(p.List()))
	}
}

func TestLoadFromDirSkipInvalidAllBad(t *testing.T) {
	dir := t.TempDir()
	p := New("test")

	os.WriteFile(filepath.Join(dir, "bad1.json"), []byte(`{invalid1}`), 0644)
	os.WriteFile(filepath.Join(dir, "bad2.json"), []byte(`{invalid2}`), 0644)

	if err := p.LoadFromDir(dir, LoadPolicySkipInvalid); err != nil {
		t.Fatalf("LoadFromDir SkipInvalid: %v", err)
	}
	if len(p.List()) != 0 {
		t.Errorf("List len = %d, want 0", len(p.List()))
	}
}

// --- TaskOutput / TaskInput edge cases ---

func TestTaskOutputControlField(t *testing.T) {
	out := TaskOutput{Control: true, Approved: true, Reason: "control step"}
	if !out.Control {
		t.Error("Control should be true")
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TaskOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.Control {
		t.Error("decoded Control should be true")
	}
}

func TestTaskInputBreedAndSink(t *testing.T) {
	breed := &BreedConfig{ID: "test"}
	input := TaskInput{
		Query: "test",
		Breed: breed,
	}
	if input.Breed == nil || input.Breed.ID != "test" {
		t.Error("Breed not set correctly")
	}
	// Sink should be nil by default
	if input.Sink != nil {
		t.Error("Sink should default to nil")
	}
}

// --- Bark edge cases ---

func TestBarkOutputStructure(t *testing.T) {
	p := New("test")
	breed := &BreedConfig{
		ID:               "breed1",
		DefaultVariantID: "v1",
		Variants:         []Variant{{ID: "v1", ClientID: "c", DefaultModel: "m"}},
		Source:           BreedSourceUser,
	}
	p.Register(breed)

	out, err := p.Bark(context.Background(), "breed1", &TaskInput{Query: "test"})
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}
	if !out.Approved {
		t.Error("Approved should be true")
	}
	if out.Data == nil {
		t.Error("Data should not be nil")
	}
	if _, ok := out.Data["steps"]; !ok {
		t.Error("Data should contain 'steps' key")
	}
}

// --- ExecuteDispatch edge cases ---

func TestExecuteDispatchNoEntries(t *testing.T) {
	p := New("test")
	plan := orchestrator.DispatchPlan{MaxDepth: 3, Total: 0}
	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results len = %d, want 0", len(results))
	}
	if len(entryErrors) != 0 {
		t.Errorf("entryErrors len = %d, want 0", len(entryErrors))
	}
}

func TestExecuteDispatchAllNonPending(t *testing.T) {
	p := New("test")
	registerStubBreed(p, "b1")

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "b1", SubTaskID: "sub-1", Status: "skipped"},
			{BreedID: "b1", SubTaskID: "sub-2", Status: "completed"},
		},
		MaxDepth: 3,
		Total:    2,
	}
	subtasks := []orchestrator.SubTask{
		{ID: "sub-1", Description: "one"},
		{ID: "sub-2", Description: "two"},
	}
	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, subtasks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results len = %d, want 0 (all non-pending)", len(results))
	}
	if len(entryErrors) != 0 {
		t.Errorf("entryErrors len = %d, want 0", len(entryErrors))
	}
}

func TestExecuteDispatchEmptySubtasks(t *testing.T) {
	p := New("test")
	registerStubBreed(p, "b1")

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "b1", SubTaskID: "sub-1", Status: "pending"},
		},
		MaxDepth: 3,
		Total:    1,
	}
	// Empty subtasks — the entry's SubTaskID won't be found, but Bark
	// should still execute since the breed is registered.
	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, []orchestrator.SubTask{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// The breed is registered, so Bark should succeed even with empty subtasks
	if len(entryErrors) != 0 {
		t.Errorf("entryErrors len = %d, want 0", len(entryErrors))
	}
	if len(results) != 1 {
		t.Errorf("results len = %d, want 1", len(results))
	}
}

func TestExecuteDispatchDepOnNonPending(t *testing.T) {
	p := New("test")
	registerStubBreed(p, "b1")

	plan := orchestrator.DispatchPlan{
		Entries: []orchestrator.DispatchEntry{
			{BreedID: "b1", SubTaskID: "sub-1", Status: "completed"},
			{BreedID: "b1", SubTaskID: "sub-2", Status: "pending", DependsOn: []string{"sub-1"}},
		},
		MaxDepth: 3,
		Total:    2,
	}
	subtasks := []orchestrator.SubTask{
		{ID: "sub-1", Description: "one"},
		{ID: "sub-2", Description: "two"},
	}
	results, entryErrors, err := p.ExecuteDispatch(context.Background(), plan, subtasks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results len = %d, want 1 (only pending)", len(results))
	}
	if _, ok := results["sub-2"]; !ok {
		t.Error("missing sub-2 result")
	}
	if len(entryErrors) != 0 {
		t.Errorf("entryErrors len = %d, want 0", len(entryErrors))
	}
}
