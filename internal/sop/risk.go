package sop

// RiskAxis represents one of the five risk dimensions.
type RiskAxis string

const (
	AxisBehavior    RiskAxis = "behavior"    // 行为变更
	AxisData        RiskAxis = "data"        // 数据变更
	AxisSecurity    RiskAxis = "security"    // 安全变更
	AxisContract    RiskAxis = "contract"    // 契约变更
	AxisIrreversible RiskAxis = "irreversible" // 不可逆变更
)

// RiskTrack is the routing track based on risk assessment.
type RiskTrack string

const (
	TrackTargeted RiskTrack = "targeted"  // low risk, fast path
	TrackFullGate RiskTrack = "full_gate" // high risk, all checks
)

// RiskAssessment holds the five-axis risk evaluation.
type RiskAssessment struct {
	Behavior     bool `json:"behavior"`
	Data         bool `json:"data"`
	Security     bool `json:"security"`
	Contract     bool `json:"contract"`
	Irreversible bool `json:"irreversible"`
}

// IsHighRisk returns true if any high-risk axis is flagged.
// Security and irreversible are always high-risk.
// Behavior + data + contract combined is high-risk if 2+ are flagged.
func (r RiskAssessment) IsHighRisk() bool {
	if r.Security || r.Irreversible {
		return true
	}
	count := 0
	if r.Behavior {
		count++
	}
	if r.Data {
		count++
	}
	if r.Contract {
		count++
	}
	return count >= 2
}

// Track returns the routing track based on risk assessment.
func (r RiskAssessment) Track() RiskTrack {
	if r.IsHighRisk() {
		return TrackFullGate
	}
	return TrackTargeted
}

// FlaggedAxes returns the list of flagged risk axes.
func (r RiskAssessment) FlaggedAxes() []RiskAxis {
	var axes []RiskAxis
	if r.Behavior {
		axes = append(axes, AxisBehavior)
	}
	if r.Data {
		axes = append(axes, AxisData)
	}
	if r.Security {
		axes = append(axes, AxisSecurity)
	}
	if r.Contract {
		axes = append(axes, AxisContract)
	}
	if r.Irreversible {
		axes = append(axes, AxisIrreversible)
	}
	return axes
}

// RiskRouter routes changes to tracks based on risk assessment.
type RiskRouter struct{}

// NewRiskRouter creates a new RiskRouter.
func NewRiskRouter() *RiskRouter {
	return &RiskRouter{}
}

// Route evaluates the risk assessment and returns the track.
func (r *RiskRouter) Route(assessment RiskAssessment) RiskTrack {
	return assessment.Track()
}

// RouteFromChangedFiles is a convenience method that assesses risk
// from changed file paths using simple heuristics.
func (r *RiskRouter) RouteFromChangedFiles(files []string) RiskTrack {
	return r.Route(AssessRiskFromFiles(files))
}

// AssessRiskFromFiles uses simple heuristics to flag risk axes from file paths.
// This is a machine-executable heuristic, not LLM reasoning.
func AssessRiskFromFiles(files []string) RiskAssessment {
	var a RiskAssessment
	for _, f := range files {
		switch {
		case containsAny(f, "internal/sop/", "internal/platform/", "internal/router/"):
			a.Behavior = true
			a.Contract = true
		case containsAny(f, "internal/memory/", "internal/ragstore/", "internal/threadstore/"):
			a.Data = true
		case containsAny(f, "packs/default/breeds/", "docs/VISION.md"):
			a.Irreversible = true
		case containsAny(f, "auth", "credential", "secret", "token", "password"):
			a.Security = true
		case containsAny(f, "internal/adapter/", "pkg/protocol/"):
			a.Contract = true
		default:
			a.Behavior = true
		}
	}
	return a
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
