package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/telemetry"
)

// distillAgentTimeout bounds how long an autonomous-distill agent run may take
// so the HTTP request can never hang indefinitely on a stuck CLI.
const distillAgentTimeout = 120 * time.Second

// ProfilesHandler exposes the relationship-capsule store (Persistent Identity
// P1) and its Approval-Hub workflow (P1-b "养熟" governance) over HTTP. The
// capsule content itself is never reasoned about inside the platform: a
// candidate is submitted as a *proposal*, and only an explicit operator
// approval promotes it to the active capsule (docs/decisions/irreversible-decisions.md §4.1 — reasoning belongs
// to a CLI agent or the operator, not to internal/).
//
// "autonomous distill" (point 6) is implemented WITHOUT internal reasoning: a
// CLI dog is spawned with the accumulated evidence + current capsule and asked
// to propose an updated primer. The dog does the reasoning; the platform only
// parses its reply, clamps it to the 300-visible-rune budget, and writes it as
// a *pending proposal* the operator must approve (POST /api/profiles/{key}/
// distill/agent). The dog distills ITS OWN primer, so the distiller is the
// distiller is the dog of the CURRENT session: the caller passes
// `?session_id=<active session>` and the platform derives the breed that ran
// it. An explicit `?client_id=<breed>` remains as an operator override. There
// is NO hardcoded default dog — if neither is supplied the endpoint refuses
// (400). The dog's own identity (L0) is injected so it distills in character.
type ProfilesHandler struct {
	profiles   *settings.ProfileRepository
	continuity *settings.ContinuityStore
	evidence   memory.EvidenceStore
	executor   agentsPorts.IAgentExecutor
	// platform resolves a breed id to its CLI client + identity (L0), and
	// resolves a session id to the breed that ran it (DistillAgent's
	// session-derived distiller). Optional; when nil distill treats client_id
	// as a raw CLI name and cannot resolve session_id.
	platform *platform.Platform
	// WorkspaceDir is the cwd handed to the distiller CLI (for file ops/MCP).
	WorkspaceDir string
}

// NewProfilesHandler creates the handler. continuity and evidence may be nil
// (the distill endpoint and continuity cross-links degrade gracefully).
// executor may be nil (autonomous-distill endpoint then returns 503). platform
// may be nil (distill then treats client_id as a raw CLI name, and session
// resolution is unavailable).
func NewProfilesHandler(profiles *settings.ProfileRepository, continuity *settings.ContinuityStore, evidence memory.EvidenceStore, executor agentsPorts.IAgentExecutor, workspaceDir string, plat *platform.Platform) *ProfilesHandler {
	return &ProfilesHandler{
		profiles:     profiles,
		continuity:   continuity,
		evidence:     evidence,
		executor:     executor,
		platform:     plat,
		WorkspaceDir: workspaceDir,
	}
}

// Routes mounts the capsule + approval-hub endpoints under /api/profiles.
func (h *ProfilesHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/profiles", h.List)
	mux.HandleFunc("GET /api/profiles/{key}", h.Get)
	mux.HandleFunc("PUT /api/profiles/{key}", h.Upsert)
	mux.HandleFunc("DELETE /api/profiles/{key}", h.Delete)
	mux.HandleFunc("POST /api/profiles/{key}/propose", h.Propose)
	mux.HandleFunc("GET /api/profiles/{key}/proposal", h.GetProposal)
	mux.HandleFunc("POST /api/profiles/{key}/proposal/approve", h.Approve)
	mux.HandleFunc("POST /api/profiles/{key}/proposal/reject", h.Reject)
	mux.HandleFunc("POST /api/profiles/{key}/distill", h.Distill)
	mux.HandleFunc("POST /api/profiles/{key}/distill/agent", h.DistillAgent)
	return mux
}

// List returns every relationship key with its status, eval counts, and
// whether a proposal is pending.
func (h *ProfilesHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.profiles.ListCapsules()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		c, ok, err := h.profiles.ReadCapsule(key)
		if err != nil || !ok {
			continue
		}
		pending, _ := h.profiles.HasProposal(key)
		out = append(out, map[string]any{
			"relationship_key": key,
			"status":           c.Status,
			"owner_dog":        c.OwnerDog,
			"source_ref":       c.SourceRef,
			"eval_approvals":   c.EvalApprovals,
			"eval_rejections":  c.EvalRejections,
			"updated_at":       c.UpdatedAt,
			"pending_proposal": pending,
		})
	}
	respondJSON(w, http.StatusOK, out)
}

