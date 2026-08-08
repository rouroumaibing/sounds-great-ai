package pack

import (
	"context"
	"fmt"
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

// Close 关闭所有 capability 资源
func (p *Pack) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, cap := range p.capabilities {
		cap.Close()
	}
	return nil
}
