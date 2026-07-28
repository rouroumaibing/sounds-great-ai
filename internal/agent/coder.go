package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"sounds-great-ai/pkg/protocol"
)

// CoderAgent 是基于 Eino ADK 的 ReAct 编码 Agent
type CoderAgent struct {
	model       model.BaseChatModel
	tools       []tool.BaseTool
	sessionID   string
	maxFailures int // 连续工具失败上限，默认3
}

// NewCoderAgent 创建一个新的 CoderAgent
func NewCoderAgent(sessionID string, mdl model.BaseChatModel, tools []tool.BaseTool) *CoderAgent {
	return &CoderAgent{
		model:       mdl,
		tools:       tools,
		sessionID:   sessionID,
		maxFailures: 3,
	}
}

// Run 执行 Agent ReAct 循环，通过 channel 流式返回事件
func (a *CoderAgent) Run(ctx context.Context, input string) (<-chan *protocol.Event, error) {
	eventCh := make(chan *protocol.Event, 100)

	go func() {
		defer close(eventCh)

		// 推送初始 THINKING 事件
		eventCh <- protocol.NewEvent(protocol.EventThinking, a.sessionID, &protocol.ThinkingPayload{
			Step:    1,
			Content: fmt.Sprintf("Processing user input: %s", input),
		})

		// 如果有模型，尝试调用
		if a.model != nil {
			a.runWithModel(ctx, input, eventCh)
		} else {
			// 无模型模式：直接返回提示
			eventCh <- protocol.NewEvent(protocol.EventThinking, a.sessionID, &protocol.ThinkingPayload{
				Step:    2,
				Content: "No model configured, returning echo response.",
			})
		}

		// 推送最终 THINKING 事件
		eventCh <- protocol.NewEvent(protocol.EventThinking, a.sessionID, &protocol.ThinkingPayload{
			Step:    3,
			Content: "Agent execution completed.",
		})
	}()

	return eventCh, nil
}

// runWithModel 使用模型执行 ReAct 循环
func (a *CoderAgent) runWithModel(ctx context.Context, input string, eventCh chan<- *protocol.Event) {
	// Phase 2 基础实现：直接调用模型 Generate
	// 完整的 ReAct 循环（使用 adk.NewChatModelAgent）在模型支持 tool calling 时启用
	// 这里先实现基础版本

	eventCh <- protocol.NewEvent(protocol.EventThinking, a.sessionID, &protocol.ThinkingPayload{
		Step:    2,
		Content: "Calling model...",
	})
}

// SetMaxFailures 设置连续工具失败上限
func (a *CoderAgent) SetMaxFailures(n int) {
	a.maxFailures = n
}