// Get returns a single capsule (with pending_proposal flag).
func (h *ProfilesHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	c, ok, err := h.profiles.ReadCapsule(key)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "capsule not found"})
		return
	}
	pending, _ := h.profiles.HasProposal(key)
	c.PendingProposal = pending
	respondJSON(w, http.StatusOK, c)
}

// Upsert creates or replaces the active capsule for a key. The body is capped
// at settings.MaxCapsuleBodyLen; an over-long body returns 400.
func (h *ProfilesHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var body struct {
		Body           string `json:"body"`
		OwnerDog       string `json:"owner_dog"`
		Status         string `json:"status"`
		SourceRef      string `json:"source_ref"`
		CorrectionPath string `json:"correction_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	c := &settings.RelationshipCapsule{
		RelationshipKey: key,
		OwnerDog:        body.OwnerDog,
		Status:          body.Status,
		SourceRef:       body.SourceRef,
		CorrectionPath:  body.CorrectionPath,
		Body:            body.Body,
	}
	if c.Status == "" {
		c.Status = "active"
	}
	if c.SourceRef == "" {
		c.SourceRef = "operator:manual"
	}
	if err := h.profiles.WriteCapsule(c); err != nil {
		if strings.Contains(err.Error(), settings.ErrCapsuleTooLong.Error()) {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error":  settings.ErrCapsuleTooLong.Error(),
				"limit":  strconv.Itoa(settings.MaxCapsuleBodyLen),
				"detail": err.Error(),
			})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// Delete removes a capsule (and any pending proposal) for a key.
func (h *ProfilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := h.profiles.DeleteCapsule(key); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = h.profiles.DeleteProposal(key)
	respondJSON(w, http.StatusOK, map[string]any{"deleted": true, "relationship_key": key})
}

// Propose submits a candidate capsule as a pending proposal (not applied).
func (h *ProfilesHandler) Propose(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var body struct {
		Body           string `json:"body"`
		OwnerDog       string `json:"owner_dog"`
		SourceRef      string `json:"source_ref"`
		CorrectionPath string `json:"correction_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	c := &settings.RelationshipCapsule{
		RelationshipKey: key,
		OwnerDog:        body.OwnerDog,
		SourceRef:       body.SourceRef,
		CorrectionPath:  body.CorrectionPath,
		Body:            body.Body,
	}
	if err := h.profiles.WriteProposal(key, c); err != nil {
		if strings.Contains(err.Error(), settings.ErrCapsuleTooLong.Error()) {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error":  settings.ErrCapsuleTooLong.Error(),
				"limit":  strconv.Itoa(settings.MaxCapsuleBodyLen),
				"detail": err.Error(),
			})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// KD-10 eval counter: a capsule update proposal was created.
	if telemetry.IsInitialized() && telemetry.ProfileUpdateProposed != nil {
		telemetry.ProfileUpdateProposed.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusCreated, map[string]any{"status": "proposed", "relationship_key": key})
}

