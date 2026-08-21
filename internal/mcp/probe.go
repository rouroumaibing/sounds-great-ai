package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultProbeTimeout bounds how long a single tool enumeration may take.
// External MCP servers that do not speak JSON-RPC over stdio will be killed
// when the context expires, so a slow/broken server never blocks the API.
const DefaultProbeTimeout = 5 * time.Second

// DefaultProbeCacheTTL is how long a successful (or failed) probe result is
// reused before the next request re-enumerates. Keeps the management API cheap
// while still reflecting recent enable/disable edits.
const DefaultProbeCacheTTL = 60 * time.Second

type probeEntry struct {
	tools  []string
	status string // "ok" | "empty" | "error"
	err    string
	at     time.Time
}

// ProbeCache enumerates tools from each MCP server on demand and caches the
// result for a short TTL. Probing spawns the server subprocess over its stdio
// (the Go SDK client does not spawn for StdioTransport; we pipe it ourselves
// via IOTransport), so results are cached to avoid repeated subprocess spawns.
type ProbeCache struct {
	mu      sync.Mutex
	entries map[string]probeEntry
	ttl     time.Duration
	timeout time.Duration
}

func NewProbeCache(ttl, timeout time.Duration) *ProbeCache {
	if ttl <= 0 {
		ttl = DefaultProbeCacheTTL
	}
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	return &ProbeCache{
		entries: make(map[string]probeEntry),
		ttl:     ttl,
		timeout: timeout,
	}
}

// Get returns the cached tools/status for a server, re-probing when the cache
// is stale or force is set. It never returns an error to the caller — probe
// failures are surfaced as status="error" so the API degrades gracefully.
func (c *ProbeCache) Get(name string, cfg *MCPServerConfig, force bool) (tools []string, status string, errMsg string) {
	c.mu.Lock()
	e, fresh := c.entries[name]
	if fresh && !force && time.Since(e.at) < c.ttl {
		c.mu.Unlock()
		return e.tools, e.status, e.err
	}
	c.mu.Unlock()

	tools, status, errMsg = c.probe(cfg)

	c.mu.Lock()
	c.entries[name] = probeEntry{tools: tools, status: status, err: errMsg, at: time.Now()}
	c.mu.Unlock()
	return tools, status, errMsg
}

// probe dispatches to the stdio or remote probe path depending on whether the
// server is reached locally (Command) or over the network (URL).
func (c *ProbeCache) probe(cfg *MCPServerConfig) (tools []string, status string, errMsg string) {
	if cfg == nil {
		return nil, "error", "nil config"
	}
	if cfg.URL != "" {
		return c.probeRemote(cfg)
	}
	if cfg.Command == "" {
		return nil, "error", "no command configured"
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "error", err.Error()
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "error", err.Error()
	}
	if err := cmd.Start(); err != nil {
		return nil, "error", fmt.Sprintf("spawn failed: %v", err)
	}
	// Ensure the subprocess is reaped even if the handshake stalls.
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	transport := &mcp.IOTransport{Reader: stdout, Writer: stdin}
	client := mcp.NewClient(&mcp.Implementation{Name: "sounds-great-ai", Version: "1.0.0"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, "error", fmt.Sprintf("connect failed: %v", err)
	}
	res, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, "error", fmt.Sprintf("tools/list failed: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, t := range res.Tools {
		names = append(names, t.Name)
	}
	if len(names) == 0 {
		return nil, "empty", ""
	}
	return names, "ok", ""
}

// probeRemote connects to a remote (HTTP/SSE) MCP server and enumerates its
// tools. This is SG's outbound remote-MCP client: SG acts as an MCP client to
// an external MCP server rather than spawning a local subprocess. Auth headers
// are injected via a custom RoundTripper so bearer tokens never touch the URL
// or logs.
func (c *ProbeCache) probeRemote(cfg *MCPServerConfig) (tools []string, status string, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "sounds-great-ai", Version: "1.0.0"}, nil)

	var transport mcp.Transport
	switch {
	case strings.HasPrefix(cfg.URL, "sse://"), strings.Contains(cfg.URL, "transport=sse"):
		endpoint := strings.TrimPrefix(cfg.URL, "sse://")
		transport = &mcp.SSEClientTransport{Endpoint: endpoint}
	default:
		httpClient := &http.Client{Timeout: c.timeout}
		if len(cfg.Headers) > 0 {
			httpClient.Transport = &headerInjector{headers: cfg.Headers, base: http.DefaultTransport}
		}
		transport = &mcp.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient}
	}

	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, "error", fmt.Sprintf("remote connect failed: %v", err)
	}
	res, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, "error", fmt.Sprintf("remote tools/list failed: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, t := range res.Tools {
		names = append(names, t.Name)
	}
	if len(names) == 0 {
		return nil, "empty", ""
	}
	return names, "ok", ""
}

// headerInjector is an http.RoundTripper that adds auth headers (e.g.
// Authorization) to every outgoing request — used for remote MCP servers that
// require bearer tokens. The base transport is preserved.
type headerInjector struct {
	headers map[string]string
	base    http.RoundTripper
}

func (h *headerInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	base := h.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
