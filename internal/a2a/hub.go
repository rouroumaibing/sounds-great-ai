package a2a

import (
	"github.com/google/uuid"
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