// GetProposal returns the pending proposal for a key, or 404 if none.
func (h *ProfilesHandler) GetProposal(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	c, ok, err := h.profiles.ReadProposal(key)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "no pending proposal"})
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// Approve promotes the pending proposal to the active capsule.
func (h *ProfilesHandler) Approve(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	c, err := h.profiles.ApproveProposal(key)
	if err != nil {
		if strings.Contains(err.Error(), "no pending proposal") {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// KD-10 eval counter: a proposal was approved and written to the capsule.
	if telemetry.IsInitialized() && telemetry.ProfileUpdateApproved != nil {
		telemetry.ProfileUpdateApproved.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusOK, c)
}

// Reject discards the pending proposal and bumps the rejection counter.
func (h *ProfilesHandler) Reject(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	c, err := h.profiles.RejectProposal(key)
	if err != nil {
		if strings.Contains(err.Error(), "no pending proposal") {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// KD-10 eval counter: a proposal was rejected (not applied).
	if telemetry.IsInitialized() && telemetry.ProfileUpdateRejected != nil {
		telemetry.ProfileUpdateRejected.Add(context.Background(), 1)
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "rejected", "relationship_key": key, "eval_rejections": cEvalRejections(c)})
}

// Distill collates accumulated evidence for a relationship key into a structured
// draft (the "养熟" trigger). It performs NO reasoning — it only aggregates the
// evidence the platform already holds so an operator or CLI agent can author a
// proposal. It never mutates the active capsule.
func (h *ProfilesHandler) Distill(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if h.evidence == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "evidence store unavailable"})
		return
	}
	records, err := h.evidence.ListEvidence()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	needle := strings.ToLower(key)
	notes := make([]map[string]any, 0)
	for _, rec := range records {
		hay := strings.ToLower(rec.Title + " " + rec.Content + " " + strings.Join(rec.Tags, " "))
		if needle == "" || strings.Contains(hay, needle) {
			notes = append(notes, map[string]any{
				"id":        rec.ID,
				"type":      rec.Type,
				"title":     rec.Title,
				"content":   rec.Content,
				"tags":      rec.Tags,
				"created_at": rec.CreatedAt,
			})
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"relationship_key": key,
		"evidence_count":   len(notes),
		"evidence":         notes,
		"hint":             "review the evidence and submit an update via POST /api/profiles/{key}/propose (or write directly via PUT /api/profiles/{key})",
		"generated_at":     time.Now().UnixMilli(),
	})
}

func cEvalRejections(c *settings.RelationshipCapsule) int {
	if c == nil {
		return 0
	}
	return c.EvalRejections
}

