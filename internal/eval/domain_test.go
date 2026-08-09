package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDomainsParsesYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "eval-a2a.yaml"), `
domainId: eval:a2a
displayName: "A2A 协作评估"
descriptionForHuman: "评估狗狗间协作质量"
evalBreed: bianmu
frequency: daily
sourceAdapter: telemetry
threadId: thread_eval_a2a
sla:
  acknowledgeHours: 24
  reevalWithinHours: 72
enabled: true
`)
	domains, err := LoadDomains(dir)
	if err != nil {
		t.Fatalf("LoadDomains: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}
	d := domains[0]
	if d.DomainID != "eval:a2a" {
		t.Errorf("DomainID = %q", d.DomainID)
	}
	if d.EvalBreed != "bianmu" {
		t.Errorf("EvalBreed = %q", d.EvalBreed)
	}
	if d.SLA.AcknowledgeHours != 24 {
		t.Errorf("AcknowledgeHours = %d", d.SLA.AcknowledgeHours)
	}
	if !d.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestLoadDomainsSkipsDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "eval-disabled.yaml"), `
domainId: eval:disabled
displayName: "Disabled"
descriptionForHuman: "test"
evalBreed: bianmu
frequency: daily
enabled: false
`)
	domains, err := LoadDomains(dir)
	if err != nil {
		t.Fatalf("LoadDomains: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("expected 0 enabled domains, got %d", len(domains))
	}
}

func TestLoadDomainsDefaultFrequency(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "eval-no-freq.yaml"), `
domainId: eval:nofreq
displayName: "No Freq"
descriptionForHuman: "test"
evalBreed: demu
enabled: true
`)
	domains, _ := LoadDomains(dir)
	if domains[0].Frequency != "daily" {
		t.Errorf("default Frequency = %q, want daily", domains[0].Frequency)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
