package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// sourceRoot 描述一个 skill 源目录及其来源类别。
type sourceRoot struct {
	dir    string
	source string // packs | user | plugin
}

// KnownCarriers 是 SG 协调的外部 CLI agent 列表（ValidCLIClientIDs）。
// 其中 claude/codex/gemini/kimi 拥有原生 skills 目录，可物理挂载（符号链接）；
// opencode 无统一原生目录，仅逻辑挂载（启用即注入 prompt）。
var KnownCarriers = []string{"claude", "codex", "gemini", "opencode", "kimi"}

// SkillManager 加载、索引 skill 源，并持有技能启用/挂载意图（持久化于
// skills-config.json）。意图分两层：global（home）为基线与权威禁用层，project
// （workspace）为工作区级覆盖。两层合并后驱动注入与物理挂载，实现「全局基线 +
// 工作区覆盖」的可移植治理级联。
type SkillManager struct {
	mu       sync.RWMutex
	sources  []sourceRoot
	skills   map[string]*Skill
	config   *SkillConfigStore   // global（home）意图存储
	project  *SkillConfigStore   // project（workspace）意图存储，可为 nil
	security *SkillSecurityStore // 技能安全状态存储（内外源权限隔离）
}

// NewManager 构造仅扫描单个目录（packs 源）的 manager，不持久化意图。
// 保留用于测试与向后兼容；生产路径应使用 NewManagerWithConfig。
func NewManager(dir string) *SkillManager {
	return NewManagerWithConfig("", "", map[string]string{dir: "packs"})
}

// NewManagerWithConfig 构造带持久化意图（skills-config.json）的 manager。
// globalConfigPath 为 home 级配置（基线 + 权威禁用），projectConfigPath 为
// 工作区覆盖配置（可为空，单配置模式）。sourceDirs 的 key 为目录路径，value 为
// 来源类别（packs|user|plugin）。configPath 全空表示仅内存态、不读写磁盘意图。
func NewManagerWithConfig(globalConfigPath, projectConfigPath string, sourceDirs map[string]string) *SkillManager {
	m := &SkillManager{
		skills: make(map[string]*Skill),
		config: NewSkillConfigStore(globalConfigPath),
	}
	if projectConfigPath != "" {
		m.project = NewSkillConfigStore(projectConfigPath)
	}
	secPath := ""
	if globalConfigPath != "" {
		secPath = filepath.Join(filepath.Dir(globalConfigPath), "skill-security.json")
	}
	m.security = NewSkillSecurityStore(secPath)
	_ = m.security.Load() // 内存态（无路径）自动 no-op
	for dir, src := range sourceDirs {
		if dir == "" {
			continue
		}
		m.sources = append(m.sources, sourceRoot{dir: dir, source: src})
	}
	return m
}

// Config 返回 global（home）持久化意图存储（永不为 nil）。
func (m *SkillManager) Config() *SkillConfigStore { return m.config }

// Security 返回技能安全状态存储（永不为 nil，无路径时为内存态）。
func (m *SkillManager) Security() *SkillSecurityStore { return m.security }

// LoadFromDir 扫描所有源目录加载 skill（向后兼容别名，委托 Scan）。
func (m *SkillManager) LoadFromDir() error { return m.Scan() }

// Scan 读取所有配置源目录，将发现的 skill 载入内存索引，并触发安全对账。
// 源目录不存在时静默跳过（例如未配置的用户级源）。
func (m *SkillManager) Scan() error {
	m.mu.Lock()
	loaded := make(map[string]*Skill)
	for _, sr := range m.sources {
		entries, err := os.ReadDir(sr.dir)
		if err != nil {
			// 单源场景保持原 NewManager(dir) 语义：目录缺失即报错；
			// 多源场景（如可选的用户级源）静默跳过。
			if len(m.sources) == 1 {
				m.mu.Unlock()
				return err
			}
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				skillPath := filepath.Join(sr.dir, entry.Name(), "SKILL.md")
				data, err := os.ReadFile(skillPath)
				if err != nil {
					continue
				}
				skill, err := ParseSkill(skillPath, data)
				if err != nil || skill.ID == "" {
					continue
				}
				skill.Source = sr.source
				loaded[skill.ID] = skill
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(sr.dir, entry.Name()))
			if err != nil {
				continue
			}
			skill, err := ParseSkill(entry.Name(), data)
			if err != nil || skill.ID == "" {
				continue
			}
			skill.Source = sr.source
			loaded[skill.ID] = skill
		}
	}
	m.skills = loaded
	m.mu.Unlock()

	// 源加载完成后对账安全状态（不在持锁期间执行，避免嵌套锁）。
	m.reconcileSecurity()
	return nil
}

