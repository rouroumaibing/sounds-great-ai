package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryCredentialStore_CRUD(t *testing.T) {
	cs := NewMemoryCredentialStore()

	cs.Set("anthropic", "sk-ant-xxx")
	val, err := cs.Get("anthropic")
	if err != nil || val != "sk-ant-xxx" {
		t.Fatalf("Get after Set: got %q, %v; want sk-ant-xxx, nil", val, err)
	}

	if !cs.Has("anthropic") {
		t.Fatal("Has should be true")
	}
	if cs.Has("openai") {
		t.Fatal("Has should be false for unset")
	}

	cs.Delete("anthropic")
	if cs.Has("anthropic") {
		t.Fatal("Has should be false after Delete")
	}
	_, err = cs.Get("anthropic")
	if err == nil {
		t.Fatal("Get should error after Delete")
	}
}

func TestFileCredentialStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	cs := NewFileCredentialStore(path, false)

	cs.Set("anthropic", "sk-ant-xxx")
	cs.Set("openai", "sk-oai-yyy")

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 0600", info.Mode().Perm())
	}

	val, err := cs.Get("anthropic")
	if err != nil || val != "sk-ant-xxx" {
		t.Fatalf("Get: got %q, %v; want sk-ant-xxx", val, err)
	}

	cs2 := NewFileCredentialStore(path, false)
	val2, _ := cs2.Get("openai")
	if val2 != "sk-oai-yyy" {
		t.Fatalf("lazy load: got %q, want sk-oai-yyy", val2)
	}

	cs.Delete("anthropic")
	if cs.Has("anthropic") {
		t.Fatal("Has should be false after Delete")
	}
}
