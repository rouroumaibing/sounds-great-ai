package governance

import (
	"errors"
	"testing"
)

func TestCostLedger_AuthorizeFailClosed(t *testing.T) {
	l := NewCostLedger()
	// no quota -> deny
	if ok, err := l.AuthorizeSpend("bianmu", 10); ok || !errors.Is(err, ErrNoQuota) {
		t.Fatalf("missing quota must deny: ok=%v err=%v", ok, err)
	}
	l.SetQuota(Quota{DogID: "bianmu", Limit: 100})
	if ok, err := l.AuthorizeSpend("bianmu", 10); !ok || err != nil {
		t.Fatalf("within quota must allow: %v", err)
	}
}

func TestCostLedger_Exceeds(t *testing.T) {
	l := NewCostLedger()
	l.SetQuota(Quota{DogID: "jinmao", Limit: 50})
	l.Record("jinmao", 40)
	if ok, err := l.AuthorizeSpend("jinmao", 5); !ok || err != nil {
		t.Fatalf("45/50 should allow: %v", err)
	}
	if ok, err := l.AuthorizeSpend("jinmao", 20); ok || !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("60/50 must deny: ok=%v err=%v", ok, err)
	}
}

func TestCostLedger_NonPositiveLimitDenies(t *testing.T) {
	l := NewCostLedger()
	l.SetQuota(Quota{DogID: "x", Limit: 0}) // ungoverned
	if ok, err := l.AuthorizeSpend("x", 1); ok || !errors.Is(err, ErrNoQuota) {
		t.Fatalf("non-positive limit must deny: ok=%v err=%v", ok, err)
	}
}

func TestCostLedger_Dashboard(t *testing.T) {
	l := NewCostLedger()
	l.SetQuota(Quota{DogID: "a", Limit: 100})
	l.SetQuota(Quota{DogID: "b", Limit: 50})
	l.Record("a", 50)
	l.Record("b", 50) // at limit
	rows := l.Dashboard()
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	for _, r := range rows {
		switch r.DogID {
		case "a":
			if r.Ratio != 0.5 || r.Exceeds {
				t.Fatalf("a row wrong: %+v", r)
			}
		case "b":
			if !r.Exceeds {
				t.Fatalf("b should exceed: %+v", r)
			}
		}
	}
}

func TestCostLedger_UsedTracks(t *testing.T) {
	l := NewCostLedger()
	l.SetQuota(Quota{DogID: "d", Limit: 1000})
	l.Record("d", 10)
	l.Record("d", 5)
	if l.Used("d") != 15 {
		t.Fatalf("used = %d", l.Used("d"))
	}
}
