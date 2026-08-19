package skills

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// nativeSkillsDirConvention 描述各 carrier 的原生 skills 目录布局（相对 base 目录）。
// 仅这些 carrier 拥有原生 skills 目录，可物理挂载（符号链接）；其余 carrier
// （如 opencode）无统一原生目录，仅做逻辑挂载（启用即注入 prompt），不建链接。
//
// 这与 SG 过去的「仅 claude 物理挂载」不同：随着外部 CLI 生态成熟，SG 现在支持将
// skill 正文物理挂载到 claude/codex/gemini/kimi 各自的原生 skills 目录，使每只 CLI
// 都能用其自身的 skill 加载器发现并加载（离线可用、client-native 生命周期）。
// 物理挂载与逻辑挂载并存：物理挂载增强 CLI 原生发现，逻辑挂载保证 system prompt 注入。
var nativeSkillsDirConvention = map[string][]string{
	"claude": {".claude", "skills"},
	"codex":  {".codex", "skills"},
	"gemini": {".gemini", "skills"},
	"kimi":   {".kimi", "skills"},
}

// carrierSupportsNativeSkillsDir 报告某 carrier 是否从磁盘原生 skills 目录读取 SKILL.md。
func carrierSupportsNativeSkillsDir(carrier string) bool {
	_, ok := nativeSkillsDirConvention[carrier]
	return ok
}

// carrierNativeSkillsDir 返回某 carrier 的原生 skills 目录。scope=global 落到 home，
// 否则落到当前项目 workspaceDir（支持全局基线 + 工作区覆盖两级）。
func carrierNativeSkillsDir(carrier, scope, workspaceDir, homeDir string) string {
	conv, ok := nativeSkillsDirConvention[carrier]
	if !ok {
		return ""
	}
	base := workspaceDir
	if scope == "global" {
		base = homeDir
	}
	parts := append([]string{base}, conv...)
	return filepath.Join(parts...)
}

// NativeSkillsDir 是 carrierNativeSkillsDir 的导出包装，供 transport handler 计算挂载健康。
func NativeSkillsDir(carrier, scope string, opts SkillSyncOptions) string {
	return carrierNativeSkillsDir(carrier, scope, opts.WorkspaceDir, opts.HomeDir)
}

// ClaudeSkillsDir 是 claude 原生 skills 目录的便捷包装（委托 NativeSkillsDir）。
func ClaudeSkillsDir(scope, workspaceDir, homeDir string) string {
	return carrierNativeSkillsDir("claude", scope, workspaceDir, homeDir)
}

// SkillSyncOptions 是挂载调谐所需的路径上下文。
type SkillSyncOptions struct {
	WorkspaceDir string
	HomeDir      string
}

// managedNativeDirs 返回所有支持原生目录的 carrier 的 project/home 两级 skills 目录
// 集合，作为清理阶段的扫描范围（确保禁用后旧链接也能被清理）。
func managedNativeDirs(opts SkillSyncOptions) map[string]bool {
	dirs := make(map[string]bool)
	for _, carrier := range KnownCarriers {
		if !carrierSupportsNativeSkillsDir(carrier) {
			continue
		}
		dirs[carrierNativeSkillsDir(carrier, "project", opts.WorkspaceDir, opts.HomeDir)] = true
		dirs[carrierNativeSkillsDir(carrier, "global", opts.WorkspaceDir, opts.HomeDir)] = true
	}
	return dirs
}

// carrierList 返回某意图要挂载的 carrier 集合。mountPoints 为空表示挂到所有已知
// carrier（启用即全量逻辑挂载，符合默认 UX）。
func carrierList(it *SkillIntent) []string {
	if it == nil || len(it.MountPoints) == 0 {
		return KnownCarriers
	}
	return it.MountPoints
}

// mountsToCarrier 判断某意图是否把 skill 挂到给定 carrier。
func mountsToCarrier(it *SkillIntent, carrier string) bool {
	if it == nil || !it.Enabled {
		return false
	}
	if len(it.MountPoints) == 0 {
		return true
	}
	for _, c := range it.MountPoints {
		if c == carrier {
			return true
		}
	}
	return false
}

