// Package frontdesk implements the "cat-ball front desk" backend: feature
// discovery, memory navigation, and triage (F229). It routes an incoming request
// to the right capability or memory area, failing closed to human triage when it
// cannot route with sufficient confidence.
package frontdesk

import "strings"

// Request is an incoming front-desk request.
type Request struct {
	Query  string
	UserID string
}

// Route is a triage decision.
type Route struct {
	Capability string
	Confidence float64
}

// Triage routes a request. It fails closed: when confidence is below the
// threshold the request is routed to human triage rather than a wrong capability.
func Triage(req Request, confidence, threshold float64) Route {
	if confidence < threshold {
		return Route{Capability: "triage", Confidence: confidence}
	}
	return Route{Capability: classify(req.Query), Confidence: confidence}
}

func classify(q string) string {
	low := strings.ToLower(q)
	switch {
	case strings.Contains(low, "memory") || strings.Contains(low, "remember") || strings.Contains(low, "recall"):
		return "memory"
	case strings.Contains(low, "feature") || strings.Contains(low, "roadmap") || strings.Contains(low, "plan"):
		return "feature"
	default:
		return "triage"
	}
}
