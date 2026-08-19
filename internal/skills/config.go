package skills

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// SkillIntent 是单个技能在 skills-config.json 中的持久化意图（启用/作用域/挂载点）。
type SkillIntent struct {
	Enabled      bool     `json:"enabled"`
	Scope        string   `json:"scope"`       // global | project
	MountPoints  []string `json:"mountPoints"` // carrier 列表：claude/codex/gemini/opencode/kimi
	Source       string   `json:"source"`      // packs | user | plugin
	PluginID     string   `json:"pluginId"`
	SkillsSource string   `json:"skillsSource"`
}

// SkillSyncState 记录最近一次调谐（sync）的结果，作为漂移检测的基线。
type SkillSyncState struct {
	SourceManifestHash string `json:"sourceManifestHash"`
	LastSyncedAt       string `json:"lastSyncedAt"` // ISO-8601
}

// SkillConfig 是 skills-config.json 的根结构。
type SkillConfig struct {
	Version int                      `json:"version"`
	Skills  map[string]*SkillIntent `json:"skills"`
	Sync    SkillSyncState          `json:"sync"`
}

// SkillConfigStore 管理 skills-config.json 的原子读写与可选热加载。
// path 为空表示仅内存态（不落盘，Save 为 no-op）。
type SkillConfigStore struct {
	mu   sync.RWMutex
	path string
	data *SkillConfig
	stop chan struct{}
}

func NewSkillConfigStore(path string) *SkillConfigStore {
	return &SkillConfigStore{
		path: path,
		data: &SkillConfig{Version: 1, Skills: map[string]*SkillIntent{}},
	}
}

// Load 从磁盘加载技能意图（文件不存在时初始化为空配置）。
func (s *SkillConfigStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *SkillConfigStore) loadLocked() error {
	if s.path == "" {
		return nil
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = &SkillConfig{Version: 1, Skills: map[string]*SkillIntent{}}
			return nil
		}
		return err
	}
	cfg := &SkillConfig{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return err
	}
	if cfg.Skills == nil {
		cfg.Skills = map[string]*SkillIntent{}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	s.data = cfg
	return nil
}

// Save 原子写回磁盘（临时文件 + rename，权限 0644，对齐 settings.writeAtomic）。
func (s *SkillConfigStore) Save() error {
	s.mu.RLock()
	data := s.data
	path := s.path
	s.mu.RUnlock()
	if path == "" {
		return nil // 内存态，不落盘
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// GetIntent 返回某技能的意图（可能为 nil）。
func (s *SkillConfigStore) GetIntent(id string) *SkillIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Skills[id]
}

// AllIntents 返回所有意图的快照拷贝。
func (s *SkillConfigStore) AllIntents() map[string]*SkillIntent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*SkillIntent, len(s.data.Skills))
	for k, v := range s.data.Skills {
		out[k] = v
	}
	return out
}

// SetIntent 直接覆盖某技能的意图。
func (s *SkillConfigStore) SetIntent(id string, it *SkillIntent) error {
	s.mu.Lock()
	s.data.Skills[id] = it
	s.mu.Unlock()
	return s.Save()
}

// Enabled 返回某技能是否启用（无意图记录时视为禁用）。
func (s *SkillConfigStore) Enabled(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if it, ok := s.data.Skills[id]; ok {
		return it.Enabled
	}
	return false
}

// SetEnabled 持久化某技能的启用状态。scope 仅在首次设置意图时填充。
func (s *SkillConfigStore) SetEnabled(id string, enabled bool, scope string) error {
	s.mu.Lock()
	it, ok := s.data.Skills[id]
	if !ok {
		it = &SkillIntent{}
		s.data.Skills[id] = it
	}
	it.Enabled = enabled
	if it.Scope == "" {
		it.Scope = scope
	}
	s.mu.Unlock()
	return s.Save()
}

// SetMountPoints 持久化某技能的挂载点（carrier 列表）。
func (s *SkillConfigStore) SetMountPoints(id string, mps []string) error {
	s.mu.Lock()
	it, ok := s.data.Skills[id]
	if !ok {
		it = &SkillIntent{}
		s.data.Skills[id] = it
	}
	it.MountPoints = mps
	s.mu.Unlock()
	return s.Save()
}

// SetSyncState 记录最近一次调谐结果。
func (s *SkillConfigStore) SetSyncState(hash, at string) error {
	s.mu.Lock()
	s.data.Sync.SourceManifestHash = hash
	s.data.Sync.LastSyncedAt = at
	s.mu.Unlock()
	return s.Save()
}

// Watch 启动后台轮询：每 3s 检查文件 mtime，检测到变更后 30s 防抖再触发
// onReload（外部进程编辑 skills-config.json 时刷新内存态，对齐 settings.HotReloader）。
func (s *SkillConfigStore) Watch(onReload func()) {
	s.mu.Lock()
	if s.stop != nil {
		s.mu.Unlock()
		return
	}
	s.stop = make(chan struct{})
	stop := s.stop
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		var pendingAt time.Time
		last := modTime(s.path)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mt := modTime(s.path)
				if !mt.Equal(last) {
					last = mt
					pendingAt = time.Now().Add(30 * time.Second)
				}
				if !pendingAt.IsZero() && !time.Now().Before(pendingAt) {
					if onReload != nil {
						onReload()
					}
					pendingAt = time.Time{}
				}
			}
		}
	}()
}

// Stop 终止后台轮询。
func (s *SkillConfigStore) Stop() {
	s.mu.Lock()
	stop := s.stop
	s.stop = nil
	s.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func modTime(p string) time.Time {
	info, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
