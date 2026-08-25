package capability

import "testing"

func TestCapabilityTip_Validate(t *testing.T) {
	good := CapabilityTip{ID: "t1", Capability: "memory", Message: "use recall", SourceRef: "doc:ft-mem-001"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid tip rejected: %v", err)
	}

	cases := map[string]CapabilityTip{
		"missing sourceRef": {ID: "t", Capability: "c", Message: "m"},
		"malformed sourceRef": {ID: "t", Capability: "c", Message: "m", SourceRef: "no-colon"},
		"empty id":          {Capability: "c", Message: "m", SourceRef: "doc:x"},
	}
	for name, tip := range cases {
		if err := tip.Validate(); err == nil {
			t.Fatalf("case %q should fail closed", name)
		}
	}
}
