package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromDirValid(t *testing.T) {
	dir := t.TempDir()

	// Register capabilities first
	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	// Write a valid breed file
	breedJSON := `{
		"id": "test-breed",
		"name": "test",
		"capabilities": [{"name": "cap1", "version": "v1"}],
		"workflow": {"steps": [{"id": "s1", "capability_ref": "cap1:v1"}]},
		"source": "user"
	}`
	os.WriteFile(filepath.Join(dir, "test-breed.json"), []byte(breedJSON), 0644)

	if err := p.LoadFromDir(dir, LoadPolicyFailFast); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(p.List()) != 1 {
		t.Errorf("List len = %d, want 1", len(p.List()))
	}
}

func TestLoadFromDirFailFast(t *testing.T) {
	dir := t.TempDir()

	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	// Write an invalid JSON file
	invalidJSON := `{invalid json}`
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte(invalidJSON), 0644)

	err := p.LoadFromDir(dir, LoadPolicyFailFast)
	if err == nil {
		t.Fatal("expected error for invalid JSON with FailFast, got nil")
	}
}

func TestLoadFromDirSkipInvalid(t *testing.T) {
	dir := t.TempDir()

	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	// Write an invalid JSON file
	invalidJSON := `{invalid json}`
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte(invalidJSON), 0644)

	// Write a valid breed file
	validJSON := `{
		"id": "good-breed",
		"name": "good",
		"capabilities": [{"name": "cap1", "version": "v1"}],
		"workflow": {"steps": [{"id": "s1", "capability_ref": "cap1:v1"}]},
		"source": "user"
	}`
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(validJSON), 0644)

	if err := p.LoadFromDir(dir, LoadPolicySkipInvalid); err != nil {
		t.Fatalf("LoadFromDir SkipInvalid: %v", err)
	}
	list := p.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1 (bad file skipped)", len(list))
	}
	if list[0].ID != "good-breed" {
		t.Errorf("List[0].ID = %q, want %q", list[0].ID, "good-breed")
	}
}

func TestLoadFromDirEmptyDir(t *testing.T) {
	dir := t.TempDir()
	p := New("test")

	if err := p.LoadFromDir(dir, LoadPolicyFailFast); err != nil {
		t.Fatalf("LoadFromDir empty: %v", err)
	}
	if len(p.List()) != 0 {
		t.Errorf("List len = %d, want 0", len(p.List()))
	}
}

func TestLoadFromDirNonExistentDir(t *testing.T) {
	p := New("test")
	err := p.LoadFromDir("/nonexistent/path/that/does/not/exist", LoadPolicyFailFast)
	if err == nil {
		t.Fatal("expected error for nonexistent dir, got nil")
	}
}

func TestLoadFromDirSkipsNonJSONFiles(t *testing.T) {
	dir := t.TempDir()

	p := New("test")
	p.RegisterCapability(&mockCapability{name: "cap1", version: "v1"})

	// Write a non-JSON file
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0644)

	// Write a valid breed file
	validJSON := `{
		"id": "good-breed",
		"name": "good",
		"capabilities": [{"name": "cap1", "version": "v1"}],
		"workflow": {"steps": [{"id": "s1", "capability_ref": "cap1:v1"}]},
		"source": "user"
	}`
	os.WriteFile(filepath.Join(dir, "good.json"), []byte(validJSON), 0644)

	if err := p.LoadFromDir(dir, LoadPolicyFailFast); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(p.List()) != 1 {
		t.Errorf("List len = %d, want 1", len(p.List()))
	}
}