// DistillAgent performs autonomous distill WITHOUT internal reasoning (VISION
// §4.1): it spawns a CLI dog with the accumulated evidence + current capsule
// and asks it to propose an updated relationship primer. The dog does the
// reasoning; the platform collects its reply, clamps it to the 300-visible-rune
// budget (KD-7), and writes it as a *pending proposal* — the operator must
// still approve it (POST .../proposal/approve) before it becomes active. This
// is exactly the "model self-improves, operator approves" loop the user
// accepted. Per F231 (the dog distills its OWN primer), the
// distiller is derived from the CURRENT session via ?session_id (no hardcoded
// default dog); ?client_id=<breed> is an explicit operator override. The dog's
// own identity (L0) is injected so it distills in character.
func (h *ProfilesHandler) DistillAgent(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	// 0. Derive which dog distills — the dog distills its
	// OWN primer, so the distiller is the dog of the CURRENT session, derived
	// from ?session_id (the operator's active session). There is NO hardcoded
	// default: an explicit ?client_id is kept only as an operator override. If
	// neither is supplied we refuse (400) instead of silently picking a dog.
	// Checked before executor/evidence availability so the refusal is not
	// masked by a 503.
	var dogID string
	var usedSession string
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		usedSession = sid
		if h.platform == nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "session resolution unavailable (platform not wired)"})
			return
		}
		b, ok := h.platform.BreedForSession(sid)
		if !ok {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("session %q has no associated dog (is the session active?)", sid)})
			return
		}
		dogID = b
	} else if cid := r.URL.Query().Get("client_id"); cid != "" {
		dogID = cid
	} else {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "distill requires ?session_id (current session) or ?client_id (explicit dog)"})
		return
	}

	if h.executor == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent executor unavailable (autonomous distill disabled)"})
		return
	}
	if h.evidence == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "evidence store unavailable"})
		return
	}

	// 1. Gather evidence for this relationship key (reuse Distill's aggregation).
	records, err := h.evidence.ListEvidence()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	needle := strings.ToLower(key)
	var evLines []string
	for _, rec := range records {
		hay := strings.ToLower(rec.Title + " " + rec.Content + " " + strings.Join(rec.Tags, " "))
		if needle == "" || strings.Contains(hay, needle) {
			evLines = append(evLines, fmt.Sprintf("- [%s] %s: %s", rec.Type, rec.Title, rec.Content))
		}
	}
	evidenceBlock := strings.Join(evLines, "\n")
	if evidenceBlock == "" {
		evidenceBlock = "(无相关 evidence)"
	}

	// 2. Current capsule body (context for the dog; not reasoned about here).
	currentBody := ""
	if c, ok, _ := h.profiles.ReadCapsule(key); ok && c != nil {
		currentBody = c.Body
	}

	// 3. Build the distill prompt. The dog must return the updated primer inside
	// a ```capsule ... ``` fenced block, ≤300 visible runes (Chinese/English/
	// punctuation all count; whitespace does not).
	system := "你是 Sounds Great AI 的「养熟助手」。你的任务是基于 operator 与狗狗协作中积累的证据，蒸馏出一份精炼的关系画像（relationship capsule）。规则：1) 纯文本、第一人称「你」指狗狗、第二人称「operator」；2) 只保留稳定、长期有效的偏好/约定/禁忌，丢弃一次性临时信息；3) 严格 ≤300 个可见字符（中英文标点均计入，空白不计入）；4) 用 ```capsule 和 ``` 包裹最终画像。"
	userPrompt := fmt.Sprintf("关系键：%s\n\n当前画像：\n%s\n\n相关证据（%d 条）：\n%s\n\n请蒸馏出更新后的关系画像，用 ```capsule ... ``` 包裹。",
		key, currentBody, len(evLines), evidenceBlock)

	// 1. Resolve the distiller's CLI client + inject its own identity (L0) so
	// it distills in character (the distiller was derived at step 0 above).
	cliName := dogID
	l0 := ""
	if h.platform != nil {
		if breed := h.platform.GetBreed(dogID); breed != nil {
			if v := breed.DefaultVariant(); v != nil && v.ClientID != "" {
				cliName = v.ClientID
			}
			l0 = h.platform.PromptBuilder.Build(prompt.BuildRequest{BreedID: dogID})
		}
		if _, err := h.platform.GetAdapter(cliName); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("unknown dog or client: %s", dogID)})
			return
		}
	}

	// 2. Spawn the dog (bounded) and collect streamed text.
	ctx, cancel := context.WithTimeout(r.Context(), distillAgentTimeout)
	defer cancel()
	req := agentsPorts.ExecuteRequest{
		ClientID:       cliName,
		SystemPrompt:   system,
		SystemPromptL0: l0,
		Messages:       []*schema.Message{schema.UserMessage(userPrompt)},
		WorkDir:        h.WorkspaceDir,
		Context:        ctx,
	}
	eventCh, err := h.executor.Execute(ctx, req)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("distill agent failed: %v", err)})
		return
	}
	var sb strings.Builder
	for ev := range eventCh {
		if ev.Type == "text" {
			sb.WriteString(ev.Content)
		}
	}
	raw := sb.String()

	// 3. Parse the ```capsule ... ``` block; fall back to the whole reply.
	body := extractFencedBlock(raw, "capsule")
	if strings.TrimSpace(body) == "" {
		body = strings.TrimSpace(raw)
	}
	if strings.TrimSpace(body) == "" {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "distill agent returned no capsule text"})
		return
	}

	// 4. Clamp to the 300-visible-rune budget and write as a pending proposal.
	body = settings.TruncateCapsuleBody(body)
	proposal := &settings.RelationshipCapsule{
		RelationshipKey: key,
		OwnerDog:        dogID,
		SourceRef:       fmt.Sprintf("dog:%s#distill", dogID),
		Body:            body,
	}
	if err := h.profiles.WriteProposal(key, proposal); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{
		"status":            "proposed",
		"relationship_key":  key,
		"distiller":         dogID,
		"client":            cliName,
		"session_id":        usedSession,
		"evidence_count":    len(evLines),
		"hint":              "review the proposal and approve via POST /api/profiles/{key}/proposal/approve",
		"generated_at":      time.Now().UnixMilli(),
	})
}

