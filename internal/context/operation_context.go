// Package context defines OperationContext: the unified, carrier-agnostic
// request context that every inbound carrier (HTTP / MCP / CLI / A2A) must
// construct before reaching business logic.
//
// It is the single structural carrier for trust/identity/scope facts that the
// later governance (P0-3), routing (P1-A), and multi-user security (P1-D,
// P1-E) capabilities depend on. There is exactly ONE shape for "who is calling
// and with what authority", regardless of how the call arrived.
//
// # Carrier consumption contract (read-only guidance; NOT wired in this change)
//
// Each carrier entry builds an OperationContext at its trust boundary, then
// threads it (e.g. via context.Context or as a parameter) into the handler.
// The four recommended ingress points are:
//
//   - HTTP : internal/transport/ws_handler.go  (HandleWS, *http.Request)
//            build via NewFromHTTP(req) at the top of HandleWS / each REST mux
//            handler so trust is established before dispatch.
//   - MCP  : cmd/platform-mcp-server/main.go   (tool handler dispatch)
//            build via NewFromMCP(ctx) after the MCP session authenticates;
//            the transport layer populates the principal via WithMCPPrincipal.
//   - CLI  : cmd/sg-cli-supervisor/main.go     (os.Args)
//            build via NewFromCLI(os.Args); trust is internal only when the
//            supervisor is platform-invoked (SG_CLI_INVOKED_BY=platform).
//   - A2A  : internal/adapter/a2a/adapter.go   (inbound A2A message — currently
//            client-only; an inbound tasks/send handler should call
//            NewFromA2A(msg)). External agents always start at trust=external
//            and must be granted scopes explicitly.
//
// This package is intentionally free of transport-layer imports (except the
// local pkg/a2a for the message type) so it can be a leaf dependency of every
// carrier without creating import cycles.
package context

import (
	"context"
	"net/http"
	"os"
	"strings"

	a2aprotocol "sounds-great-ai/pkg/a2a"

	"github.com/google/uuid"
)

// PrincipalType enumerates the kind of caller.
type PrincipalType string

const (
	// PrincipalUser is a human operator (browser, CLI user, MCP user).
	PrincipalUser PrincipalType = "user"
	// PrincipalAgent is an autonomous agent (e.g. an A2A peer or a sub-agent).
	PrincipalAgent PrincipalType = "agent"
	// PrincipalService is a non-human service/system (internal job, supervisor).
	PrincipalService PrincipalType = "service"
)

// Principal is the calling identity.
type Principal struct {
	// ID uniquely identifies the principal within its type (e.g. user id,
	// agent name, service name). Never empty after a valid construction.
	ID string
	// Type classifies the principal.
	Type PrincipalType
	// Name is an optional human-readable label.
	Name string
	// ExternalID is the identity asserted by an external system (e.g. the
	// "sub" of a bearer token, or an A2A SenderName). Empty for first-party.
	ExternalID string
}

// TrustBoundary enumerates the trust tier of the request origin.
type TrustBoundary string

const (
	// TrustInternal is a same-process / first-party subsystem we fully trust
	// (e.g. the platform launching its own CLI supervisor).
	TrustInternal TrustBoundary = "internal"
	// TrustExternal is an authenticated external caller (bearer token, A2A
	// peer, authenticated MCP client).
	TrustExternal TrustBoundary = "external"
	// TrustUntrusted is an anonymous / unauthenticated origin. Protected
	// operations are always denied from this boundary (fail-closed).
	TrustUntrusted TrustBoundary = "untrusted"
)

// IsValid reports whether the boundary is one of the known values.
func (t TrustBoundary) IsValid() bool {
	switch t {
	case TrustInternal, TrustExternal, TrustUntrusted:
		return true
	default:
		return false
	}
}

// SourceCarrier enumerates the transport that introduced the request.
type SourceCarrier string

const (
	CarrierHTTP SourceCarrier = "http"
	CarrierMCP  SourceCarrier = "mcp"
	CarrierCLI  SourceCarrier = "cli"
	CarrierA2A  SourceCarrier = "a2a"
)

// IsValid reports whether the carrier is one of the known values.
func (c SourceCarrier) IsValid() bool {
	switch c {
	case CarrierHTTP, CarrierMCP, CarrierCLI, CarrierA2A:
		return true
	default:
		return false
	}
}

// Source identifies the inbound carrier and its concrete identifier.
type Source struct {
	// Carrier is the transport kind.
	Carrier SourceCarrier
	// ID is a carrier-specific identifier: HTTP request id, MCP session id,
	// CLI invocation id, or A2A message id.
	ID string
	// RemoteAddr is the network origin when applicable (HTTP/A2A).
	RemoteAddr string
}

