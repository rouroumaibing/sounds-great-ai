package eval

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// EvalDomain defines an evaluation domain.
type EvalDomain struct {
	DomainID      string  `yaml:"domainId"`
	DisplayName   string  `yaml:"displayName"`
	Description   string  `yaml:"descriptionForHuman"`
	EvalBreed     string  `yaml:"evalBreed"`
	Frequency     string  `yaml:"frequency"`
	SourceAdapter string  `yaml:"sourceAdapter"`
	ThreadID      string  `yaml:"threadId"`
	SLA           EvalSLA `yaml:"sla"`
	Enabled       bool    `yaml:"enabled"`
}

// EvalSLA defines the service-level agreement for closure.
type EvalSLA struct {
	AcknowledgeHours  int `yaml:"acknowledgeHours"`
	ReevalWithinHours int `yaml:"reevalWithinHours"`
}

// LoadDomains reads all enabled eval domain YAML files from dir.
func LoadDomains(dir string) ([]EvalDomain, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var domains []EvalDomain
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var d EvalDomain
		if err := yaml.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		if d.Frequency == "" {
			d.Frequency = "daily"
		}
		if !d.Enabled {
			continue
		}
		domains = append(domains, d)
	}
	return domains, nil
}
