package capability

import (
	"context"
	"testing"

	"sounds-great-ai/pkg/pack"
)

// sensitiveFilterMock 整合测试用 mock，模拟 sensitive_filter capability
type sensitiveFilterMock struct{}

func (s *sensitiveFilterMock) Name() string    { return "sensitive_filter" }
func (s *sensitiveFilterMock) Version() string { return "v1" }
func (s *sensitiveFilterMock) Init(ctx context.Context) error { return nil }
func (s *sensitiveFilterMock) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	return &pack.TaskOutput{Approved: true, Reason: "no sensitive data detected"}, nil
}
func (s *sensitiveFilterMock) Health() error { return nil }
func (s *sensitiveFilterMock) Close() error  { return nil }

func TestIntegrationZhongHuaTianYuanQuanBlocksDangerousCommand(t *testing.T) {
	// 1. 创建 Pack 并注册 capability
	p := pack.New("test")
	if err := p.RegisterCapability(NewCommandCheck()); err != nil {
		t.Fatalf("RegisterCapability CommandCheck: %v", err)
	}
	if err := p.RegisterCapability(NewPathValidate()); err != nil {
		t.Fatalf("RegisterCapability PathValidate: %v", err)
	}
	if err := p.RegisterCapability(&sensitiveFilterMock{}); err != nil {
		t.Fatalf("RegisterCapability SensitiveFilter: %v", err)
	}

	// 2. 注册 zhonghuatianyuanquan breed（结构与 zhonghuatianyuanquan.json 一致）
	breed := &pack.BreedConfig{
		ID:          "zhonghuatianyuanquan",
		Name:        "zhonghuatianyuanquan",
		DisplayName: "中华田园犬",
		Avatar:      "zhonghuatianyuanquan.png",
		Personality: "忠诚、稳重、守护家园不留死角",
		SystemPrompt: "你是中华田园犬，Pack 的安全护栏。",
		ModelConfig: pack.ModelConfig{
			Provider:    "openai",
			Model:       "gpt-4o-mini",
			Temperature: 0.1,
			MaxTokens:   2048,
		},
		Capabilities: []pack.CapabilityBinding{
			{Name: "command_check", Version: "v1"},
			{Name: "path_validate", Version: "v1"},
			{Name: "sensitive_filter", Version: "v1"},
		},
		Workflow: pack.WorkflowConfig{
			Steps: []pack.WorkflowStep{
				{ID: "cmd", CapabilityRef: "command_check:v1"},
				{ID: "path", CapabilityRef: "path_validate:v1", Depends: []string{"cmd"}},
				{ID: "filter", CapabilityRef: "sensitive_filter:v1", Depends: []string{"path"}},
			},
		},
		Security: pack.SecurityPolicy{
			Network:   false,
			Filesystem: pack.FileSystemPolicy{Write: false, Paths: []string{"/workspace"}},
		},
		Source:  pack.BreedSourceSystem,
		Version: "v1",
	}
	if err := p.Register(breed); err != nil {
		t.Fatalf("Register breed: %v", err)
	}

	// 3. Bark with dangerous command "rm -rf /"
	input := &pack.TaskInput{
		Command: "rm -rf /",
		Path:    "/workspace/src/main.go",
	}
	out, err := p.Bark(context.Background(), "zhonghuatianyuanquan", input)
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}

	// 4. 验证 cmd step 拒绝了危险命令
	steps := out.Data["steps"].(map[string]*pack.TaskOutput)
	cmdResult := steps["cmd"]
	if cmdResult.Approved {
		t.Error("cmd step should block 'rm -rf /', got Approved = true")
	}
	if cmdResult.Reason == "" {
		t.Error("cmd step should have a reason for blocking")
	}
}

func TestIntegrationZhongHuaTianYuanQuanAllowsSafeCommand(t *testing.T) {
	// 1. 创建 Pack 并注册 capability
	p := pack.New("test")
	p.RegisterCapability(NewCommandCheck())
	p.RegisterCapability(NewPathValidate())
	p.RegisterCapability(&sensitiveFilterMock{})

	// 2. 注册 zhonghuatianyuanquan breed
	breed := &pack.BreedConfig{
		ID:          "zhonghuatianyuanquan",
		Name:        "zhonghuatianyuanquan",
		DisplayName: "中华田园犬",
		Capabilities: []pack.CapabilityBinding{
			{Name: "command_check", Version: "v1"},
			{Name: "path_validate", Version: "v1"},
			{Name: "sensitive_filter", Version: "v1"},
		},
		Workflow: pack.WorkflowConfig{
			Steps: []pack.WorkflowStep{
				{ID: "cmd", CapabilityRef: "command_check:v1"},
				{ID: "path", CapabilityRef: "path_validate:v1", Depends: []string{"cmd"}},
				{ID: "filter", CapabilityRef: "sensitive_filter:v1", Depends: []string{"path"}},
			},
		},
		Source: pack.BreedSourceSystem,
	}
	p.Register(breed)

	// 3. Bark with safe command "ls" and normal path
	input := &pack.TaskInput{
		Command: "ls",
		Path:    "/workspace/src/main.go",
	}
	out, err := p.Bark(context.Background(), "zhonghuatianyuanquan", input)
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}

	// 4. 验证所有 step 都通过
	steps := out.Data["steps"].(map[string]*pack.TaskOutput)
	cmdResult := steps["cmd"]
	if !cmdResult.Approved {
		t.Errorf("cmd step should allow 'ls', got Approved = false, Reason = %q", cmdResult.Reason)
	}
	pathResult := steps["path"]
	if !pathResult.Approved {
		t.Errorf("path step should allow normal path, got Approved = false, Reason = %q", pathResult.Reason)
	}
	filterResult := steps["filter"]
	if !filterResult.Approved {
		t.Error("filter step should pass")
	}
}

func TestIntegrationPathValidateBlocksConfigFileWrite(t *testing.T) {
	p := pack.New("test")
	p.RegisterCapability(NewCommandCheck())
	p.RegisterCapability(NewPathValidate())
	p.RegisterCapability(&sensitiveFilterMock{})

	breed := &pack.BreedConfig{
		ID:          "zhonghuatianyuanquan",
		Name:        "zhonghuatianyuanquan",
		Capabilities: []pack.CapabilityBinding{
			{Name: "command_check", Version: "v1"},
			{Name: "path_validate", Version: "v1"},
			{Name: "sensitive_filter", Version: "v1"},
		},
		Workflow: pack.WorkflowConfig{
			Steps: []pack.WorkflowStep{
				{ID: "cmd", CapabilityRef: "command_check:v1"},
				{ID: "path", CapabilityRef: "path_validate:v1", Depends: []string{"cmd"}},
				{ID: "filter", CapabilityRef: "sensitive_filter:v1", Depends: []string{"path"}},
			},
		},
		Source: pack.BreedSourceSystem,
	}
	p.Register(breed)

	// Bark with safe command but config file path
	input := &pack.TaskInput{
		Command: "ls",
		Path:    "go.mod",
	}
	out, err := p.Bark(context.Background(), "zhonghuatianyuanquan", input)
	if err != nil {
		t.Fatalf("Bark: %v", err)
	}

	steps := out.Data["steps"].(map[string]*pack.TaskOutput)
	pathResult := steps["path"]
	if pathResult.Approved {
		t.Error("path step should block writing to go.mod, got Approved = true")
	}
}
