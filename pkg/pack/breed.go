package pack

// ModelConfig 模型配置
type ModelConfig struct {
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"max_tokens"`
	TimeoutSeconds int     `json:"timeout_seconds"`
}

// CapabilityBinding capability 绑定（版本 + 参数）
type CapabilityBinding struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Config  map[string]any `json:"config"`
}

// WorkflowStep workflow 单步
type WorkflowStep struct {
	ID            string   `json:"id"`
	CapabilityRef string   `json:"capability_ref"`
	Depends       []string `json:"depends"`
}

// WorkflowConfig workflow DAG 配置
type WorkflowConfig struct {
	Steps []WorkflowStep `json:"steps"`
}

// BreedSource breed 来源类型
type BreedSource string

const (
	BreedSourceSystem BreedSource = "system"
	BreedSourceUser   BreedSource = "user"
	BreedSourcePlugin BreedSource = "plugin"
)

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	Network    bool             `json:"network"`
	Filesystem FileSystemPolicy `json:"filesystem"`
}

// FileSystemPolicy 文件系统策略
type FileSystemPolicy struct {
	Write bool     `json:"write"`
	Paths []string `json:"paths"`
}

// BreedConfig breed 主配置结构
type BreedConfig struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	DisplayName  string              `json:"display_name"`
	Avatar       string              `json:"avatar"`
	Personality  string              `json:"personality"`
	SystemPrompt string              `json:"system_prompt"`
	ModelConfig  ModelConfig         `json:"model_config"`
	Capabilities []CapabilityBinding `json:"capabilities"`
	Workflow     WorkflowConfig      `json:"workflow"`
	Security     SecurityPolicy      `json:"security"`
	Mentions     []string            `json:"mentions"`
	Source       BreedSource         `json:"source"`
	Version      string              `json:"version"`
}