// reconcileSecurity 对每个加载的 skill 分类来源并维护安全状态：内部可信源
// （packs）默认 approved 且内容变更自动续期；外部不可信源（user/plugin）首次见到
// 为 pending（注入前需人工批准），内容指纹变更后强制 quarantine（防上游替换下毒）。
func (m *SkillManager) reconcileSecurity() {
	if m.security == nil {
		return
	}
	m.mu.RLock()
	skills := make(map[string]*Skill, len(m.skills))
	for k, v := range m.skills {
		skills[k] = v
	}
	m.mu.RUnlock()

	for id, s := range skills {
		trusted := s.Source == "packs"
		fp := skillFingerprint(s.Body)
		st := m.security.StateOf(id)
		if st == nil {
			if trusted {
				_ = m.security.SetState(id, &SkillSecurityState{
					ID: id, Source: s.Source, Trusted: true, Fingerprint: fp, Status: SecurityApproved,
				})
			} else {
				_ = m.security.SetState(id, &SkillSecurityState{
					ID: id, Source: s.Source, Trusted: false, Fingerprint: fp, Status: SecurityPending,
				})
			}
			continue
		}
		if st.Fingerprint != fp {
			if trusted {
				st.Fingerprint = fp // 内部源自动续期
				_ = m.security.SetState(id, st)
			} else {
				st.Fingerprint = fp
				st.Status = SecurityQuarantined // 外部源内容变更后强制复审
				_ = m.security.SetState(id, st)
			}
		}
	}
	_ = m.security.Save()
}

// securityBlocks 报告某 skill 当前安全状态是否阻断注入（pending/quarantined/revoked）。
// 否则（approved 或无状态）不阻断。
func (m *SkillManager) securityBlocks(id string) bool {
	if m.security == nil {
		return false
	}
	st := m.security.StateOf(id)
	if st == nil {
		return false
	}
	return st.Status == SecurityPending || st.Status == SecurityQuarantined || st.Status == SecurityRevoked
}

func (m *SkillManager) Get(id string) *Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.skills[id]
}

// All 返回所有已加载的 skill（不区分启用状态）。
func (m *SkillManager) All() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Skill, 0, len(m.skills))
	for _, s := range m.skills {
		out = append(out, s)
	}
	return out
}

