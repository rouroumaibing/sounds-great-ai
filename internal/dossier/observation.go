package dossier

import (
	"time"
)

// Observation is a single operator/peer capability observation about a dog.
// Observations accumulate in a staging layer (never the summary layer):
// their only promotion path is being cited as evidence in a distillation
// proposal, which itself needs operator approval. AC-D3: observations do NOT
// replace the peer/operator-judged summary.
type Observation struct {
	ID         string                `json:"id"`
	DogID      string                `json:"dogId"`
	Content    string                `json:"content"`
	Provenance ObservationProvenance `json:"provenance"`
	CreatedAt  time.Time             `json:"createdAt"`
}

// ObservationProvenance anchors who observed what, when.
type ObservationProvenance struct {
	// Type is the observation source kind. v1: "operator" (human) and
	// "peer" (another dog's review remark).
	Type   string `json:"type"`
	Author string `json:"author"`
	Date   string `json:"date"` // YYYY-MM-DD
}

// AddObservationInput is the write payload.
type AddObservationInput struct {
	DogID   string
	Content string
	Author  string
}

// ObservationStore is the staging-layer port.
type ObservationStore interface {
	Add(input AddObservationInput) (Observation, error)
	List(dogID string, limit int) ([]Observation, error)
	ListAll(limit int) (map[string][]Observation, error)
	Get(id string) (Observation, bool, error)
}
