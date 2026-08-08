package telemetry

import "testing"

func TestInit(t *testing.T) {
	cleanup, err := Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer cleanup()
	if !IsInitialized() {
		t.Error("IsInitialized = false after Init")
	}
}

func TestInit_Idempotent(t *testing.T) {
	cleanup1, _ := Init()
	cleanup2, _ := Init()
	defer cleanup1()
	defer cleanup2()
}

func TestStartTime(t *testing.T) {
	st := StartTime()
	if st.IsZero() {
		t.Error("StartTime is zero")
	}
}
