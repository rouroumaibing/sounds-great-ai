package unified

import (
	"strings"
	"testing"
)

func TestParseNDJSONValid(t *testing.T) {
	input := `{"type":"text","content":"hello"}
{"type":"done"}`
	ch := ParseNDJSON(strings.NewReader(input))
	var objs []map[string]any
	var errors []ParseError
	for evt := range ch {
		if IsParseError(evt) {
			errors = append(errors, evt.(ParseError))
			continue
		}
		objs = append(objs, evt.(map[string]any))
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected parse errors: %+v", errors)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
	if objs[0]["type"] != "text" {
		t.Errorf("first object type = %v, want text", objs[0]["type"])
	}
}

func TestParseNDJSONBlankLines(t *testing.T) {
	input := "{\"a\":1}\n\n{\"b\":2}\n"
	ch := ParseNDJSON(strings.NewReader(input))
	count := 0
	for evt := range ch {
		if !IsParseError(evt) {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 objects (blank lines skipped), got %d", count)
	}
}

func TestParseNDJSONParseErrorNotPanic(t *testing.T) {
	input := `{"valid":1}
this is not json
{"also_valid":2}`
	ch := ParseNDJSON(strings.NewReader(input))
	var objs []map[string]any
	var parseErrors []ParseError
	for evt := range ch {
		if IsParseError(evt) {
			parseErrors = append(parseErrors, evt.(ParseError))
			continue
		}
		objs = append(objs, evt.(map[string]any))
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 valid objects, got %d", len(objs))
	}
	if len(parseErrors) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(parseErrors))
	}
	if !strings.Contains(parseErrors[0].Line, "this is not json") {
		t.Errorf("parse error line = %q", parseErrors[0].Line)
	}
}

func TestParseNDJSONEmptyInput(t *testing.T) {
	ch := ParseNDJSON(strings.NewReader(""))
	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 objects for empty input, got %d", count)
	}
}

func TestIsParseError(t *testing.T) {
	pe := ParseError{Line: "bad", Error: "err"}
	if !IsParseError(pe) {
		t.Fatal("expected IsParseError=true for ParseError")
	}
	obj := map[string]any{"type": "text"}
	if IsParseError(obj) {
		t.Fatal("expected IsParseError=false for map")
	}
}