// Scope is a single permission / permission-group token.
type Scope string

// OperationContext is the unified request context. It is a value type meant to
// be passed by pointer; all mutating helpers return a NEW copy (immutable
// chaining) so the original is never silently altered.
type OperationContext struct {
	Principal    Principal
	TrustBoundary TrustBoundary
	Scopes       []Scope
	Source       Source

	// TraceID is an optional correlation id propagated across carriers.
	TraceID string
	// RequestID is a per-request unique id minted at the carrier boundary.
	RequestID string
}

// --- context keys used by the MCP carrier to inject identity -------------

type ctxKey int

const (
	ctxMCPPrincipal ctxKey = iota
	ctxMCPScopes
	ctxMCPSessionID
)

// WithMCPPrincipal attaches an authenticated MCP principal to ctx so that
// NewFromMCP can read it. Called by the MCP transport after auth (P0-3).
func WithMCPPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxMCPPrincipal, p)
}

// WithMCPScopes attaches MCP-granted scopes to ctx for NewFromMCP.
func WithMCPScopes(ctx context.Context, scopes ...Scope) context.Context {
	return context.WithValue(ctx, ctxMCPScopes, scopes)
}

// WithMCPSessionID attaches the MCP session id to ctx for NewFromMCP.
func WithMCPSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ctxMCPSessionID, sessionID)
}

// --- constructors --------------------------------------------------------

// NewFromHTTP builds an OperationContext from an inbound HTTP request.
//
// Trust rule (fail-closed default):
//   - Authorization header present  -> TrustExternal, PrincipalUser
//   - no Authorization               -> TrustUntrusted (anonymous)
//
// Identity/scope may be supplied by trusted headers (X-User-Id, X-Scopes) when
// the request has already been authenticated at a gateway; the HTTP carrier
// (P0-3) is responsible for setting these truthfully.
func NewFromHTTP(req *http.Request) *OperationContext {
	c := &OperationContext{
		TrustBoundary: TrustUntrusted,
		Source: Source{
			Carrier:    CarrierHTTP,
			ID:         req.Header.Get("X-Request-Id"),
			RemoteAddr: req.RemoteAddr,
		},
		RequestID: uuid.NewString(),
	}
	if c.Source.ID == "" {
		c.Source.ID = c.RequestID
	}
	c.TraceID = req.Header.Get("X-Trace-Id")

	if req.Header.Get("Authorization") != "" {
		c.TrustBoundary = TrustExternal
		c.Principal = Principal{
			Type:       PrincipalUser,
			ID:         req.Header.Get("X-User-Id"),
			ExternalID: req.Header.Get("X-User-Id"),
			Name:       req.Header.Get("X-User-Name"),
		}
		if c.Principal.ID == "" {
			c.Principal.ID = "http:" + c.Source.ID
		}
	}
	if sc := req.Header.Get("X-Scopes"); sc != "" {
		for _, s := range strings.Split(sc, ",") {
			if s = strings.TrimSpace(s); s != "" {
				c.Scopes = append(c.Scopes, Scope(s))
			}
		}
	}
	return c
}

// NewFromMCP builds an OperationContext from an MCP session context.
//
// The MCP transport (P0-3) must populate the principal/scopes/session via
// WithMCPPrincipal / WithMCPScopes / WithMCPSessionID BEFORE calling handlers.
// Absent an injected principal, the request is treated as untrusted.
func NewFromMCP(ctx context.Context) *OperationContext {
	c := &OperationContext{
		TrustBoundary: TrustUntrusted,
		Source:        Source{Carrier: CarrierMCP},
		RequestID:     uuid.NewString(),
	}
	if sid, ok := ctx.Value(ctxMCPSessionID).(string); ok {
		c.Source.ID = sid
	}
	if c.Source.ID == "" {
		c.Source.ID = c.RequestID
	}
	if p, ok := ctx.Value(ctxMCPPrincipal).(Principal); ok && p.ID != "" {
		c.Principal = p
		c.TrustBoundary = TrustExternal
		if c.Source.ID == "" && p.ExternalID != "" {
			c.Source.ID = p.ExternalID
		}
	} else {
		c.Principal = Principal{Type: PrincipalService, ID: "mcp:" + c.Source.ID}
	}
	if sc, ok := ctx.Value(ctxMCPScopes).([]Scope); ok {
		c.Scopes = append(c.Scopes, sc...)
	}
	return c
}