// AutoDistillSession performs a best-effort autonomous distill on session seal
// (KD-10 F276 maturity, homologous ProfileDistillationTrigger):
// it aggregates the evidence store for the CURRENT session's dog relationship
// key and, if matching evidence exists and no proposal is already pending,
// writes a pending capsule proposal (never auto-applied). It performs NO
// reasoning — faithful to the platform-only aggregation contract (docs/decisions/irreversible-decisions.md §4.1).
// Every failure path is swallowed; the caller must treat it as fire-and-forget.
func (h *ProfilesHandler) AutoDistillSession(ctx context.Context, sessionID, breedID string) {
	if h == nil || h.evidence == nil || h.profiles == nil || h.platform == nil {
		return
	}
	// Env gate: disabled when explicitly off (default on).
	if v := os.Getenv("SG_AUTO_DISTILL_ON_SEAL"); v == "false" || v == "0" {
		return
	}
	key := h.relationshipKeyForBreed(breedID)
	if key == "" {
		return
	}
	// Skip when a proposal is already pending for this key (don't pile up drafts).
	if pending, _ := h.profiles.HasProposal(key); pending {
		return
	}
	// Aggregate evidence (reuse Distill's filter).
	records, err := h.evidence.ListEvidence()
	if err != nil {
		return
	}
	needle := strings.ToLower(key)
	var evLines []string
	for _, rec := range records {
		hay := strings.ToLower(rec.Title + " " + rec.Content + " " + strings.Join(rec.Tags, " "))
		if needle == "" || strings.Contains(hay, needle) {
			evLines = append(evLines, fmt.Sprintf("- [%s] %s: %s", rec.Type, rec.Title, rec.Content))
		}
	}
	if len(evLines) == 0 {
		return // nothing to distill
	}
	body := fmt.Sprintf("（自动蒸馏草稿，基于 %d 条证据，待 operator 审核）\n%s", len(evLines), strings.Join(evLines, "\n"))
	body = settings.TruncateCapsuleBody(body)
	proposal := &settings.RelationshipCapsule{
		RelationshipKey: key,
		OwnerDog:        breedID,
		SourceRef:       fmt.Sprintf("dog:%s#auto-distill", breedID),
		Body:            body,
	}
	if err := h.profiles.WriteProposal(key, proposal); err != nil {
		return
	}
	// KD-10 eval counter: a capsule update proposal was auto-created.
	if telemetry.IsInitialized() && telemetry.ProfileUpdateProposed != nil {
		telemetry.ProfileUpdateProposed.Add(context.Background(), 1)
	}
}

// relationshipKeyForBreed resolves the capsule key for the dog that ran the
// session. The distiller is the session's dog (the dog distills its OWN
// primer), so the key comes from that breed's config.
func (h *ProfilesHandler) relationshipKeyForBreed(breedID string) string {
	if h.platform == nil {
		return ""
	}
	b := h.platform.GetBreed(breedID)
	if b == nil {
		return ""
	}
	return b.RelationshipKey
}

// extractFencedBlock returns the content inside the first ```name ... ``` block
// in text, or "" if none. Whitespace around the block is trimmed.
func extractFencedBlock(text, name string) string {
	open := "```" + name
	start := strings.Index(text, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	rest := text[start:]
	// Skip a single leading newline after the opening fence.
	rest = strings.TrimPrefix(rest, "\n")
	end := strings.Index(rest, "```")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// ContinuityHandler exposes the rotation-aware continuity store (Persistent
// Identity P3, item 5) for inspection. In one-shot mode each breed has a single
// rotation-0 checkpoint; once a long (warm) session exists, the ring fills with
// per-rotation checkpoints.
type ContinuityHandler struct {
	store *settings.ContinuityStore
}

// NewContinuityHandler creates the handler.
func NewContinuityHandler(store *settings.ContinuityStore) *ContinuityHandler {
	return &ContinuityHandler{store: store}
}

// Routes mounts the continuity endpoints under /api/continuity.
func (h *ContinuityHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/continuity", h.List)
	mux.HandleFunc("GET /api/continuity/{breedID}", h.Get)
	return mux
}

// List returns the breeds that currently hold a continuity digest.
func (h *ContinuityHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.List()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, keys)
}

// Get returns the full checkpoint ring for a breed.
func (h *ContinuityHandler) Get(w http.ResponseWriter, r *http.Request) {
	breedID := r.PathValue("breedID")
	doc, ok, err := h.store.GetDoc(breedID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "no continuity digest"})
		return
	}
	respondJSON(w, http.StatusOK, doc)
}
