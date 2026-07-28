package component

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ProviderType string

const (
	ProviderTypeAPIKey ProviderType = "api_key"
	ProviderTypeCLI    ProviderType = "cli"
)

type ModelConfig struct {
	Type      ProviderType `json:"type"`
	BaseURL   string       `json:"base_url"`
	APIKey    string       `json:"api_key"`
	ModelName string       `json:"model_name"`
	CLIPath   string       `json:"cli_path"`
	CLIArgs   []string     `json:"cli_args"`
}

func NewChatModel(ctx context.Context, cfg *ModelConfig) (model.BaseChatModel, error) {
	switch cfg.Type {
	case ProviderTypeAPIKey:
		chatCfg := &openai.ChatModelConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.ModelName,
			BaseURL: cfg.BaseURL,
		}
		return openai.NewChatModel(ctx, chatCfg)
	case ProviderTypeCLI:
		return &CLIModel{
			cliPath: cfg.CLIPath,
			args:    cfg.CLIArgs,
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}

type CLIModel struct {
	cliPath string
	args    []string
}

func (m *CLIModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	prompt := messagesToPrompt(in)
	cmd := exec.CommandContext(ctx, m.cliPath, m.args...)
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("CLI model %s failed: %w, output: %s", m.cliPath, err, string(out))
	}
	return schema.AssistantMessage(strings.TrimSpace(string(out)), nil), nil
}

func (m *CLIModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	prompt := messagesToPrompt(in)
	cmd := exec.CommandContext(ctx, m.cliPath, m.args...)
	cmd.Stdin = strings.NewReader(prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("CLI model %s stdout pipe failed: %w", m.cliPath, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("CLI model %s start failed: %w", m.cliPath, err)
	}
	reader, writer := schema.Pipe[*schema.Message](10)
	go func() {
		defer writer.Close()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			writer.Send(schema.AssistantMessage(line, nil), nil)
		}
		cmd.Wait()
	}()
	return reader, nil
}

func messagesToPrompt(msgs []*schema.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
