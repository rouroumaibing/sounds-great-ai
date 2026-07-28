package workspace

import (
	"bufio"
	"context"
	"os/exec"
	"path/filepath"
	"sync"
)

// PTYExecutor 在工作区内执行命令
type PTYExecutor struct {
	workspaceDir string
}

// PTYOutput 表示命令输出的一个片段
type PTYOutput struct {
	Stream string // "stdout" or "stderr"
	Data   string
}

// NewPTYExecutor 创建一个新的 PTY 执行器
func NewPTYExecutor(workspaceDir string) *PTYExecutor {
	return &PTYExecutor{workspaceDir: workspaceDir}
}

// Execute 执行命令并通过 channel 流式返回输出
func (p *PTYExecutor) Execute(ctx context.Context, cmd string) (<-chan PTYOutput, error) {
	command := exec.CommandContext(ctx, "sh", "-c", cmd)
	command.Dir = p.workspaceDir
	command.Env = isolatedEnv(p.workspaceDir)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := command.Start(); err != nil {
		return nil, err
	}

	outputCh := make(chan PTYOutput, 100)

	go func() {
		defer close(outputCh)

		var wg sync.WaitGroup
		wg.Add(2)

		// 读取 stdout
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				outputCh <- PTYOutput{Stream: "stdout", Data: scanner.Text() + "\n"}
			}
		}()

		// 读取 stderr
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				outputCh <- PTYOutput{Stream: "stderr", Data: scanner.Text() + "\n"}
			}
		}()

		wg.Wait()
		command.Wait()
	}()

	return outputCh, nil
}

// RunIsolatedCmd 以隔离环境运行命令，返回合并输出
func RunIsolatedCmd(ctx context.Context, workDir string, userCmd string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", userCmd)
	cmd.Dir = workDir
	cmd.Env = isolatedEnv(workDir)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// isolatedEnv 构建隔离的环境变量
func isolatedEnv(workDir string) []string {
	return []string{
		"PATH=/usr/bin:/bin:/usr/local/go/bin",
		"HOME=" + workDir,
		"GOPATH=" + filepath.Join(workDir, ".gopath"),
		"TMPDIR=" + filepath.Join(workDir, ".tmp"),
	}
}
