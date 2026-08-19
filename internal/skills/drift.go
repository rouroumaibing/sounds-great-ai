package skills

import (
	"os"
	"path/filepath"
)

// DriftType 是漂移问题的分类。
type DriftType string

const (
	DriftUnregistered  DriftType = "unregistered"   // 源有 skill，config 未启用
	DriftPhantom       DriftType = "phantom"        // config 启用，源已删
	DriftConflict      DriftType = "conflict"       // carrier skills 目录存在非 symlink 同名占用
	DriftMountMissing  DriftType = "mount-missing"  // enabled+carrier 但 symlink 缺失
	DriftStaleMount    DriftType = "stale-mount"    // symlink 存在但 config 已禁用/移除
	DriftConfigNew     DriftType = "config-new"     // 全局启用但项目层未收到（级联缺口）
	DriftConfigOrphan  DriftType = "config-orphan"  // 项目层引用源已删的 skill（幽灵启用）
)

// DriftIssue 描述一处技能挂载漂移。
type DriftIssue struct {
	SkillID string   `json:"skillId"`
	Type    DriftType `json:"type"`
	Carrier string   `json:"carrier,omitempty"`
	Detail  string   `json:"detail"`
}

// DetectSkillDrift 比较相邻边，产出漂移问题（三层四边模型 + 配置层两类）：
//   - unregistered：源有 skill，生效意图未启用
//   - phantom：生效意图启用，源已删
//   - conflict：carrier 原生 skills 目录存在非 symlink 同名占用（用户手动放）
//   - mount-missing：enabled+carrier 但 symlink 缺失
//   - stale-mount：symlink 存在但生效意图已禁用/移除
//   - config-new：全局启用但项目层无对应覆盖（级联未下发，仅当项目层已启用治理时）
//   - config-orphan：项目层引用了源已删除的 skill（幽灵启用）
func (m *SkillManager) DetectSkillDrift(opts SkillSyncOptions) []DriftIssue {
	m.mu.RLock()
	intents := m.MergedIntents()
	skills := make(map[string]*Skill, len(m.skills))
	for k, v := range m.skills {
		skills[k] = v
	}
	m.mu.RUnlock()

	var issues []DriftIssue

	// 源↔config 边：unregistered + phantom。
	for id, sk := range skills {
		it := intents[id]
		if it == nil || !it.Enabled {
			issues = append(issues, DriftIssue{
				SkillID: id, Type: DriftUnregistered,
				Detail: "源存在技能但未被启用（config 无启用意图）",
			})
			continue
		}
		// enabled：检查每个 carrier 的磁盘状态。
		for _, carrier := range carrierList(it) {
			if !carrierSupportsNativeSkillsDir(carrier) {
				continue // 逻辑挂载无磁盘态
			}
			link := filepath.Join(carrierNativeSkillsDir(carrier, it.Scope, opts.WorkspaceDir, opts.HomeDir), id)
			info, err := os.Lstat(link)
			if os.IsNotExist(err) {
				issues = append(issues, DriftIssue{
					SkillID: id, Type: DriftMountMissing, Carrier: carrier,
					Detail: "已启用并挂载到 " + carrier + "，但符号链接缺失",
				})
				continue
			}
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				issues = append(issues, DriftIssue{
					SkillID: id, Type: DriftConflict, Carrier: carrier,
					Detail: carrier + " skills 目录存在非 SG 管理的同名占用，需备份后解决",
				})
				continue
			}
			target, err := os.Readlink(link)
			if err != nil {
				continue
			}
			if !m.isManagedTarget(target) || filepath.Dir(target) != filepath.Dir(sk.FilePath) {
				issues = append(issues, DriftIssue{
					SkillID: id, Type: DriftStaleMount, Carrier: carrier,
					Detail: "符号链接指向非当前源，可能已过期",
				})
			}
		}
	}
	// config↔源 反向边：phantom（启用但源已删）。
	for id, it := range intents {
		if !it.Enabled {
			continue
		}
		if _, ok := skills[id]; !ok {
			issues = append(issues, DriftIssue{
				SkillID: id, Type: DriftPhantom,
				Detail: "生效意图标记为启用，但源目录已无此技能",
			})
		}
	}
	// symlink↔config 边：stale-mount（生效意图禁用/移除但链接仍在）。
	for id, it := range intents {
		enabled := it != nil && it.Enabled
		if enabled {
			continue
		}
		for _, carrier := range KnownCarriers {
			if !carrierSupportsNativeSkillsDir(carrier) {
				continue
			}
			link := filepath.Join(carrierNativeSkillsDir(carrier, it.Scope, opts.WorkspaceDir, opts.HomeDir), id)
			info, err := os.Lstat(link)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				issues = append(issues, DriftIssue{
					SkillID: id, Type: DriftStaleMount, Carrier: carrier,
					Detail: "生效意图已禁用/移除，但符号链接仍存在",
				})
			}
		}
	}
	// 配置层边（仅当存在 project 覆盖层）：config-new / config-orphan。
	if m.project != nil {
		// config-orphan：project 意图引用了源已删除的 skill。
		for id := range m.project.AllIntents() {
			if _, ok := skills[id]; !ok {
				issues = append(issues, DriftIssue{
					SkillID: id, Type: DriftConfigOrphan,
					Detail: "项目层启用了源已删除的技能（幽灵启用），建议清理项目覆盖",
				})
			}
		}
		// config-new：项目层已启用治理（存在任意覆盖），但全局启用而项目层未收到。
		if len(m.project.AllIntents()) > 0 {
			for id, g := range m.config.AllIntents() {
				if !g.Enabled {
					continue
				}
				if _, ok := m.project.AllIntents()[id]; !ok {
					issues = append(issues, DriftIssue{
						SkillID: id, Type: DriftConfigNew,
						Detail: "全局已启用，但项目层治理未收到该技能（级联缺口）",
					})
				}
			}
		}
	}
	return issues
}

