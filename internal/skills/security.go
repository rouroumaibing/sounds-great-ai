package skills

import (
	"encoding/json"
	"os"
	"sync"
)

// SkillSecurityStatus 是技能安全状态机：
//   - pending：外部不可信源首次出现，注入前需人工批准（owner-gate 风格）
//   - approved：已批准，可注入
//   - quarantined：内容指纹失配或主动隔离，阻断注入
//   - revoked：长期撤销，阻断注入
type SkillSecurityStatus string

const (
	SecurityPending     SkillSecurityStatus = "pending"
	SecurityApproved    SkillSecurityStatus = "approved"
	SecurityQuarantined SkillSecurityStatus = "quarantined"
	SecurityRevoked     SkillSecurityStatus = "revoked"
)

// SkillSecurityState 是单个技能的安全状态（持久化于 skill-security.json）。
type SkillSecurityState struct {
	ID          string             `json:"id"`
	Source      string             `json:"source"`    // packs | user | plugin
	Trusted     bool               `json:"trusted"`   // 内部可信源（packs）为 true
	Fingerprint string             `json:"fingerprint"` // skill 正文 sha256
	Status      SkillSecurityStatus `json:"status"`
	ReviewedBy  string             `json:"reviewedBy,omitempty"`
}

// SkillSecurityStore 管理 skill-security.json 的原子读写。path 为空表示内存态
// （SetState/Save 为 no-op），用于测试与单配置内存场景。
type SkillSecurityStore struct {
	mu   sync.RWMutex
	path string
	data map[string]*SkillSecurityState
}

func NewSkillSecurityStore(path string) *SkillSecurityStore {
	return &SkillSecurityStore{
		path: path,
		data: map[string]*SkillSecurityState{},
	}
}

// Load 从磁盘加载安全状态（文件不存在时初始化为空）。
func (s *SkillSecurityStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = map[string]*SkillSecurityState{}
			return nil
		}
		return err
	}
	states := map[string]*SkillSecurityState{}
	if err := json.Unmarshal(raw, &states); err != nil {
		return err
	}
	if states == nil {
		states = map[string]*SkillSecurityState{}
	}
	s.data = states
	return nil
}

// Save 原子写回磁盘（临时文件 + rename，权限 0644）。
func (s *SkillSecurityStore) Save() error {
	s.mu.RLock()
	data := s.data
	path := s.path
	s.mu.RUnlock()
	if path == "" {
		return nil
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

// StateOf 返回某技能的安全状态（可能为 nil）。
func (s *SkillSecurityStore) StateOf(id string) *SkillSecurityState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[id]
}

// SetState 直接覆盖某技能的安全状态并落盘。
func (s *SkillSecurityStore) SetState(id string, st *SkillSecurityState) error {
	s.mu.Lock()
	s.data[id] = st
	s.mu.Unlock()
	return s.Save()
}

// Approved 报告某技能当前是否处于 approved（可注入）。
func (s *SkillSecurityStore) Approved(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.data[id]
	return ok && st.Status == SecurityApproved
}

// All 返回全部安全状态的快照拷贝。
func (s *SkillSecurityStore) All() map[string]*SkillSecurityState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*SkillSecurityState, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}
