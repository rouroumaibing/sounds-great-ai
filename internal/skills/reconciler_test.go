package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestSkill(t *testing.T, dir, id, name string) string {
	t.Helper()
	skDir := filepath.Join(dir, id)
	if err := os.MkdirAll(skDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(skDir, "SKILL.md")
	content := "---\nid: " + id + "\nname: " + name + "\n---\nbody\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSyncSkillMounts_MultiCarrierSymlink(t *testing.T) {
	src := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "skills-config.json")
	ws := t.TempDir()
	home := t.TempDir()

	p := writeTestSkill(t, src, "demo", "Demo")
	m := NewManagerWithConfig(cfgPath, "", map[string]string{src: "packs"})
	if err := m.Config().Load(); err != nil {
		t.Fatal(err)
	}
	if err := m.Scan(); err != nil {
		t.Fatal(err)
	}
	if err := m.SetEnabled("demo", true, "project"); err != nil {
		t.Fatal(err)
	}
	// 挂载到全部支持原生目录的 carrier（claude/codex/gemini/kimi），opencode 仅逻辑挂载。
	if err := m.SetMountPoints("demo", []string{"claude", "codex", "gemini", "kimi"}); err != nil {
		t.Fatal(err)
	}

	opts := SkillSyncOptions{WorkspaceDir: ws, HomeDir: home}
	if err := m.SyncSkillMounts(opts); err != nil {
		t.Fatalf("SyncSkillMounts: %v", err)
	}

	for _, carrier := range []string{"claude", "codex", "gemini", "kimi"} {
		dir := NativeSkillsDir(carrier, "project", opts)
		link := filepath.Join(dir, "demo")
		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("expected symlink at %s: %v", link, err)
		}
		if target != filepath.Dir(p) {
			t.Fatalf("%s symlink target = %s, want %s", carrier, target, filepath.Dir(p))
		}
	}
	// opencode 无原生目录，不应建链接。
	if _, err := os.Lstat(filepath.Join(ws, ".opencode", "skills", "demo")); !os.IsNotExist(err) {
		t.Fatalf("opencode should not have a native symlink")
	}

	// 禁用后重新调谐，四个 carrier 的符号链接都应被移除。
	if err := m.SetEnabled("demo", false, "project"); err != nil {
		t.Fatal(err)
	}
	if err := m.SyncSkillMounts(opts); err != nil {
		t.Fatalf("SyncSkillMounts(disable): %v", err)
	}
	for _, carrier := range []string{"claude", "codex", "gemini", "kimi"} {
		link := filepath.Join(NativeSkillsDir(carrier, "project", opts), "demo")
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Fatalf("expected %s symlink removed, got err=%v", carrier, err)
		}
	}
}

func TestCascadeConfigNewOrphan(t *testing.T) {
	src := t.TempDir()
	globalCfg := filepath.Join(t.TempDir(), "global.json")
	projCfg := filepath.Join(t.TempDir(), "project.json")
	ws := t.TempDir()
	home := t.TempDir()
	writeTestSkill(t, src, "demo", "Demo")

	// 项目层已启用治理（写入一条覆盖），但「ghost」引用了源已删除的 skill → config-orphan。
	m := NewManagerWithConfig(globalCfg, projCfg, map[string]string{src: "packs"})
	if err := m.ReloadAll(); err != nil {
		t.Fatal(err)
	}
	if err := m.SetEnabled("demo", true, "global"); err != nil {
		t.Fatal(err)
	}
	// 项目层覆盖一个源已删除的 orphan（项目治理已激活，≥1 覆盖）。
	if err := m.project.SetEnabled("ghost", true, "project"); err != nil {
		t.Fatal(err)
	}

	opts := SkillSyncOptions{WorkspaceDir: ws, HomeDir: home}
	issues := m.DetectSkillDrift(opts)
	seen := map[DriftType]bool{}
	for _, iss := range issues {
		seen[iss.Type] = true
	}
	if !seen[DriftConfigNew] {
		t.Fatalf("expected config-new drift, got %+v", issues)
	}
	if !seen[DriftConfigOrphan] {
		t.Fatalf("expected config-orphan drift, got %+v", issues)
	}
}

