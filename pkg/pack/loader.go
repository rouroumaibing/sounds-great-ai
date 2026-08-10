package pack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadPolicy 加载策略
type LoadPolicy int

const (
	LoadPolicyFailFast    LoadPolicy = 0 // 遇到无效 JSON 立即返回错误
	LoadPolicySkipInvalid LoadPolicy = 1 // 跳过无效文件，继续加载
)

// LoadFromDir 从目录加载所有 .json breed 配置文件
func (p *Pack) LoadFromDir(dir string, policy LoadPolicy) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			if policy == LoadPolicySkipInvalid {
				continue
			}
			return fmt.Errorf("read file %q: %w", path, err)
		}
		var breed BreedConfig
		if err := json.Unmarshal(data, &breed); err != nil {
			if policy == LoadPolicySkipInvalid {
				continue
			}
			return fmt.Errorf("parse file %q: %w", path, err)
		}
		if err := p.Register(&breed); err != nil {
			if policy == LoadPolicySkipInvalid {
				continue
			}
			return fmt.Errorf("register breed from %q: %w", path, err)
		}
	}
	return nil
}

// LoadFromFile reads a single consolidated template file (dog-template.json) and
// registers every breed found in its "breeds" array.
func (p *Pack) LoadFromFile(path string, policy LoadPolicy) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %q: %w", path, err)
	}
	var tmpl DogTemplateFile
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return fmt.Errorf("parse file %q: %w", path, err)
	}
	for i := range tmpl.Breeds {
		if err := p.Register(&tmpl.Breeds[i]); err != nil {
			if policy == LoadPolicySkipInvalid {
				continue
			}
			return fmt.Errorf("register breed from %q: %w", path, err)
		}
	}
	return nil
}
