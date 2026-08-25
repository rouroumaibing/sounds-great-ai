package message

import "testing"

func TestMerger_SelectDedup(t *testing.T) {
	b := Select([]Message{{ID: "1"}, {ID: "1"}, {ID: "2"}})
	if len(b.IDs) != 2 {
		t.Fatalf("select should dedup, got %d", len(b.IDs))
	}
}

func TestMerger_ForwardIdempotent(t *testing.T) {
	m := NewMerger()
	lookup := func(id string) (Message, bool) { return Message{ID: id}, true }
	b := MessageBundle{IDs: []string{"1", "2"}}
	first, err := m.Forward(b, TransferTarget{ThreadID: "t1"}, lookup)
	if err != nil || len(first) != 2 {
		t.Fatalf("first forward wrong: %v %v", first, err)
	}
	// Second forward is idempotent: nothing new transferred.
	second, err := m.Forward(b, TransferTarget{ThreadID: "t1"}, lookup)
	if err != nil || len(second) != 0 {
		t.Fatalf("second forward must be idempotent, got %v", second)
	}
}

func TestMerger_ForwardEmptyTargetFailClosed(t *testing.T) {
	m := NewMerger()
	if _, err := m.Forward(MessageBundle{IDs: []string{"1"}}, TransferTarget{}, nil); err != ErrEmptyTarget {
		t.Fatalf("empty target must fail closed, got %v", err)
	}
}
