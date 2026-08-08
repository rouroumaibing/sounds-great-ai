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

func TestDispatchEntryJSONRoundtrip(t *testing.T) {
	in := DispatchEntry{
		BreedID:    "bianmu",
		SubTaskID:  "sub-5",
		Priority:   2,
		DependsOn:  []string{"sub-1", "sub-3"},
		Status:     "pending",
		SkipReason: "",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out DispatchEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", out, in)
	}
}

func TestDispatchEntryAllFields(t *testing.T) {
	in := DispatchEntry{
		BreedID:    "demu",
		SubTaskID:  "sub-9",
		Priority:   5,
		DependsOn:  []string{"sub-1", "sub-2", "sub-3"},
		Status:     "skipped",
		SkipReason: "breed unavailable",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out DispatchEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SkipReason != "breed unavailable" {
		t.Errorf("SkipReason = %q", out.SkipReason)
	}
	if out.Status != "skipped" {
		t.Errorf("Status = %q", out.Status)
	}
	if len(out.DependsOn) != 3 {
		t.Errorf("DependsOn len = %d, want 3", len(out.DependsOn))
	}
}

func TestSubTaskEmptyDependsOn(t *testing.T) {
	in := SubTask{
		ID:           "sub-1",
		Title:        "standalone",
		Description:  "no deps",
		SuggestBreed: "jinmao",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out SubTask
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.DependsOn) != 0 {
		t.Errorf("DependsOn should be empty, got %v", out.DependsOn)
	}
}

func TestDispatchPlanEmptyEntries(t *testing.T) {
	in := DispatchPlan{
		Entries:  nil,
		MaxDepth: 5,
		Total:    0,
		Skipped:  0,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out DispatchPlan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Entries) != 0 {
		t.Errorf("Entries should be empty, got %d", len(out.Entries))
	}
	if out.MaxDepth != 5 {
		t.Errorf("MaxDepth = %d, want 5", out.MaxDepth)
	}
}

func TestDispatchPlanWithSkipped(t *testing.T) {
	in := DispatchPlan{
		Entries: []DispatchEntry{
			{BreedID: "b1", SubTaskID: "sub-1", Status: "pending"},
			{BreedID: "b2", SubTaskID: "sub-2", Status: "skipped", SkipReason: "no capability"},
		},
		MaxDepth: 3,
		Total:    2,
		Skipped:  1,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out DispatchPlan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", out.Skipped)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("Entries len = %d, want 2", len(out.Entries))
	}
	if out.Entries[1].SkipReason != "no capability" {
		t.Errorf("SkipReason = %q", out.Entries[1].SkipReason)
	}
}

func TestSubTaskZeroValue(t *testing.T) {
	var st SubTask
	if st.ID != "" || st.Title != "" || st.Description != "" || st.SuggestBreed != "" {
		t.Error("zero value SubTask should have all empty string fields")
	}
	if st.DependsOn != nil {
		t.Error("zero value DependsOn should be nil")
	}
}

func TestDispatchEntryZeroValue(t *testing.T) {
	var de DispatchEntry
	if de.BreedID != "" || de.SubTaskID != "" || de.Status != "" {
		t.Error("zero value DispatchEntry should have all empty string fields")
	}
	if de.Priority != 0 {
		t.Error("zero value Priority should be 0")
	}
	if de.DependsOn != nil {
		t.Error("zero value DependsOn should be nil")
	}
}