func TestSecurityBlocksExternal(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "external.md"), []byte("---\nname: External\ndescription: e\n---\nbody"), 0o644)

	// 外部不可信源（user）：启用后处于 pending，阻断注入，需人工批准后才放行。
	m := NewManagerWithConfig("", "", map[string]string{src: "user"})
	if err := m.Scan(); err != nil {
		t.Fatal(err)
	}
	if err := m.SetEnabled("external", true, "project"); err != nil {
		t.Fatal(err)
	}
	if len(m.AllEnabled()) != 0 {
		t.Fatalf("pending external must be blocked until approved, got %d", len(m.AllEnabled()))
	}
	// 人工批准后放行。
	if err := m.ApproveSkill("external", "operator"); err != nil {
		t.Fatal(err)
	}
	enabled := m.AllEnabled()
	if len(enabled) != 1 || enabled[0].ID != "external" {
		t.Fatalf("after approve AllEnabled = %v, want [external]", idsOf(enabled))
	}
	// 隔离后再次阻断。
	if err := m.QuarantineSkill("external", "operator"); err != nil {
		t.Fatal(err)
	}
	if len(m.AllEnabled()) != 0 {
		t.Fatalf("quarantined skill must be blocked, got %d", len(m.AllEnabled()))
	}
}

func TestDetectSkillDrift_ConflicAndStale(t *testing.T) {
	src := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "skills-config.json")
	ws := t.TempDir()
	home := t.TempDir()
	writeTestSkill(t, src, "demo", "Demo")

	m := NewManagerWithConfig(cfgPath, "", map[string]string{src: "packs"})
	if err := m.Config().Load(); err != nil {
		t.Fatal(err)
	}
	if err := m.Scan(); err != nil {
		t.Fatal(err)
	}
	if err := m.SetEnabled("demo", true, "project"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetMountPoints("demo", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	opts := SkillSyncOptions{WorkspaceDir: ws, HomeDir: home}

	// 在 claude skills 目录手动放一个非 symlink 同名占用 → conflict。
	claudeDir := filepath.Join(ws, ".claude", "skills")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "demo"), []byte("user file"), 0o644); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, iss := range m.DetectSkillDrift(opts) {
		if iss.SkillID == "demo" && iss.Type == DriftConflict {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected conflict drift, got %+v", m.DetectSkillDrift(opts))
	}

	// 解决：备份占用并调谐 → 冲突消除，链接就位。
	if _, err := m.ResolveSkillDrift(opts, "keep-project"); err != nil {
		t.Fatalf("ResolveSkillDrift: %v", err)
	}
	for _, iss := range m.DetectSkillDrift(opts) {
		if iss.SkillID == "demo" && iss.Type == DriftConflict {
			t.Fatalf("conflict should be resolved: %+v", iss)
		}
	}
	if _, err := os.Readlink(filepath.Join(claudeDir, "demo")); err != nil {
		t.Fatalf("expected symlink after resolve: %v", err)
	}
}

func TestEnabledForCarrier(t *testing.T) {
	src := t.TempDir()
	cfgPath := filepath.Join(t.TempDir(), "skills-config.json")
	writeTestSkill(t, src, "demo", "Demo")

	m := NewManagerWithConfig(cfgPath, "", map[string]string{src: "packs"})
	if err := m.Config().Load(); err != nil {
		t.Fatal(err)
	}
	if err := m.Scan(); err != nil {
		t.Fatal(err)
	}
	// 启用但未指定 mountPoints → 默认挂所有 carrier。
	if err := m.SetEnabled("demo", true, "project"); err != nil {
		t.Fatal(err)
	}
	if got := m.EnabledForCarrier("claude"); len(got) != 1 || got[0] != "demo" {
		t.Fatalf("EnabledForCarrier(claude) = %v, want [demo]", got)
	}
	if got := m.EnabledForCarrier("kimi"); len(got) != 1 {
		t.Fatalf("EnabledForCarrier(kimi) = %v, want [demo]", got)
	}
	// 仅挂 claude → kimi 不再命中。
	if err := m.SetMountPoints("demo", []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if got := m.EnabledForCarrier("kimi"); len(got) != 0 {
		t.Fatalf("EnabledForCarrier(kimi) = %v, want []", got)
	}
}
