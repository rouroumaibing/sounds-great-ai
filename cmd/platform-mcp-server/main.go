package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"sounds-great-ai/internal/mcp/governance"
)

// SECRET_PATTERNS are output-redaction rules: secret material that may appear
// in tool output is scrubbed before it crosses the HTTP surface.
// Defense-in-depth only — the SG platform API is the source of truth and
// already scopes responses.
var SECRET_PATTERNS = []*regexp.Regexp{
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`ghs_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{32,}`),
	regexp.MustCompile(`xox[bpo]-[A-Za-z0-9-]+`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
}

func redactSecrets(text string) string {
	for _, pat := range SECRET_PATTERNS {
		text = pat.ReplaceAllString(text, "[REDACTED-SECRET]")
	}
	return text
}

type server struct {
	apiBase    string
	token      string
	httpClient *http.Client
}

func main() {
	apiBase := flag.String("api-base", "http://localhost:8080", "Base URL of the SG platform HTTP API (loopback).")
	apiToken := flag.String("api-token", os.Getenv("SG_API_TOKEN"), "Bearer token for the SG platform API. Inherited from SG_API_TOKEN when set; empty in dev (auth disabled).")
	transport := flag.String("transport", "stdio", "MCP transport: 'stdio' (default, for local CLI agents) or 'http' (Streamable HTTP).")
	addr := flag.String("addr", "127.0.0.1:8090", "Bind address for --transport http. Defaults to loopback; never bind a public interface.")
	httpToken := flag.String("http-token", os.Getenv("SG_MCP_HTTP_TOKEN"), "Required bearer token for --transport http (fail-closed if empty).")
	flag.Parse()

	srv := &server{
		apiBase: strings.TrimRight(*apiBase, "/"),
		token:   *apiToken,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}

	// Build the governed tool surface once and reuse it for both transports.
	mcpServer := buildServer(srv)

	if *transport == "http" {
		// Fail-closed gate: refuse to start an open surface. We bind loopback
		// only and require a token.
		if *httpToken == "" {
			log.Fatalf("[platform-mcp] --transport http requires --http-token (or SG_MCP_HTTP_TOKEN); refusing to start without auth")
		}
		runHTTP(mcpServer, *addr, *httpToken)
		return
	}

	log.Printf("[platform-mcp] serving %d tools over stdio (api-base=%s)", len(governance.Catalog()), srv.apiBase)
	if err := mcpServer.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("[platform-mcp] server error: %v", err)
	}
}

// buildServer registers every governed catalog tool (with MCP wire annotations)
// onto a fresh mcp.Server. The catalog is the single source of truth
// (governance.Catalog()); the baseline/attestation gate keeps it honest.
func buildServer(s *server) *mcp.Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "sounds-great-platform",
		Version: "1.0.0",
	}, nil)

	for _, t := range governance.Catalog() {
		tool := &mcp.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: buildInputSchema(t),
			Annotations: &mcp.ToolAnnotations{
				Title:           t.Name,
				ReadOnlyHint:    t.ReadOnly,
				IdempotentHint:  t.Idempotent,
				DestructiveHint: boolPtr(t.Destructive),
				OpenWorldHint:   boolPtr(t.OpenWorld),
			},
		}
		mcpServer.AddTool(tool, makeHandler(s, t))
	}
	return mcpServer
}

func boolPtr(b bool) *bool { return &b }

// runHTTP serves the same toolset over MCP Streamable HTTP. Bound to loopback
// by default, token-gated, output-redacted. This exposes SG's OWN platform
// capability tools — not local agents for third-party push — so it is
// consistent with SG's existing inbound REST server and does not violate the
// A2A-server iron law.
func runHTTP(s *mcp.Server, addr, token string) {
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{JSONResponse: true, Stateless: true},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", authMiddleware(token, handler))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"server":  "sounds-great-platform",
			"version": "1.0.0",
			"tools":   len(governance.Catalog()),
			"mode":    "http-streamable",
		})
	})

	srv := &http.Server{Addr: addr, Handler: &methodLoggedWriter{next: mux, addr: addr}}
	log.Printf("[platform-mcp] serving HTTP/Streamable MCP on %s (token required, loopback)", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("[platform-mcp] http server error: %v", err)
	}
}

// authMiddleware enforces a bearer token (query ?token= or Authorization
// header) with a constant-time compare.
func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "unauthorized",
				"hint":  "pass ?token= or Authorization: Bearer",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAuthorized(r *http.Request, token string) bool {
	if token == "" {
		return false // defense-in-depth: never fail-open
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return subtle.ConstantTimeCompare([]byte(q), []byte(token)) == 1
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(token)) == 1
	}
	return false
}

// methodLoggedWriter logs each request.
type methodLoggedWriter struct {
	next http.Handler
	addr string
}

