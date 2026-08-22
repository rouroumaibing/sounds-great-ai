// Package governance defines the canonical, governed surface of SG's
// platform-as-MCP-server toolset. It is the single source of truth for the
// tool catalog, including the per-tool governance annotations (readOnly /
// destructive / idempotent / openWorld) and the baseline + attestation used to
// detect tool-surface drift.
//
// Every tool MUST carry a "governance certificate" (the four annotations) or
// registration is rejected; SG enforces this contract at the catalog level.
package governance

// ToolDefinition is one governed platform-capability tool exposed to CLI
// agents. Each tool maps 1:1 onto an SG REST endpoint and belongs to a
// "family" (collab / memory / people / roster / breeds).
type ToolDefinition struct {
	Name        string
	Family      string
	Description string
	Method      string   // GET or POST
	Path        string   // e.g. /api/threads/{id}/messages
	PathParams  []string
	BodyParams  []string
	QueryParams []string
	Required    []string

	// Governance annotations: every tool MUST declare these so the running
	// surface is auditable and the baseline digest is stable.
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	OpenWorld   bool
}

// Catalog returns the full, ordered tool catalog. Order is irrelevant for the
// baseline (which sorts), but kept stable here for readability.
func Catalog() []ToolDefinition {
	return []ToolDefinition{
		// --- collab: threads + messages (the agent's conversation surface) ---
		{
			Name: "sg_list_threads", Family: "collab",
			Description: "[collab] List all conversation threads. Use to discover existing discussions before posting.",
			Method: "GET", Path: "/api/threads", QueryParams: []string{"limit"},
			Required: nil, ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_get_thread", Family: "collab",
			Description: "[collab] Get the event log of a thread by id.",
			Method: "GET", Path: "/api/threads/{id}", PathParams: []string{"id"},
			Required: []string{"id"}, ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_create_thread", Family: "collab",
			Description: "[collab] Create a new conversation thread with a title.",
			Method: "POST", Path: "/api/threads", BodyParams: []string{"title"},
			Required: []string{"title"}, ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false,
		},
		{
			Name: "sg_list_messages", Family: "collab",
			Description: "[collab] List a thread's messages (cursor-paginated via 'before' and 'limit').",
			Method: "GET", Path: "/api/threads/{id}/messages", PathParams: []string{"id"},
			QueryParams: []string{"before", "limit"}, Required: []string{"id"},
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_post_message", Family: "collab",
			Description: "[collab] Post a message to a thread. 'role' defaults to 'user'; 'sender' identifies the author (default 'mcp').",
			Method: "POST", Path: "/api/threads/{id}/messages", PathParams: []string{"id"},
			BodyParams: []string{"content", "role", "sender"}, Required: []string{"id", "content"},
			ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false,
		},

		// --- memory: evidence store ---
		{
			Name: "sg_search_evidence", Family: "memory",
			Description: "[memory] List evidence records (optionally filter by tag/type via query params).",
			Method: "GET", Path: "/api/memory/evidence", QueryParams: []string{"tag", "type", "limit"},
			Required: nil, ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_add_evidence", Family: "memory",
			Description: "[memory] Add an evidence record. 'content' is required; 'type'/'title'/'thread_id'/'tags' optional.",
			Method: "POST", Path: "/api/memory/evidence",
			BodyParams: []string{"content", "type", "title", "thread_id", "tags"},
			Required: []string{"content"}, ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false,
		},

		// --- people: people-memory recall (read-only; propose is gated by source authz) ---
		{
			Name: "sg_people_recall", Family: "people",
			Description: "[people] Recall a person memory card by personID (or 'alias' query).",
			Method: "GET", Path: "/api/people-memory/person/{personID}/card", PathParams: []string{"personID"},
			QueryParams: []string{"alias"}, Required: []string{"personID"},
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},

		// --- roster: dog (agent) profiles ---
		{
			Name: "sg_list_dogs", Family: "roster",
			Description: "[roster] List the dog (agent) roster profiles.",
			Method: "GET", Path: "/api/profiles", Required: nil,
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_get_dog", Family: "roster",
			Description: "[roster] Get a single dog profile by key.",
			Method: "GET", Path: "/api/profiles/{key}", PathParams: []string{"key"},
			Required: []string{"key"}, ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},

		// --- breeds: agent role templates ---
		{
			Name: "sg_list_breeds", Family: "breeds",
			Description: "[breeds] List available breeds (agent role templates).",
			Method: "GET", Path: "/api/breeds", Required: nil,
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},

		// --- dossier: capability profiles (FT-DS-001). Dogs read profiles
		// before complex handoffs, check their own distillation
		// opportunities, and propose evidence-backed summary updates. ---
		{
			Name: "sg_get_dossier", Family: "dossier",
			Description: "[dossier] Read the capability dossier: per-dog structured profiles (one-liner, roster summary, routing signals, provenance) joined with the catalog and grouped by model, plus coverage meta. Use before complex handoffs to route by capability, not role.",
			Method: "GET", Path: "/api/dossier", Required: nil,
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_get_dossier_base_hash", Family: "dossier",
			Description: "[dossier] Get the current dog-dossier.md content hash — required as baseHash when creating a distillation proposal.",
			Method: "GET", Path: "/api/dossier/base-hash", Required: nil,
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_list_distillation_opportunities", Family: "dossier",
			Description: "[dossier] List pending distillation opportunities (capability-relevant events awaiting judgment). The operator sees all; pass actor=<your dogId> to scope to your own. Dismiss if not worth distilling; convert after creating a proposal.",
			Method: "GET", Path: "/api/dossier/distillation-opportunities", QueryParams: []string{"actor"},
			Required: nil, ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_propose_dossier_distillation", Family: "dossier",
			Description: "[dossier] Propose an evidence-backed update to a dog's dossier summary layer (targetDogId, targetFields, beforeSnapshot, afterDraft, rationale, evidenceRefs, baseHash from sg_get_dossier_base_hash). Empty evidenceRefs fails closed. Operator approves; only the target dog may apply.",
			Method: "POST", Path: "/api/dossier/distillations",
			BodyParams: []string{"sourceEvent", "sourceId", "targetDogId", "targetFields", "beforeSnapshot", "afterDraft", "rationale", "evidenceRefs", "baseHash", "actor"},
			Required: []string{"sourceEvent", "sourceId", "targetDogId", "targetFields", "beforeSnapshot", "afterDraft", "rationale", "evidenceRefs", "baseHash"},
			ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false,
		},

		// --- workflow: SOP bulletin board (feature stage, baton holder,
		// resume capsule, check attestations). Information sharing, not flow
		// control — dogs decide their own actions. ---
		{
			Name: "sg_get_workflow_sop", Family: "sop",
			Description: "[sop] Get the SOP workflow bulletin board for a feature (stage, baton holder, next skill, resume capsule, check attestations). 404 when no board exists yet.",
			Method: "GET", Path: "/api/backlog/{itemId}/workflow-sop", PathParams: []string{"itemId"},
			Required: []string{"itemId"}, ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false,
		},
		{
			Name: "sg_update_workflow_sop", Family: "sop",
			Description: "[sop] Update the SOP workflow bulletin board for a feature: record current stage, baton holder, next skill and resume capsule. Stages: kickoff -> impl -> quality_gate -> [fresh_context] -> review -> merge -> completion. Pass expected_stage to CAS-guard concurrent writes (409 on conflict); a new board must start at kickoff. This is information sharing, not flow control.",
			Method: "PUT", Path: "/api/backlog/{itemId}/workflow-sop", PathParams: []string{"itemId"},
			BodyParams: []string{"feature_id", "stage", "baton_holder", "next_skill", "resume_capsule", "expected_stage"},
			Required: []string{"itemId"}, ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false,
		},
	}
}
