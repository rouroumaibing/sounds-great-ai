package cue

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConsumptionEpisodeAction is how a cue was consumed (content-free ledger).
type ConsumptionEpisodeAction string

const (
	EpisodePresented ConsumptionEpisodeAction = "presented" // cue was injected into prompt
	EpisodeDrilled   ConsumptionEpisodeAction = "drilled"   // agent drilled into the cue
	EpisodeApplied   ConsumptionEpisodeAction = "applied"   // agent used the cue content
	EpisodeDismissed ConsumptionEpisodeAction = "dismissed" // agent ignored the cue
)

// ConsumptionEpisode is a content-free ledger entry tracking cue consumption.
// It records metadata only — no memory content is stored here.
type ConsumptionEpisode struct {
	ID         string                    `json:"id"`
	EnvelopeID string                    `json:"envelope_id"`
	Lane       string                    `json:"lane"`
	Action     ConsumptionEpisodeAction  `json:"action"`
	SessionID  string                    `json:"session_id"`
	Timestamp  int64                     `json:"timestamp"`
}

// SourceInvalidation tracks when a cue's source is invalidated.
// When a source is corrected/forgotten/revoked, the cue must fail-closed.
type SourceInvalidation struct {
	SourceID    string `json:"source_id"`
	Reason      string `json:"reason"` // "corrected" | "forgotten" | "revoked"
	Timestamp   int64  `json:"timestamp"`
}

// ConsumptionLedger tracks cue consumption episodes and source invalidations.
type ConsumptionLedger struct {
	mu             sync.RWMutex
	episodes       []*ConsumptionEpisode
	invalidations  map[string]*SourceInvalidation // sourceID -> invalidation
}

// NewConsumptionLedger creates a new ConsumptionLedger.
func NewConsumptionLedger() *ConsumptionLedger {
	return &ConsumptionLedger{
		invalidations: make(map[string]*SourceInvalidation),
	}
}

// RecordPresented records that a cue was presented (injected into prompt).
func (cl *ConsumptionLedger) RecordPresented(envelopeID, lane, sessionID string) *ConsumptionEpisode {
	return cl.record(envelopeID, lane, EpisodePresented, sessionID)
}

// RecordDrilled records that an agent drilled into a cue.
func (cl *ConsumptionLedger) RecordDrilled(envelopeID, lane, sessionID string) *ConsumptionEpisode {
	return cl.record(envelopeID, lane, EpisodeDrilled, sessionID)
}

// RecordApplied records that an agent applied a cue.
func (cl *ConsumptionLedger) RecordApplied(envelopeID, lane, sessionID string) *ConsumptionEpisode {
	return cl.record(envelopeID, lane, EpisodeApplied, sessionID)
}

// RecordDismissed records that an agent dismissed a cue.
func (cl *ConsumptionLedger) RecordDismissed(envelopeID, lane, sessionID string) *ConsumptionEpisode {
	return cl.record(envelopeID, lane, EpisodeDismissed, sessionID)
}

func (cl *ConsumptionLedger) record(envelopeID, lane string, action ConsumptionEpisodeAction, sessionID string) *ConsumptionEpisode {
	e := &ConsumptionEpisode{
		ID:         uuid.NewString(),
		EnvelopeID: envelopeID,
		Lane:       lane,
		Action:     action,
		SessionID:  sessionID,
		Timestamp:  time.Now().UnixMilli(),
	}
	cl.mu.Lock()
	cl.episodes = append(cl.episodes, e)
	cl.mu.Unlock()
	return e
}

// InvalidateSource marks a source as invalidated.
// Any cue referencing this source must fail-closed (not be injected).
func (cl *ConsumptionLedger) InvalidateSource(sourceID, reason string) *SourceInvalidation {
	inv := &SourceInvalidation{
		SourceID:  sourceID,
		Reason:    reason,
		Timestamp: time.Now().UnixMilli(),
	}
	cl.mu.Lock()
	cl.invalidations[sourceID] = inv
	cl.mu.Unlock()
	return inv
}

// IsSourceInvalidated checks if a source has been invalidated.
// If true, any cue from this source must fail-closed.
func (cl *ConsumptionLedger) IsSourceInvalidated(sourceID string) bool {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	_, ok := cl.invalidations[sourceID]
	return ok
}

// EpisodesForEnvelope returns all consumption episodes for a given envelope.
func (cl *ConsumptionLedger) EpisodesForEnvelope(envelopeID string) []*ConsumptionEpisode {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	var result []*ConsumptionEpisode
	for _, e := range cl.episodes {
		if e.EnvelopeID == envelopeID {
			result = append(result, e)
		}
	}
	return result
}

// EpisodesForSession returns all consumption episodes for a given session.
func (cl *ConsumptionLedger) EpisodesForSession(sessionID string) []*ConsumptionEpisode {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	var result []*ConsumptionEpisode
	for _, e := range cl.episodes {
		if e.SessionID == sessionID {
			result = append(result, e)
		}
	}
	return result
}

// AllEpisodes returns all consumption episodes.
func (cl *ConsumptionLedger) AllEpisodes() []*ConsumptionEpisode {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	result := make([]*ConsumptionEpisode, len(cl.episodes))
	copy(result, cl.episodes)
	return result
}

// AllInvalidations returns all source invalidations.
func (cl *ConsumptionLedger) AllInvalidations() []*SourceInvalidation {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	result := make([]*SourceInvalidation, 0, len(cl.invalidations))
	for _, inv := range cl.invalidations {
		result = append(result, inv)
	}
	return result
}

// EpisodeCount returns the total number of consumption episodes.
func (cl *ConsumptionLedger) EpisodeCount() int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return len(cl.episodes)
}
