package sop

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// CheckStatus represents the status of a workflow check.
type CheckStatus string

const (
	CheckAttested CheckStatus = "attested" // claimed but not verified
	CheckVerified CheckStatus = "verified" // machine-verified
	CheckUnknown  CheckStatus = "unknown"
)

// WorkflowCheck is a single check in the workflow state.
type WorkflowCheck struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	At     time.Time   `json:"at"`
}

// WorkflowState is the SOP state for a feature/task.
type WorkflowState struct {
	FeatureID     string          `json:"feature_id"`
	Stage         string          `json:"stage"`
	BatonHolder   string          `json:"baton_holder"`
	NextSkill     string          `json:"next_skill"`
	ResumeCapsule string          `json:"resume_capsule"`
	Checks        []WorkflowCheck `json:"checks"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Valid stage transitions (rule-driven, not hardcoded DAG).
var validTransitions = map[string][]string{
	"kickoff":      {"impl"},
	"impl":         {"quality_gate"},
	"quality_gate": {"review", "impl"}, // can go back to impl
	"review":       {"merge", "impl"},  // can go back to impl
	"merge":        {"completion", "review"}, // can go back to review
	"completion":   {}, // terminal
}

// IsValidTransition checks if a stage transition is allowed.
func IsValidTransition(from, to string) bool {
	if from == "" {
		return to == "kickoff"
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == to {
			return true
		}
	}
	return false
}

// WorkflowSOP manages Redis-backed SOP state with graceful degradation.
type WorkflowSOP struct {
	redis      *redis.Client
	memStore   map[string]*WorkflowState
	memMu      sync.RWMutex
	useMemory  bool
}

// NewWorkflowSOP creates a WorkflowSOP with Redis. Falls back to memory if Redis is nil.
func NewWorkflowSOP(rdb *redis.Client) *WorkflowSOP {
	ws := &WorkflowSOP{
		memStore: make(map[string]*WorkflowState),
	}
	if rdb != nil {
		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			ws.useMemory = true
		} else {
			ws.redis = rdb
		}
	} else {
		ws.useMemory = true
	}
	return ws
}

// keyPrefix is the Redis key prefix for workflow state.
const keyPrefix = "sop:workflow:"

func (ws *WorkflowSOP) key(featureID string) string {
	return keyPrefix + featureID
}

// GetState retrieves the workflow state for a feature.
func (ws *WorkflowSOP) GetState(ctx context.Context, featureID string) (*WorkflowState, error) {
	if ws.useMemory {
		ws.memMu.RLock()
		defer ws.memMu.RUnlock()
		state, ok := ws.memStore[featureID]
		if !ok {
			return nil, nil
		}
		copied := *state
		return &copied, nil
	}

	data, err := ws.redis.Get(ctx, ws.key(featureID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}
	return &state, nil
}

// SetState sets the workflow state using CAS (Compare-And-Swap).
// expectedStage is the expected current stage; if empty, creates new state.
func (ws *WorkflowSOP) SetState(ctx context.Context, state WorkflowState, expectedStage string) error {
	state.UpdatedAt = time.Now()

	if ws.useMemory {
		return ws.setMemory(ctx, state, expectedStage)
	}
	return ws.setRedis(ctx, state, expectedStage)
}

func (ws *WorkflowSOP) setMemory(_ context.Context, state WorkflowState, expectedStage string) error {
	ws.memMu.Lock()
	defer ws.memMu.Unlock()
	existing, ok := ws.memStore[state.FeatureID]
	if ok {
		if expectedStage != "" && existing.Stage != expectedStage {
			return ErrConcurrentModification
		}
	} else if expectedStage != "" {
		return ErrConcurrentModification
	}
	ws.memStore[state.FeatureID] = &state
	return nil
}

func (ws *WorkflowSOP) setRedis(ctx context.Context, state WorkflowState, expectedStage string) error {
	key := ws.key(state.FeatureID)
	// CAS: watch the key, verify current stage, then set
	txf := func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return err
		}
		if err == redis.Nil {
			if expectedStage != "" {
				return ErrConcurrentModification
			}
		} else {
			var existing WorkflowState
			if err := json.Unmarshal([]byte(current), &existing); err != nil {
				return err
			}
			if expectedStage != "" && existing.Stage != expectedStage {
				return ErrConcurrentModification
			}
		}
		data, err := json.Marshal(state)
		if err != nil {
			return err
		}
		_, err = tx.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, data, 0)
			return nil
		})
		return err
	}
	return ws.redis.Watch(ctx, txf, key)
}

// TransitionStage moves a feature to a new stage with CAS safety.
func (ws *WorkflowSOP) TransitionStage(ctx context.Context, featureID, newStage string) error {
	state, err := ws.GetState(ctx, featureID)
	if err != nil {
		return err
	}
	if state == nil {
		if newStage != "kickoff" {
			return fmt.Errorf("cannot start at stage %s", newStage)
		}
		return ws.SetState(ctx, WorkflowState{
			FeatureID: featureID,
			Stage:     newStage,
		}, "")
	}
	if !IsValidTransition(state.Stage, newStage) {
		return fmt.Errorf("invalid transition %s → %s", state.Stage, newStage)
	}
	oldStage := state.Stage
	state.Stage = newStage
	return ws.SetState(ctx, *state, oldStage)
}

// AttestCheck records a check as attested (claimed but not verified).
func (ws *WorkflowSOP) AttestCheck(ctx context.Context, featureID, checkName string) error {
	state, err := ws.GetState(ctx, featureID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("feature %s not found", featureID)
	}
	oldStage := state.Stage
	state.Checks = upsertCheck(state.Checks, WorkflowCheck{
		Name:   checkName,
		Status: CheckAttested,
		At:     time.Now(),
	})
	return ws.SetState(ctx, *state, oldStage)
}

// VerifyCheck records a check as verified (machine-verified).
func (ws *WorkflowSOP) VerifyCheck(ctx context.Context, featureID, checkName string) error {
	state, err := ws.GetState(ctx, featureID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("feature %s not found", featureID)
	}
	oldStage := state.Stage
	state.Checks = upsertCheck(state.Checks, WorkflowCheck{
		Name:   checkName,
		Status: CheckVerified,
		At:     time.Now(),
	})
	return ws.SetState(ctx, *state, oldStage)
}

// Resume reads state from Redis after restart, returning the resume capsule.
func (ws *WorkflowSOP) Resume(ctx context.Context, featureID string) (*WorkflowState, error) {
	return ws.GetState(ctx, featureID)
}

func upsertCheck(checks []WorkflowCheck, newCheck WorkflowCheck) []WorkflowCheck {
	for i, c := range checks {
		if c.Name == newCheck.Name {
			checks[i] = newCheck
			return checks
		}
	}
	return append(checks, newCheck)
}

// ErrConcurrentModification is returned when CAS detects a concurrent change.
var ErrConcurrentModification = fmt.Errorf("concurrent modification detected")
