package guidance

import "testing"

const sampleYAML = `
step1:
  id: step1
  prompt: "Welcome! Tell me your goal."
  next: step2
step2:
  id: step2
  prompt: "Great, here is how to start."
  next: ""
`

func TestGuidance_StateMachine(t *testing.T) {
	e, err := Load(sampleYAML)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := e.Current("")
	if err != nil || cur.ID != "step1" {
		t.Fatalf("start should be step1, got %+v err=%v", cur, err)
	}
	next, hasNext, err := e.Advance("step1")
	if err != nil || !hasNext || next.ID != "step2" {
		t.Fatalf("advance step1 failed: %+v hasNext=%v err=%v", next, hasNext, err)
	}
	// Terminal step.
	_, hasNext, err = e.Advance("step2")
	if err != nil || hasNext {
		t.Fatalf("step2 should be terminal, hasNext=%v err=%v", hasNext, err)
	}
}

func TestGuidance_SystemPromptBuilder(t *testing.T) {
	base := "You are a helpful agent."
	got := SystemPromptBuilder(base, "Do X first.")
	want := base + "\n\n[Guidance] Do X first."
	if got != want {
		t.Fatalf("prompt injection wrong: %q", got)
	}
	if SystemPromptBuilder(base, "") != base {
		t.Fatal("empty step must not alter the prompt")
	}
}