// AllEnabled 返回已加载、合并意图启用且未被安全层阻断的 skill。
func (m *SkillManager) AllEnabled() []*Skill {
	m.mu.RLock()
	ids := make([]string, 0, len(m.skills))
	for id := range m.skills {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	out := make([]*Skill, 0)
	for _, id := range ids {
		if !m.effectiveEnabled(id) || m.securityBlocks(id) {
			continue
		}
		if s := m.Get(id); s != nil {
			out = append(out, s)
		}
	}
	return out
}

// effectiveEnabled 按合并规则判定某 skill 是否生效：全局缺失视为默认启用，项目层
// 可覆盖；全局显式禁用为权威（覆盖项目启用）。
func (m *SkillManager) effectiveEnabled(id string) bool {
	it := mergeIntent(m.config.GetIntent(id), m.optionalProjectIntent(id))
	return it != nil && it.Enabled
}

// optionalProjectIntent 返回 project 层意图（无 project 存储时为 nil）。
func (m *SkillManager) optionalProjectIntent(id string) *SkillIntent {
	if m.project == nil {
		return nil
	}
	return m.project.GetIntent(id)
}

// MergedIntents 返回合并后的生效意图快照（global + project 两层）。
func (m *SkillManager) MergedIntents() map[string]*SkillIntent {
	out := make(map[string]*SkillIntent)
	if m.config != nil {
		for id, g := range m.config.AllIntents() {
			out[id] = mergeIntent(g, m.optionalProjectIntent(id))
		}
	}
	if m.project != nil {
		for id, p := range m.project.AllIntents() {
			out[id] = mergeIntent(m.config.GetIntent(id), p)
		}
	}
	return out
}

// mergeIntent 合并 global（g）与 project（p）意图：g 缺失默认启用，p 缺失继承；
// g 显式禁用为权威。scope 取 project（若存在）否则 global。
func mergeIntent(g, p *SkillIntent) *SkillIntent {
	if g == nil && p == nil {
		return nil
	}
	gE := g == nil || g.Enabled // 全局缺失 = 默认启用
	pE := p == nil || p.Enabled // 项目缺失 = 继承
	eff := &SkillIntent{Enabled: gE && pE}
	if p != nil {
		eff.Scope = "project"
		eff.MountPoints = p.MountPoints
		eff.Source = p.Source
		eff.PluginID = p.PluginID
		eff.SkillsSource = p.SkillsSource
	} else if g != nil {
		eff.Scope = g.Scope
		eff.MountPoints = g.MountPoints
		eff.Source = g.Source
		eff.PluginID = g.PluginID
		eff.SkillsSource = g.SkillsSource
	}
	return eff
}

// IntentOf 返回某 skill 的合并生效意图（可能为 nil）。
func (m *SkillManager) IntentOf(id string) *SkillIntent {
	return mergeIntent(m.config.GetIntent(id), m.optionalProjectIntent(id))
}

// SetEnabled 持久化某 skill 的启用状态。scope=global 写全局层，否则写项目层
// （项目层存在时），实现工作区覆盖。
func (m *SkillManager) SetEnabled(id string, enabled bool, scope string) error {
	if m.config == nil {
		return fmt.Errorf("skill manager has no config store")
	}
	store := m.config
	if scope != "global" && m.project != nil {
		store = m.project
	}
	return store.SetEnabled(id, enabled, scope)
}

// SetMountPoints 持久化某 skill 的挂载点（carrier 列表）。写入当前已持有该意图
// 的存储层，避免仅在项目层写挂载点误触禁用。
func (m *SkillManager) SetMountPoints(id string, mps []string) error {
	if m.config == nil {
		return fmt.Errorf("skill manager has no config store")
	}
	store := m.config
	if m.project != nil && m.project.GetIntent(id) != nil {
		store = m.project
	}
	return store.SetMountPoints(id, mps)
}

// Resolve 按显式 ID 列表返回已加载且未被安全层阻断的 skill（用于 prompt builder 注入）。
func (m *SkillManager) Resolve(ids []string) []*Skill {
	if len(ids) == 0 {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Skill, 0, len(ids))
	for _, id := range ids {
		if m.securityBlocks(id) {
			continue
		}
		if s, ok := m.skills[id]; ok {
			out = append(out, s)
		}
	}
	return out
}

// EnabledForCarrier 返回当前 carrier 下已生效且挂载到该 carrier 且未被安全层
// 阻断的 skill ID 集合。供 Build 集成注入 SkillIDs。
func (m *SkillManager) EnabledForCarrier(carrier string) []string {
	m.mu.RLock()
	ids := make([]string, 0, len(m.skills))
	for id := range m.skills {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	out := make([]string, 0)
	for _, id := range ids {
		it := m.IntentOf(id)
		if !m.effectiveEnabled(id) || m.securityBlocks(id) {
			continue
		}
		if mountsToCarrier(it, carrier) {
			out = append(out, id)
		}
	}
	return out
}

// SecurityState 返回某 skill 的安全状态（可能为 nil）。
func (m *SkillManager) SecurityState(id string) *SkillSecurityState {
	if m.security == nil {
		return nil
	}
	return m.security.StateOf(id)
}

// AllSecurityStates 返回全部技能安全状态的快照。
func (m *SkillManager) AllSecurityStates() map[string]*SkillSecurityState {
	if m.security == nil {
		return map[string]*SkillSecurityState{}
	}
	return m.security.All()
}

// ApproveSkill 将某 skill 标记为已批准（owner-gate 风格的人工放行），仅 trusted
// 来源或经评审的外部来源可注入。
func (m *SkillManager) ApproveSkill(id, by string) error {
	if m.security == nil {
		return nil
	}
	s := m.Get(id)
	src := ""
	fp := ""
	if s != nil {
		src = s.Source
		fp = skillFingerprint(s.Body)
	}
	return m.security.SetState(id, &SkillSecurityState{
		ID: id, Source: src, Trusted: src == "packs", Fingerprint: fp,
		Status: SecurityApproved, ReviewedBy: by,
	})
}

// QuarantineSkill 将某 skill 隔离（阻断注入），用于内容可疑或指纹失配。
func (m *SkillManager) QuarantineSkill(id, by string) error {
	if m.security == nil {
		return nil
	}
	s := m.Get(id)
	src, fp := "", ""
	if s != nil {
		src = s.Source
		fp = skillFingerprint(s.Body)
	}
	return m.security.SetState(id, &SkillSecurityState{
		ID: id, Source: src, Trusted: src == "packs", Fingerprint: fp, Status: SecurityQuarantined, ReviewedBy: by,
	})
}

// RevokeSkill 撤销某 skill 的注入资格（长期禁用）。
func (m *SkillManager) RevokeSkill(id, by string) error {
	if m.security == nil {
		return nil
	}
	s := m.Get(id)
	src, fp := "", ""
	if s != nil {
		src = s.Source
		fp = skillFingerprint(s.Body)
	}
	return m.security.SetState(id, &SkillSecurityState{
		ID: id, Source: src, Trusted: src == "packs", Fingerprint: fp, Status: SecurityRevoked, ReviewedBy: by,
	})
}

// ReloadAll 重新加载 global + project 两层意图并重扫源（热加载回调入口）。
func (m *SkillManager) ReloadAll() error {
	if m.config != nil {
		_ = m.config.Load()
	}
	if m.project != nil {
		_ = m.project.Load()
	}
	return m.Scan()
}

// InjectSkills 将给定 skill 正文追加到系统提示词（向后兼容）。
func (m *SkillManager) InjectSkills(systemPrompt string, skills []*Skill) string {
	if len(skills) == 0 {
		return systemPrompt
	}
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n--- Skills ---\n")
	for _, s := range skills {
		sb.WriteString(s.Body)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// skillFingerprint 计算 skill 正文确定性指纹（sha256），用于内容变更检测。
func skillFingerprint(body string) string {
	h := sha256.Sum256([]byte(body))
	return hex.EncodeToString(h[:])
}