func (m *methodLoggedWriter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[platform-mcp] %s %s", r.Method, r.URL.Path)
	m.next.ServeHTTP(&redactWriter{ResponseWriter: w}, r)
}

// redactWriter wraps an http.ResponseWriter and scrubs known secret patterns
// from every Write, so tool output never leaks tokens across the HTTP surface.
type redactWriter struct {
	http.ResponseWriter
}

func (w *redactWriter) Write(b []byte) (int, error) {
	redacted := []byte(redactSecrets(string(b)))
	n, err := w.ResponseWriter.Write(redacted)
	if err != nil {
		return n, err
	}
	return len(b), nil // report original length to satisfy net/http accounting
}

func (w *redactWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func buildInputSchema(t governance.ToolDefinition) map[string]any {
	props := map[string]any{}
	required := []string{}
	all := append(append([]string{}, t.PathParams...), append(t.BodyParams, t.QueryParams...)...)
	for _, p := range all {
		props[p] = map[string]any{"type": "string", "description": paramDescription(p)}
	}
	for _, r := range t.Required {
		required = append(required, r)
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func paramDescription(p string) string {
	switch p {
	case "id":
		return "Thread id."
	case "itemId":
		return "Backlog item / feature id."
	case "key":
		return "Dog profile key."
	case "personID":
		return "Person id."
	case "content":
		return "Message or evidence content (text)."
	case "title":
		return "Thread title."
	case "role":
		return "Message role (user/assistant/system). Default 'user'."
	case "sender":
		return "Message author identifier. Default 'mcp'."
	case "feature_id":
		return "Feature id; must match the backlog item id (else 422 feature_mismatch)."
	case "stage":
		return "SOP stage: kickoff -> impl -> quality_gate -> [fresh_context] -> review -> merge -> completion."
	case "baton_holder":
		return "Dog holding the baton for the current stage."
	case "next_skill":
		return "Suggested next skill for the next dog."
	case "resume_capsule":
		return "Cold-start capsule: goal / done / current focus, so the next dog resumes without re-reading the whole thread."
	case "expected_stage":
		return "Current stage as read before this update; CAS guard. 409 on mismatch."
	case "type", "tag", "alias", "before", "limit":
		return "Optional filter / pagination parameter."
	default:
		return "Parameter."
	}
}

// makeHandler returns the MCP tool handler closure for a given catalog tool.
func makeHandler(s *server, t governance.ToolDefinition) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult(fmt.Sprintf("invalid arguments JSON: %v", err)), nil
			}
		}
		out, err := s.doRequest(ctx, t, args)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: out}},
		}, nil
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: "error: " + msg}},
	}
}

// doRequest performs the proxied HTTP call to the SG platform API.
func (s *server) doRequest(ctx context.Context, t governance.ToolDefinition, args map[string]any) (string, error) {
	// Substitute path params; missing required path params are an error.
	path := t.Path
	for _, pp := range t.PathParams {
		v, ok := args[pp]
		if !ok || fmt.Sprint(v) == "" {
			return "", fmt.Errorf("missing required path parameter %q", pp)
		}
		path = strings.ReplaceAll(path, "{"+pp+"}", url.PathEscape(fmt.Sprint(v)))
	}

	fullURL := s.apiBase + path
	var bodyReader io.Reader
	if t.Method == "POST" || t.Method == "PUT" {
		payload := map[string]any{}
		for _, bp := range t.BodyParams {
			if v, ok := args[bp]; ok && v != nil && fmt.Sprint(v) != "" {
				payload[bp] = v
			}
		}
		// 'tags' may arrive as a JSON array or a comma string; normalize.
		if raw, ok := args["tags"]; ok && raw != nil {
			payload["tags"] = normalizeTags(raw)
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		q := url.Values{}
		for _, qp := range t.QueryParams {
			if v, ok := args[qp]; ok && fmt.Sprint(v) != "" {
				q.Set(qp, fmt.Sprint(v))
			}
		}
		if len(q) > 0 {
			fullURL += "?" + q.Encode()
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, t.Method, fullURL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		// Surfaced to the agent so it can fall back to the REST callback
		// surface (see GET /api/mcp/servers/platform/fallback) when the
		// platform API is unreachable.
		return "", fmt.Errorf("platform api unreachable (%s %s): %w", t.Method, fullURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("platform api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	// Pretty-print JSON when possible so the agent gets readable output.
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err == nil {
		if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			return string(b), nil
		}
	}
	return string(raw), nil
}

// normalizeTags accepts either a []string or a comma-separated string and
// always returns a []string for the JSON body.
func normalizeTags(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			out = append(out, fmt.Sprint(e))
		}
		return out
	case string:
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	default:
		return nil
	}
}
