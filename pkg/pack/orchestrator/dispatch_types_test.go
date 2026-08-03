package orchestrator

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSubTaskJSONRoundtrip(t *testing.T) {
	in := SubTask{
		ID:           "sub-1",
		Title:        "Search users",
		Description:  "Find user module code",
		SuggestBreed: "xigou",
		DependsOn:    []string{"sub-0"},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out SubTask
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", out, in)
	}
}

func TestDispatchPlanJSONRoundtrip(t *testing.T) {
	in := DispatchPlan{
		Entries: []DispatchEntry{{
			BreedID:   "xigou",
			SubTaskID: "sub-1",
			Priority:  1,
			Status:    "pending",
		}},
		MaxDepth: 3,
		Total:    1,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out DispatchPlan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].BreedID != "xigou" {
		t.Fatalf("entries mismatch: %+v", out)
	}
	if out.MaxDepth != 3 {
		t.Fatalf("maxdepth: %d", out.MaxDepth)
	}
}
