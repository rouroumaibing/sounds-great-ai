package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClosureState represents the lifecycle state of a verdict (10-state machine).
type ClosureState string

const (
	StateOpen          ClosureState = "open"
	StateAcknowledged  ClosureState = "acknowledged"
	StateActionPlanned ClosureState = "action_planned"
	StateFixLanded     ClosureState = "fix_landed"
	StateMainLanded    ClosureState = "main_landed"
	StateLiveActive    ClosureState = "live_active"
	StateReevalPending ClosureState = "reeval_pending"
	StateResolved      ClosureState = "resolved"
	StateSuppressed    ClosureState = "suppressed"
	StateEscalated     ClosureState = "escalated"
)

// ClosureEvent is one event in the verdict lifecycle log.
type ClosureEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor"`
	Data      map[string]any `json:"data,omitempty"`
}

var ErrDuplicateEvent = errors.New("duplicate event")

// ClosureService is the interface for verdict lifecycle management.
type ClosureService interface {
	AppendEvent(ctx context.Context, verdictID string, event ClosureEvent) error
	CurrentState(ctx context.Context, verdictID string) (ClosureState, error)
	GetEvents(ctx context.Context, verdictID string) ([]ClosureEvent, error)
}

// NewClosureService returns Redis-backed service, or memory fallback if redis is nil.
func NewClosureService(rdb *redis.Client) ClosureService {
	if rdb == nil {
		log.Printf("Warning: Redis unavailable, ClosureService using in-memory fallback")
		return &MemoryClosureService{
			events: make(map[string][]ClosureEvent),
			seen:   make(map[string]bool),
		}
	}
	return &RedisClosureService{redis: rdb}
}

// --- RedisClosureService ---

// closureLuaScript is the atomic Lua script for idempotent event append.
const closureLuaScript = `
local already = redis.call('SISMEMBER', KEYS[2], ARGV[1])
if already == 1 then
  return {0, -1}
end
local current = redis.call('LLEN', KEYS[1])
if current ~= tonumber(ARGV[2]) then
  return {-1, current}
end
redis.call('SADD', KEYS[2], ARGV[1])
redis.call('SADD', KEYS[3], ARGV[4])
redis.call('RPUSH', KEYS[1], ARGV[3])
return {1, current}
`

type RedisClosureService struct {
	redis *redis.Client
}

func (s *RedisClosureService) AppendEvent(ctx context.Context, verdictID string, event ClosureEvent) error {
	logKey := fmt.Sprintf("eval:verdict-lifecycle:log:%s", verdictID)
	seenKey := "eval:verdict-lifecycle:events:seen"
	indexKey := "eval:verdict-lifecycle:verdicts"

	// Get current length for optimistic lock
	currentLen, err := s.redis.LLen(ctx, logKey).Result()
	if err != nil {
		return fmt.Errorf("llen: %w", err)
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	res, err := s.redis.Eval(ctx, closureLuaScript, []string{logKey, seenKey, indexKey},
		event.ID, currentLen, string(eventJSON), verdictID).Result()
	if err != nil {
		return fmt.Errorf("eval lua: %w", err)
	}
	vals, ok := res.([]any)
	if !ok || len(vals) < 1 {
		return fmt.Errorf("unexpected lua result: %v", res)
	}
	code, ok := vals[0].(int64)
	if !ok {
		return fmt.Errorf("unexpected lua result type: %T", vals[0])
	}
	if code == 0 {
		return ErrDuplicateEvent
	}
	if code == -1 {
		return fmt.Errorf("sequence conflict: event %s rejected", event.ID)
	}
	return nil
}

func (s *RedisClosureService) GetEvents(ctx context.Context, verdictID string) ([]ClosureEvent, error) {
	logKey := fmt.Sprintf("eval:verdict-lifecycle:log:%s", verdictID)
	raw, err := s.redis.LRange(ctx, logKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	events := make([]ClosureEvent, 0, len(raw))
	for _, item := range raw {
		var ev ClosureEvent
		if err := json.Unmarshal([]byte(item), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func (s *RedisClosureService) CurrentState(ctx context.Context, verdictID string) (ClosureState, error) {
	events, err := s.GetEvents(ctx, verdictID)
	if err != nil {
		return StateOpen, err
	}
	return projectState(events), nil
}

// --- MemoryClosureService (fallback when Redis unavailable) ---

type MemoryClosureService struct {
	events map[string][]ClosureEvent
	seen   map[string]bool
	mu     sync.RWMutex
}

func (s *MemoryClosureService) AppendEvent(_ context.Context, verdictID string, event ClosureEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[event.ID] {
		return ErrDuplicateEvent
	}
	s.seen[event.ID] = true
	s.events[verdictID] = append(s.events[verdictID], event)
	return nil
}

func (s *MemoryClosureService) GetEvents(_ context.Context, verdictID string) ([]ClosureEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events[verdictID], nil
}

func (s *MemoryClosureService) CurrentState(ctx context.Context, verdictID string) (ClosureState, error) {
	events, err := s.GetEvents(ctx, verdictID)
	if err != nil {
		return StateOpen, err
	}
	return projectState(events), nil
}

// projectState computes the current state from the event sequence.
var stateTransitions = map[string]ClosureState{
	"verdict_opened":     StateOpen,
	"owner_acknowledged": StateAcknowledged,
	"action_planned":     StateActionPlanned,
	"fix_recorded":       StateFixLanded,
	"reeval_requested":   StateReevalPending,
	"reeval_passed":      StateResolved,
	"reeval_failed":      StateFixLanded,
	"cvo_suppressed":     StateSuppressed,
	"sla_escalated":      StateEscalated,
}

func projectState(events []ClosureEvent) ClosureState {
	state := StateOpen
	for _, ev := range events {
		if next, ok := stateTransitions[ev.Type]; ok {
			if ev.Type == "cvo_suppressed" || ev.Type == "sla_escalated" {
				return next // terminal overrides
			}
			state = next
		}
	}
	return state
}
