package memory

import (
	"path/filepath"
	"testing"
)

func TestSensitivityRankAndValid(t *testing.T) {
	if SensitivityRank("") != 0 || SensitivityRank("public") != 0 {
		t.Fatalf("public rank wrong")
	}
	if SensitivityRank("restricted") != 3 {
		t.Fatalf("restricted rank wrong")
	}
	if !ValidSensitivity("internal") || ValidSensitivity("bogus") {
		t.Fatalf("ValidSensitivity wrong")
	}
}

func TestEntryVisibleSensitivityClearance(t *testing.T) {
	// Reset to known state.
	SetOperatorClearance(map[string]int{"alice": 1}) // internal
	defer SetOperatorClearance(nil)
	SetCollectionGrants(nil)

	e := &LaneEntry{OperatorID: "", Sensitivity: "restricted"}
	if EntryVisible(e, "alice") {
		t.Fatalf("alice (internal) must NOT see restricted shared entry")
	}
	if !EntryVisible(e, "") {
		t.Fatalf("system scope must see restricted entry")
	}

	e2 := &LaneEntry{OperatorID: "", Sensitivity: "internal"}
	if !EntryVisible(e2, "alice") {
		t.Fatalf("alice should see internal shared entry")
	}
}

func TestEntryVisibleCollectionGrant(t *testing.T) {
	SetOperatorClearance(nil)
	SetCollectionGrants(map[string][]string{"secret": {"bob"}})
	defer SetCollectionGrants(nil)

	e := &LaneEntry{OperatorID: "carol", CollectionID: "secret", Sensitivity: "public"}
	if EntryVisible(e, "alice") {
		t.Fatalf("alice not granted to 'secret' collection must be hidden")
	}
	if !EntryVisible(e, "bob") {
		t.Fatalf("bob is granted to 'secret' and must see it")
	}
	if !EntryVisible(e, "") {
		t.Fatalf("system scope sees all")
	}
}

func TestSetSensitivityPersists(t *testing.T) {
	dir := t.TempDir()
	reg := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	defer reg.Close()
	ent := reg.Lane(LaneDecision).Submit("decide X", "s:1")
	if !reg.SetSensitivity(ent.ID, "private") {
		t.Fatalf("SetSensitivity failed")
	}
	if !EntryVisible(&LaneEntry{OperatorID: ent.OperatorID, Sensitivity: "private"}, "") {
		t.Fatalf("private should be visible to system")
	}
	// Round-trips to disk.
	reg.Close()
	re := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	defer re.Close()
	laneType, ok := re.FindLaneOf(ent.ID)
	if !ok {
		t.Fatalf("entry lost")
	}
	got, _ := re.Lane(laneType).Get(ent.ID)
	if got.Sensitivity != "private" {
		t.Fatalf("sensitivity not persisted: %q", got.Sensitivity)
	}
}