// ResolveSkillDrift 解决检测到的漂移：先把 conflict 占用备份到
// <ConfigRoot>/.drift-backup/<carrier>-<skillId>，再跑 SyncSkillMounts 调谐；
// 调谐成功即视为解决（备份保留以便人工回滚）。strategy 支持 "use-global" /
// "keep-project"（影响原生目录 scope 选择，默认 keep-project）。
func (m *SkillManager) ResolveSkillDrift(opts SkillSyncOptions, strategy string) ([]DriftIssue, error) {
	issues := m.DetectSkillDrift(opts)
	for _, iss := range issues {
		if iss.Type != DriftConflict || iss.Carrier == "" {
			continue
		}
		link := filepath.Join(carrierNativeSkillsDir(iss.Carrier, resolveScope(strategy, iss.SkillID, m), opts.WorkspaceDir, opts.HomeDir), iss.SkillID)
		_ = m.backupConflict(link, iss.Carrier, iss.SkillID)
	}
	if err := m.SyncSkillMounts(opts); err != nil {
		return issues, err
	}
	return issues, nil
}

// resolveScope 按冲突策略选择原生目录 scope。
func resolveScope(strategy, skillID string, m *SkillManager) string {
	if strategy == "use-global" {
		return "global"
	}
	if it := m.IntentOf(skillID); it != nil && it.Scope == "global" {
		return "global"
	}
	return ""
}

// backupConflict 把冲突占用备份到 ConfigRoot 下的 .drift-backup，避免覆盖用户文件。
func (m *SkillManager) backupConflict(link, carrier, skillID string) error {
	info, err := os.Lstat(link)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil // 仅备份非 symlink 占用
	}
	cfgPath := m.config.path
	backupDir := filepath.Join(filepath.Dir(cfgPath), ".drift-backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(backupDir, carrier+"-"+skillID)
	_ = os.Remove(dst)
	return os.Rename(link, dst)
}
