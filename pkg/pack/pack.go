package pack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Pack 协调者，管理 breed 注册表和 capability
type Pack struct {
	name         string
	registry     map[string]*BreedConfig
	capabilities map[string]Capability
	mu           sync.RWMutex
}

// New 创建一个新的 Pack
func New(name string) *Pack {
	return &Pack{
		name:         name,
		registry:     make(map[string]*BreedConfig),
		capabilities: make(map[string]Capability),
	}
}

// RegisterCapability 注册并初始化一个 capability
func (p *Pack) RegisterCapability(cap Capability) error {
	if err := cap.Init(context.Background()); err != nil {
		return fmt.Errorf("capability %q init failed: %w", cap.Name(), err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key := fmt.Sprintf("%s:%s", cap.Name(), cap.Version())
	if _, exists := p.capabilities[key]; exists {
		cap.Close()
		return fmt.Errorf("capability %q already registered", key)
	}
	p.capabilities[key] = cap
	return nil
}

// Register 注册一个 breed 配置，校验 source 保护
func (p *Pack) Register(breed *BreedConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// system breed 不可被 user/plugin 覆盖
	if existing, ok := p.registry[breed.ID]; ok {
		if existing.Source == BreedSourceSystem && breed.Source != BreedSourceSystem {
			return fmt.Errorf("system breed %q cannot be overwritten", breed.ID)
		}
	}

	p.registry[breed.ID] = breed
	return nil
}

// Unregister 删除一个 breed，system breed 不可删除
func (p *Pack) Unregister(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	breed, ok := p.registry[id]
	if !ok {
		return fmt.Errorf("breed %q not found", id)
	}
	if breed.Source == BreedSourceSystem {
		return fmt.Errorf("system breed %q cannot be deleted", id)
	}
	delete(p.registry, id)
	return nil
}

// Validate 校验 breed 配置但不修改状态
func (p *Pack) Validate(breed *BreedConfig) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// system breed 保护检查
	if existing, ok := p.registry[breed.ID]; ok {
		if existing.Source == BreedSourceSystem && breed.Source != BreedSourceSystem {
			return fmt.Errorf("system breed %q cannot be overwritten", breed.ID)
		}
	}

	return nil
}

// List 返回所有已注册的 breed
func (p *Pack) List() []*BreedConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*BreedConfig, 0, len(p.registry))
	for _, b := range p.registry {
		result = append(result, b)
	}
	return result
}

// HasBreed 检查 breed 是否已注册
func (p *Pack) HasBreed(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.registry[id]
	return ok
}

// GetBreed 返回指定 breed 配置
func (p *Pack) GetBreed(id string) *BreedConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.registry[id]
}

// ReloadFromDir re-reads all breed configs from dir, replacing the registry.
// System breeds are preserved; user/plugin breeds are refreshed from disk.
func (p *Pack) ReloadFromDir(dir string, policy LoadPolicy) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", dir, err)
	}

	newRegistry := make(map[string]*BreedConfig)
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
		newRegistry[breed.ID] = &breed
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// preserve system breeds that aren't on disk
	for id, b := range p.registry {
		if b.Source == BreedSourceSystem {
			if _, ok := newRegistry[id]; !ok {
				newRegistry[id] = b
			}
		}
	}
	p.registry = newRegistry
	return nil
}

// ReloadFromFile re-reads the single consolidated template file, replacing the
// registry. System breeds not present on disk are preserved.
func (p *Pack) ReloadFromFile(path string, policy LoadPolicy) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %q: %w", path, err)
	}
	var tmpl DogTemplateFile
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return fmt.Errorf("parse file %q: %w", path, err)
	}

	newRegistry := make(map[string]*BreedConfig)
	for i := range tmpl.Breeds {
		newRegistry[tmpl.Breeds[i].ID] = &tmpl.Breeds[i]
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// preserve system breeds that aren't on disk
	for id, b := range p.registry {
		if b.Source == BreedSourceSystem {
			if _, ok := newRegistry[id]; !ok {
				newRegistry[id] = b
			}
		}
	}
	p.registry = newRegistry
	return nil
}

// Close 关闭所有 capability 资源
func (p *Pack) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, cap := range p.capabilities {
		cap.Close()
	}
	return nil
}
