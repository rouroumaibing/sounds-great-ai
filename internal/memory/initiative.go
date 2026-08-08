package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// OwnedSeed is a private seed adopted from cues by a breed.
// The breed treats this as its own thought, not an external instruction.
type OwnedSeed struct {
	ID        string   `json:"id"`
	BreedID   string   `json:"breed_id"`
	Content   string   `json:"content"`
	Origin    string   `json:"origin"`     // "cue" | "observation" | "inference"
	Lane      LaneType `json:"lane"`
	Timestamp int64    `json:"timestamp"`
}

// IntentAction converts an intent into a concrete action.
type IntentAction struct {
	ID         string `json:"id"`
	SeedID     string `json:"seed_id"`
	Intent     string `json:"intent"`
	Action     string `json:"action"`
	Confidence float64 `json:"confidence"`
	Timestamp  int64  `json:"timestamp"`
}

// InitiativeEngine manages private initiative: owned seeds and intent→action.
// This is the breed's private thought process — NOT platform reasoning.
// The platform only stores and tracks; the breed (CLI) does the thinking.
type InitiativeEngine struct {
	mu       sync.RWMutex
	seeds    map[string]*OwnedSeed
	actions  []*IntentAction
}

// NewInitiativeEngine creates a new InitiativeEngine.
func NewInitiativeEngine() *InitiativeEngine {
	return &InitiativeEngine{
		seeds: make(map[string]*OwnedSeed),
	}
}

// AdoptSeed lets a breed adopt a cue as its own thought (owned seed).
// The breed is the owner — this is private initiative, not platform inference.
func (ie *InitiativeEngine) AdoptSeed(breedID, content, origin string, lane LaneType) *OwnedSeed {
	s := &OwnedSeed{
		ID:        uuid.NewString(),
		BreedID:   breedID,
		Content:   content,
		Origin:    origin,
		Lane:      lane,
		Timestamp: time.Now().UnixMilli(),
	}
	ie.mu.Lock()
	ie.seeds[s.ID] = s
	ie.mu.Unlock()
	return s
}

// GetSeed returns an owned seed by ID.
func (ie *InitiativeEngine) GetSeed(id string) (*OwnedSeed, bool) {
	ie.mu.RLock()
	defer ie.mu.RUnlock()
	s, ok := ie.seeds[id]
	return s, ok
}

// SeedsForBreed returns all owned seeds for a given breed.
func (ie *InitiativeEngine) SeedsForBreed(breedID string) []*OwnedSeed {
	ie.mu.RLock()
	defer ie.mu.RUnlock()
	var result []*OwnedSeed
	for _, s := range ie.seeds {
		if s.BreedID == breedID {
			result = append(result, s)
		}
	}
	return result
}

// SeedsForLane returns all owned seeds for a given lane.
func (ie *InitiativeEngine) SeedsForLane(lane LaneType) []*OwnedSeed {
	ie.mu.RLock()
	defer ie.mu.RUnlock()
	var result []*OwnedSeed
	for _, s := range ie.seeds {
		if s.Lane == lane {
			result = append(result, s)
		}
	}
	return result
}

// RecordAction records an intent→action conversion.
// The breed performs this conversion — the platform only records it.
func (ie *InitiativeEngine) RecordAction(seedID, intent, action string, confidence float64) *IntentAction {
	a := &IntentAction{
		ID:        uuid.NewString(),
		SeedID:    seedID,
		Intent:    intent,
		Action:    action,
		Confidence: confidence,
		Timestamp: time.Now().UnixMilli(),
	}
	ie.mu.Lock()
	ie.actions = append(ie.actions, a)
	ie.mu.Unlock()
	return a
}

// ActionsForSeed returns all actions derived from a given seed.
func (ie *InitiativeEngine) ActionsForSeed(seedID string) []*IntentAction {
	ie.mu.RLock()
	defer ie.mu.RUnlock()
	var result []*IntentAction
	for _, a := range ie.actions {
		if a.SeedID == seedID {
			result = append(result, a)
		}
	}
	return result
}

// AllSeeds returns all owned seeds.
func (ie *InitiativeEngine) AllSeeds() []*OwnedSeed {
	ie.mu.RLock()
	defer ie.mu.RUnlock()
	result := make([]*OwnedSeed, 0, len(ie.seeds))
	for _, s := range ie.seeds {
		result = append(result, s)
	}
	return result
}

// AllActions returns all recorded actions.
func (ie *InitiativeEngine) AllActions() []*IntentAction {
	ie.mu.RLock()
	defer ie.mu.RUnlock()
	result := make([]*IntentAction, len(ie.actions))
	copy(result, ie.actions)
	return result
}

// ForgetSeed removes an owned seed (the breed drops a private thought).
func (ie *InitiativeEngine) ForgetSeed(id string) bool {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	if _, ok := ie.seeds[id]; !ok {
		return false
	}
	delete(ie.seeds, id)
	return true
}