// NewFromCLI builds an OperationContext from CLI arguments (os.Args form).
//
// Recognized flags (all optional, overridable by env for the supervisor):
//
//	--principal <id>   principal id (env SG_CLI_PRINCIPAL)
//	--scope <s,...>    comma-separated granted scopes (env SG_CLI_SCOPES)
//	--trust <tier>     internal|external|untrusted (env SG_CLI_TRUST)
//
// Default trust is internal ONLY when the supervisor was platform-invoked
// (SG_CLI_INVOKED_BY=platform); otherwise external. This keeps a CLI that is
// run ad-hoc from silently gaining internal trust.
func NewFromCLI(args []string) *OperationContext {
	principalID := os.Getenv("SG_CLI_PRINCIPAL")
	trust := TrustBoundary(os.Getenv("SG_CLI_TRUST"))
	scopes := splitScopes(os.Getenv("SG_CLI_SCOPES"))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--principal":
			if i+1 < len(args) {
				principalID = args[i+1]
				i++
			}
		case "--scope":
			if i+1 < len(args) {
				scopes = splitScopes(args[i+1])
				i++
			}
		case "--trust":
			if i+1 < len(args) {
				trust = TrustBoundary(args[i+1])
				i++
			}
		}
	}

	if !trust.IsValid() {
		if os.Getenv("SG_CLI_INVOKED_BY") == "platform" {
			trust = TrustInternal
		} else {
			trust = TrustExternal
		}
	}
	if principalID == "" {
		principalID = "cli"
	}

	return &OperationContext{
		Principal:    Principal{Type: PrincipalService, ID: principalID},
		TrustBoundary: trust,
		Scopes:       scopes,
		Source:       Source{Carrier: CarrierCLI, ID: uuid.NewString()},
		RequestID:    uuid.NewString(),
	}
}

// NewFromA2A builds an OperationContext from an inbound A2A message.
//
// External agents are always treated as TrustExternal and are NOT granted any
// scope by default — the governance layer (P0-3) must grant scopes explicitly
// before any protected operation. msg must be non-nil.
func NewFromA2A(msg *a2aprotocol.Message) *OperationContext {
	id := "a2a"
	var sender string
	var remote string
	if msg != nil {
		if msg.ID != "" {
			id = msg.ID
		}
		sender = msg.SenderName
	}
	principalID := "a2a:" + sender
	if sender == "" {
		principalID = id
	}
	return &OperationContext{
		Principal: Principal{
			Type:       PrincipalAgent,
			ID:         principalID,
			ExternalID: sender,
			Name:       sender,
		},
		TrustBoundary: TrustExternal,
		Scopes:        nil,
		Source:        Source{Carrier: CarrierA2A, ID: id, RemoteAddr: remote},
		RequestID:     uuid.NewString(),
	}
}

// --- immutable chaining helpers (return a new copy) -----------------------

func cloneScopes(in []Scope) []Scope {
	if len(in) == 0 {
		return nil
	}
	out := make([]Scope, len(in))
	copy(out, in)
	return out
}

// WithScope returns a copy with scope s appended to the granted scopes.
func (c *OperationContext) WithScope(s Scope) *OperationContext {
	if c == nil {
		return &OperationContext{Scopes: []Scope{s}}
	}
	cp := c.clone()
	cp.Scopes = append(cp.Scopes, s)
	return cp
}

// WithScopes returns a copy with scopes appended.
func (c *OperationContext) WithScopes(scopes ...Scope) *OperationContext {
	if c == nil {
		return &OperationContext{Scopes: cloneScopes(scopes)}
	}
	cp := c.clone()
	cp.Scopes = append(cp.Scopes, scopes...)
	return cp
}

// WithTrustBoundary returns a copy with the trust boundary replaced.
func (c *OperationContext) WithTrustBoundary(t TrustBoundary) *OperationContext {
	cp := c.clone()
	cp.TrustBoundary = t
	return cp
}

// WithPrincipal returns a copy with the principal replaced.
func (c *OperationContext) WithPrincipal(p Principal) *OperationContext {
	cp := c.clone()
	cp.Principal = p
	return cp
}

// WithSource returns a copy with the source replaced.
func (c *OperationContext) WithSource(s Source) *OperationContext {
	cp := c.clone()
	cp.Source = s
	return cp
}

// WithTraceID returns a copy with the trace id set.
func (c *OperationContext) WithTraceID(id string) *OperationContext {
	cp := c.clone()
	cp.TraceID = id
	return cp
}

// clone deep-copies the value so chained helpers never alias the original.
func (c *OperationContext) clone() *OperationContext {
	cp := *c
	cp.Scopes = cloneScopes(c.Scopes)
	return &cp
}

// splitScopes parses a comma-separated scope string into a Scope slice.
func splitScopes(s string) []Scope {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]Scope, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, Scope(p))
		}
	}
	return out
}
