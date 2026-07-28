package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/eino/components/model"
	"github.com/joho/godotenv"

	"sounds-great-ai/internal/a2a/client"
	"sounds-great-ai/internal/a2a/orchestrator"
	"sounds-great-ai/internal/a2a/server"
	"sounds-great-ai/internal/aspect"
	"sounds-great-ai/internal/component"
	"sounds-great-ai/pkg/a2a"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: could not load .env: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create models
	modelA, err := createModel("AIHUBMIX")
	if err != nil {
		log.Fatalf("Agent A model: %v", err)
	}
	modelB, err := createModel("HWS")
	if err != nil {
		log.Fatalf("Agent B model: %v", err)
	}

	// Create command guard
	guard := aspect.NewCommandGuard()

	// Agent cards
	cardA := a2a.AgentCard{
		Name:                "AgentA",
		Description:         "Agent A using AIHUBMIX",
		URL:                 "http://localhost:9001",
		SupportedInterfaces: []string{"tasks/send", "tasks/get", "tasks/cancel"},
	}
	cardB := a2a.AgentCard{
		Name:                "AgentB",
		Description:         "Agent B using HWS",
		URL:                 "http://localhost:9002",
		SupportedInterfaces: []string{"tasks/send", "tasks/get", "tasks/cancel"},
	}

	// System prompts
	promptA := "你是 Agent A，一个大模型助手。你的模型是 " + os.Getenv("AIHUBMIX_API_MODEL") + "。当被问到你是什么模型时，请如实回答。你可以与其他 Agent 交流。你可以在回复中通过 @AgentB 向 Agent B 发送消息。"
	promptB := "你是 Agent B，一个大模型助手。你的模型是 " + os.Getenv("HWS_API_MODEL") + "。当被问到你是什么模型时，请如实回答。你可以与其他 Agent 交流。你可以在回复中通过 @AgentA 向 Agent A 发送消息。"

	// Create servers
	serverA := server.NewAgentServer(cardA, modelA, promptA, guard)
	serverB := server.NewAgentServer(cardB, modelB, promptB, guard)

	if err := serverA.Start(":9001"); err != nil {
		log.Fatalf("Start server A: %v", err)
	}
	defer serverA.Stop()
	if err := serverB.Start(":9002"); err != nil {
		serverA.Stop()
		log.Fatalf("Start server B: %v", err)
	}
	defer serverB.Stop()

	fmt.Println("Agent A listening on :9001")
	fmt.Println("Agent B listening on :9002")
	fmt.Println("Starting 4-turn conversation...")

	// Create clients
	clients := map[string]*client.AgentClient{
		"AgentA": client.NewAgentClient("http://localhost:9001", ""),
		"AgentB": client.NewAgentClient("http://localhost:9002", ""),
	}
	agents := map[string]*server.AgentServer{
		"AgentA": serverA,
		"AgentB": serverB,
	}
	urls := map[string]string{
		"AgentA": "http://localhost:9001",
		"AgentB": "http://localhost:9002",
	}

	// Create and run orchestrator
	script := orchestrator.NewTestScript()
	orch := orchestrator.NewOrchestrator(agents, clients, urls, script)

	if err := orch.Run(ctx); err != nil {
		log.Printf("Orchestrator error: %v", err)
	}

	fmt.Println("\nConversation complete.")
}

func createModel(prefix string) (model.BaseChatModel, error) {
	cfg := &component.ModelConfig{
		Type:      component.ProviderTypeAPIKey,
		BaseURL:   os.Getenv(prefix + "_API_BASEURL"),
		APIKey:    os.Getenv(prefix + "_API_KEY"),
		ModelName: os.Getenv(prefix + "_API_MODEL"),
	}
	return component.NewChatModel(context.Background(), cfg)
}