// isManagedTarget 判断符号链接目标是否落在 SG 管理的 skill 源目录内，避免误删
// 用户在 CLI skills 目录下手动放置的文件。
func (m *SkillManager) isManagedTarget(target string) bool {
	m.mu.RLock()
	roots := make([]string, 0, len(m.sources))
	for _, sr := range m.sources {
		roots = append(roots, sr.dir)
	}
	m.mu.RUnlock()
	for _, r := range roots {
		if strings.HasPrefix(target, r) {
			return true
		}
	}
	return false
}

// SyncSkillMounts 把磁盘符号链接（SG 管理的部分）调谐到生效意图。
// 对支持原生 skills 目录的 carrier（claude/codex/gemini/kimi）按 per-skill 建符号
// 链接（不采用目录级单链接，因部分 CLI 不递归扫描子目录会导致「已链接但不可加载」）；
// 逻辑挂载的 carrier 在构建时由 SkillIDs 消费，无需磁盘操作。
//
// scope 由合并意图决定：global 层启用的 skill 写入 home 原生目录（所有项目共享基线），
// project 层覆盖启用的 skill 写入 workspace 原生目录（工作区特定），从而实现级联挂载。
//
// 原子性：先读意图，删 stale 的托管 symlink，再建 desired 的 symlink，最后写回
// sync 状态。symlink 创建失败会返回错误（调用方可据此决策是否回滚意图）。
func (m *SkillManager) SyncSkillMounts(opts SkillSyncOptions) error {
	m.mu.RLock()
	intents := m.MergedIntents()
	skills := make(map[string]*Skill, len(m.skills))
	for k, v := range m.skills {
		skills[k] = v
	}
	m.mu.RUnlock()

	// 计算 desired 链接集合（link 路径 -> skill 目录）。
	desired := make(map[string]string)
	for id, it := range intents {
		sk, ok := skills[id]
		if !ok || !it.Enabled {
			continue
		}
		skillDir := filepath.Dir(sk.FilePath)
		for _, carrier := range carrierList(it) {
			if !carrierSupportsNativeSkillsDir(carrier) {
				continue
			}
			dir := carrierNativeSkillsDir(carrier, it.Scope, opts.WorkspaceDir, opts.HomeDir)
			link := filepath.Join(dir, id)
			desired[link] = skillDir
		}
	}

	// Phase 3：删除 disabled/removed skill 在所有 carrier 目录的托管 symlink。
	// managedDirs 覆盖所有原生 carrier 的 project/home 两级目录，确保禁用后
	// 旧链接也能被清理（desired 为空时不会漏扫）。
	managedDirs := managedNativeDirs(opts)
	for dir := range managedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			link := filepath.Join(dir, e.Name())
			if _, ok := desired[link]; ok {
				continue
			}
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := os.Readlink(link)
			if err == nil && m.isManagedTarget(target) {
				_ = os.Remove(link)
			}
		}
	}

	// Phase 2：建 desired 符号链接。
	for link, targetDir := range desired {
		if cur, err := os.Readlink(link); err == nil && cur == targetDir {
			continue // 已正确指向，跳过
		}
		_ = os.Remove(link)
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(targetDir, link); err != nil {
			return err
		}
	}

	// Phase 4：写回调谐基线。
	hash := computeManifestHash(skills, intents)
	return m.config.SetSyncState(hash, time.Now().UTC().Format(time.RFC3339))
}

// computeManifestHash 对（源清单 + 意图）做确定性哈希，作为漂移检测基线。
func computeManifestHash(skills map[string]*Skill, intents map[string]*SkillIntent) string {
	ids := make([]string, 0, len(skills)+len(intents))
	for id := range skills {
		ids = append(ids, "src:"+id)
	}
	for id := range intents {
		ids = append(ids, "int:"+id)
	}
	sort.Strings(ids)
	h := sha256.New()
	for _, id := range ids {
		if sk, ok := skills[id[len("src:"):]]; ok {
			h.Write([]byte(id + ":" + sk.Name + "\n"))
		}
		if it, ok := intents[id[len("int:"):]]; ok {
			h.Write([]byte(id + ":" + it.canonical() + "\n"))
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// canonical 返回意图的规范化字符串（用于哈希）。
func (it *SkillIntent) canonical() string {
	mp := append([]string(nil), it.MountPoints...)
	sort.Strings(mp)
	return it.Scope + "|" + boolStr(it.Enabled) + "|" + strings.Join(mp, ",")
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
