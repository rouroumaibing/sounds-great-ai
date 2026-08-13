package ports

// TrailEntry is a single projected custody event in a thread's audit trail.
type TrailEntry struct {
	Seq       int64  `json:"seq"`
	Type      string `json:"type"`
	Holder    string `json:"holder,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// Briefing projects the custody ledger for a thread into a human-readable
// summary plus the full ordered trail. This is the read model behind the
// Brief & Trail API (D5 engine; the UI is deferred to P5).
type Briefing struct {
	ThreadID string       `json:"thread_id"`
	State    string       `json:"state"`
	Holder   string       `json:"holder,omitempty"`
	Turns    int          `json:"turns"`
	Handoffs int          `json:"handoffs"`
	Holds    int          `json:"holds"`
	Trail    []TrailEntry `json:"trail"`
}

// UnifiedTrailEntry is a single event in the merged custody + code-repo
// timeline (G14). It folds the ball-custody trail and the git-ref repo
// trajectory into one time-ordered axis so a thread's collaboration history
// and its associated code activity are visible together — closing the
// "two independent endpoints" degradation of Gap 4.
type UnifiedTrailEntry struct {
	Source    string `json:"source"` // "custody" | "repo"
	Timestamp int64  `json:"timestamp"`
	Kind      string `json:"kind"` // event type (custody) / repo event kind
	Holder    string `json:"holder,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Branch    string `json:"branch,omitempty"`
	HeadSHA   string `json:"head_sha,omitempty"`
	Seq       int64  `json:"seq,omitempty"`
}

// BriefingSummary is a compact per-thread entry in a duty briefing.
type BriefingSummary struct {
	ThreadID string `json:"thread_id"`
	Holder   string `json:"holder,omitempty"`
	State    string `json:"state"`
	UpdatedAt int64 `json:"updated_at"`
}

// DutyBriefing is the cross-thread operations view (G6). It classifies every
// thread's custody state into operator-actionable buckets so a human can see,
// at a glance, what needs attention.
type DutyBriefing struct {
	GeneratedAt   int64              `json:"generated_at"`
	Counts        map[string]int     `json:"counts"`
	NeedsUser     []BriefingSummary  `json:"needs_user"`     // parked holds awaiting wake
	DeadBalls     []BriefingSummary  `json:"dead_balls"`     // zombie / dead
	VoidPasses    []BriefingSummary  `json:"void_passes"`    // void (no valid target)
	StaleBlocked  []BriefingSummary  `json:"stale_blocked"`  // blocked and aged
}
