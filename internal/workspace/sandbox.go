package workspace

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SandboxManager 管理工作区沙箱的生命周期
type SandboxManager struct {
	BaseRepo string // 主仓库路径
	WorkDir  string // 沙箱根目录
}

// NewSandboxManager 创建一个新的沙箱管理器
func NewSandboxManager(baseRepo, workDir string) *SandboxManager {
	return &SandboxManager{
		BaseRepo: baseRepo,
		WorkDir:  workDir,
	}
}

// CleanSandbox 执行三步清理序列
func (m *SandboxManager) CleanSandbox(ctx context.Context, taskID string, deleteBranch bool) error {
	sandboxPath := filepath.Join(m.WorkDir, taskID)

	// 1. 杀死沙箱子进程
	killSandboxProcesses(sandboxPath)

	// 2. 删除沙箱目录 (git worktree remove 或 fallback 到 os.RemoveAll)
	if err := os.RemoveAll(sandboxPath); err != nil {
		log.Printf("警告: 删除沙箱目录失败: %v", err)
	}

	// 3. git branch -D + git worktree prune (如果 deleteBranch 为 true)
	if deleteBranch {
		pruneWorktrees(ctx, m.BaseRepo)
	}

	return nil
}

// StartGCTicker 启动后台 GC，清理孤立沙箱
func (m *SandboxManager) StartGCTicker(ctx context.Context, maxAgeHours int) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runGCOnce(maxAgeHours)
			}
		}
	}()
}

// runGCOnce 执行一次 GC 扫描
func (m *SandboxManager) runGCOnce(maxAgeHours int) {
	entries, err := os.ReadDir(m.WorkDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeHours) * time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(m.WorkDir, entry.Name())
			log.Printf("GC: cleaning orphan sandbox %s (age: %v)", path, time.Since(info.ModTime()))
			killSandboxProcesses(path)
			os.RemoveAll(path)
		}
	}
}

// killSandboxProcesses 杀死与沙箱路径关联的进程
func killSandboxProcesses(sandboxPath string) {
	// 使用 pkill -9 -f <sandboxPath> 杀死相关进程
	// 忽略错误（可能没有匹配的进程）
	cmd := exec.Command("pkill", "-9", "-f", sandboxPath)
	cmd.Run()
}

// pruneWorktrees 清理 git worktree
func pruneWorktrees(ctx context.Context, baseRepo string) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = baseRepo
	cmd.Run()
}
