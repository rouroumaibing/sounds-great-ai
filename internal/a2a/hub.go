package a2a

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"sounds-great-ai/internal/telemetry"
)

type Message struct {
	ID        string
	FromBreed string
	Content   string
	Role      string
}

type Thread struct {
	ID               string
	History          []Message
	Participants     []string
	Task             string
	Status           string
	ReviewRoundCount int
}

func (t *Thread) IncrementReviewRound() { t.ReviewRoundCount++ }
func (t *Thread) ResetReviewRounds()    { t.ReviewRoundCount = 0 }

type Handoff struct {
	FromBreed  string
	ToBreed    string
	Artifact   string
	Context    []Message
	ReviewFlag bool
}

type A2AHub struct {
	threads map[string]*Thread
}

func NewHub(threads map[string]*Thread) *A2AHub {
	if threads == nil {
		threads = make(map[string]*Thread)
	}
	return &A2AHub{threads: threads}
}

func (h *A2AHub) CreateThread(task string, participants []string) *Thread {
	t := &Thread{
		ID:           uuid.New().String(),
		Task:         task,
		Participants: participants,
		Status:       "active",
	}
	h.threads[t.ID] = t
	return t
}

func (h *A2AHub) GetThread(id string) *Thread {
	return h.threads[id]
}

// Handoff records a task handoff from one breed to another in the thread.
// It increments the review round counter, appends the artifact to history,
// and adds the target breed to participants if not already present.
func (h *A2AHub) Handoff(thread *Thread, hf Handoff) (*Thread, error) {
	// Telemetry: record handoff span + counter
	if telemetry.IsInitialized() {
		ctx := context.Background()
		tracer := otel.Tracer("sounds-great-ai")
		_, span := tracer.Start(ctx, "a2a.handoff")
		span.SetAttributes(
			attribute.String("from", hf.FromBreed),
			attribute.String("to", hf.ToBreed),
		)
		defer span.End()
		if telemetry.A2AHandoffCount != nil {
			telemetry.A2AHandoffCount.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("from", hf.FromBreed),
					attribute.String("to", hf.ToBreed),
				))
		}
	}

	thread.IncrementReviewRound()
	thread.History = append(thread.History, Message{
		ID:        uuid.New().String(),
		FromBreed: hf.FromBreed,
		Content:   hf.Artifact,
		Role:      "handoff",
	})
	thread.Participants = appendUnique(thread.Participants, hf.ToBreed)
	return thread, nil
}

// appendUnique appends s to list only if not already present.
func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}
