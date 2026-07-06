package main

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleSchema = `{
  "flavor": "demo", "version": 1,
  "options": [
    {"key": "name", "label": "Name", "type": "string", "required": true},
    {"key": "count", "label": "Count", "type": "number", "min": 1, "max": 5, "integer": true},
    {"key": "use_extra", "label": "Extra", "type": "bool", "default": false},
    {"key": "extra_kind", "label": "Kind", "type": "enum-one", "enum": ["a","b"], "showIf": {"use_extra": true}},
    {"key": "order", "label": "Order", "type": "enum-list", "enum": ["x","y","z"]}
  ]
}`

func writeSchema(t *testing.T, body string) *FlavorSchema {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, flavorSchemaFile), body)
	s, err := parseFlavorSchema(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return s
}

func TestParseSchema(t *testing.T) {
	s := writeSchema(t, sampleSchema)
	if len(s.Options) != 5 {
		t.Fatalf("want 5 options, got %d", len(s.Options))
	}
	if s.Options[1].Min == nil || *s.Options[1].Min != 1 {
		t.Errorf("min not parsed")
	}
}

func TestValidation(t *testing.T) {
	s := writeSchema(t, sampleSchema)
	v := s.defaults()
	v["name"] = "" // required -> should fail
	if err := s.validateInput(v); err == nil {
		t.Error("empty required name should fail")
	}
	v["name"] = "ok"
	v["count"] = float64(10) // > max
	if err := s.validateInput(v); err == nil {
		t.Error("count over max should fail")
	}
	v["count"] = float64(2.5) // not integer
	if err := s.validateInput(v); err == nil {
		t.Error("non-integer count should fail")
	}
	v["count"] = float64(3)
	if err := s.validateInput(v); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
}

func TestShowIfHidesAndSkips(t *testing.T) {
	s := writeSchema(t, sampleSchema)
	kind := s.Options[3] // extra_kind, showIf use_extra==true
	if kind.visible(map[string]any{"use_extra": false}) {
		t.Error("extra_kind should be hidden when use_extra=false")
	}
	if !kind.visible(map[string]any{"use_extra": true}) {
		t.Error("extra_kind should be visible when use_extra=true")
	}
	// A hidden option with an otherwise-invalid value must not block submission.
	v := s.defaults()
	v["name"] = "ok"
	v["count"] = float64(3) // valid, so only extra_kind's visibility is under test
	v["use_extra"] = false
	v["extra_kind"] = "not-in-enum" // invalid, but hidden -> skipped
	if err := s.validateInput(v); err != nil {
		t.Errorf("hidden invalid option should be skipped, got: %v", err)
	}
	// When visible, the same bad value must fail.
	v["use_extra"] = true
	if err := s.validateInput(v); err == nil {
		t.Error("visible invalid enum value should fail")
	}
}

func TestOrderedListValidation(t *testing.T) {
	s := writeSchema(t, sampleSchema)
	order := s.Options[4]
	if err := order.validateValue([]string{"x", "z"}); err != nil {
		t.Errorf("valid subset rejected: %v", err)
	}
	if err := order.validateValue([]string{"x", "nope"}); err == nil {
		t.Error("non-enum member should fail")
	}
}

func TestReorderHelper(t *testing.T) {
	got := reorder([]string{"a", "b", "c"}, "c", -1)
	if got[1] != "c" || got[2] != "b" {
		t.Errorf("reorder up failed: %v", got)
	}
	// out-of-range move is a no-op
	same := reorder([]string{"a", "b"}, "a", -1)
	if same[0] != "a" {
		t.Errorf("out-of-range reorder should no-op: %v", same)
	}
}

func TestSchemaFileMissing(t *testing.T) {
	if _, err := parseFlavorSchema(t.TempDir()); !os.IsNotExist(err) {
		t.Errorf("missing schema should return not-exist, got %v", err)
	}
}
