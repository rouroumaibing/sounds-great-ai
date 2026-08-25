// Package guidance implements a scenario-based onboarding engine (F155): a YAML
// state machine that drives step-by-step guidance and injects the current step
// into the system prompt via SystemPromptBuilder.
package guidance

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Step is one node in the guidance state machine.
type Step struct {
	ID     string `yaml:"id"`
	Prompt string `yaml:"prompt"`
	Next   string `yaml:"next"` // next step id, "" = terminal
}

// Engine is the scenario-based guidance engine (F155).
type Engine struct {
	steps map[string]Step
	start string
}

// Load parses a YAML state machine: a map of step id -> Step.
func Load(yamlSrc string) (*Engine, error) {
	var steps map[string]Step
	if err := yaml.Unmarshal([]byte(yamlSrc), &steps); err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("guidance: no steps")
	}
	e := &Engine{steps: steps}
	// Deterministic start: lexicographically first step id.
	start := ""
	for id := range steps {
		if start == "" || id < start {
			start = id
		}
	}
	e.start = start
	return e, nil
}

// Current returns the step for a state id (or the start step when empty).
func (e *Engine) Current(stateID string) (Step, error) {
	id := stateID
	if id == "" {
		id = e.start
	}
	s, ok := e.steps[id]
	if !ok {
		return Step{}, fmt.Errorf("guidance: unknown state %q", id)
	}
	return s, nil
}

// Advance moves from a state to its next step. The bool reports whether a next
// step exists (false at a terminal step).
func (e *Engine) Advance(stateID string) (Step, bool, error) {
	cur, err := e.Current(stateID)
	if err != nil {
		return Step{}, false, err
	}
	if cur.Next == "" {
		return cur, false, nil
	}
	next, ok := e.steps[cur.Next]
	if !ok {
		return Step{}, false, fmt.Errorf("guidance: dangling next %q", cur.Next)
	}
	return next, true, nil
}

// SystemPromptBuilder injects the current guidance step into the system prompt.
func SystemPromptBuilder(base, stepPrompt string) string {
	if stepPrompt == "" {
		return base
	}
	return base + "\n\n[Guidance] " + stepPrompt
}
