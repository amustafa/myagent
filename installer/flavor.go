package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// A flavorable skill ships two files beside its SKILL.md:
//   - flavor.json  — the option schema (parsed here)
//   - install.py   — the renderer the installer runs to materialize a flavor
// Presence of install.py is what flags a component as flavorable (scan.go).

const (
	flavorSchemaFile = "flavor.json"
	flavorRenderFile = "install.py"
)

// OptType enumerates the widget/value kinds a flavor option can take.
type OptType string

const (
	OptString       OptType = "string"         // free text            -> string
	OptNumber       OptType = "number"         // numeric (min/max)    -> float64
	OptBool         OptType = "bool"           // toggle               -> bool
	OptPath         OptType = "path"           // filesystem path      -> string
	OptEnumOne      OptType = "enum-one"       // pick one of enum     -> string
	OptEnumOrCustom OptType = "enum-or-custom" // pick one OR type own -> string
	OptEnumSet      OptType = "enum-set"       // unordered subset     -> []string
	OptEnumList     OptType = "enum-list"      // ordered subset       -> []string
)

// FlavorOption is one configurable field in a flavor schema.
type FlavorOption struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Type        OptType         `json:"type"`
	Help        string          `json:"help,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Enum        []string        `json:"enum,omitempty"`
	ShowIf      map[string]any  `json:"showIf,omitempty"` // option is visible only when every key==value holds

	// numeric/string validation
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Integer bool     `json:"integer,omitempty"`
	Regex   string   `json:"regex,omitempty"`
}

// FlavorSchema is a parsed flavor.json.
type FlavorSchema struct {
	Flavor  string         `json:"flavor"`
	Version int            `json:"version"`
	Options []FlavorOption `json:"options"`
}

// parseFlavorSchema loads and lightly validates the flavor.json in skillDir.
func parseFlavorSchema(skillDir string) (*FlavorSchema, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, flavorSchemaFile))
	if err != nil {
		return nil, err
	}
	var s FlavorSchema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", flavorSchemaFile, err)
	}
	seen := map[string]bool{}
	for i, o := range s.Options {
		if o.Key == "" {
			return nil, fmt.Errorf("option %d: missing key", i)
		}
		if seen[o.Key] {
			return nil, fmt.Errorf("duplicate option key %q", o.Key)
		}
		seen[o.Key] = true
		if o.enumBacked() && len(o.Enum) == 0 {
			return nil, fmt.Errorf("option %q is %s but has no enum", o.Key, o.Type)
		}
	}
	return &s, nil
}

// enumBacked reports whether the type draws from the Enum list.
func (o FlavorOption) enumBacked() bool {
	switch o.Type {
	case OptEnumOne, OptEnumOrCustom, OptEnumSet, OptEnumList:
		return true
	}
	return false
}

// isMulti reports whether the value is a []string (set/list) vs a scalar.
func (o FlavorOption) isMulti() bool {
	return o.Type == OptEnumSet || o.Type == OptEnumList
}

// defaultValue decodes the schema default into the option's canonical Go type,
// falling back to a sensible zero when no default is declared.
func (o FlavorOption) defaultValue() any {
	if len(o.Default) > 0 {
		if v, ok := o.decode(o.Default); ok {
			return v
		}
	}
	switch {
	case o.isMulti():
		return []string{}
	case o.Type == OptBool:
		return false
	case o.Type == OptNumber:
		return float64(0)
	default:
		return ""
	}
}

// decode converts raw JSON into the option's canonical Go type.
func (o FlavorOption) decode(raw json.RawMessage) (any, bool) {
	switch {
	case o.isMulti():
		var xs []string
		if json.Unmarshal(raw, &xs) == nil {
			return xs, true
		}
	case o.Type == OptBool:
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return b, true
		}
	case o.Type == OptNumber:
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return f, true
		}
	default:
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s, true
		}
	}
	return nil, false
}

// visible evaluates an option's showIf against the current value set. An option
// with no showIf is always visible; otherwise every key must equal its target.
func (o FlavorOption) visible(values map[string]any) bool {
	for k, want := range o.ShowIf {
		if !equalLoose(values[k], want) {
			return false
		}
	}
	return true
}

// equalLoose compares values across the JSON/Go type gap (e.g. a bool target
// decoded from showIf vs a bool value; numbers as float64).
func equalLoose(got, want any) bool {
	switch w := want.(type) {
	case bool:
		g, ok := got.(bool)
		return ok && g == w
	case string:
		g, ok := got.(string)
		return ok && g == w
	case float64:
		if g, ok := got.(float64); ok {
			return g == w
		}
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}

// validateValue checks a single option's value against its type and constraints.
func (o FlavorOption) validateValue(v any) error {
	switch {
	case o.isMulti():
		xs, ok := v.([]string)
		if !ok {
			return fmt.Errorf("%s: expected a list", o.Label)
		}
		if o.Required && len(xs) == 0 {
			return fmt.Errorf("%s: pick at least one", o.Label)
		}
		if o.Type == OptEnumSet || o.Type == OptEnumList {
			for _, x := range xs {
				if !contains(o.Enum, x) {
					return fmt.Errorf("%s: %q is not an allowed value", o.Label, x)
				}
			}
		}
	case o.Type == OptBool:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: expected true/false", o.Label)
		}
	case o.Type == OptNumber:
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s: expected a number", o.Label)
		}
		if o.Integer && f != float64(int64(f)) {
			return fmt.Errorf("%s: must be a whole number", o.Label)
		}
		if o.Min != nil && f < *o.Min {
			return fmt.Errorf("%s: must be ≥ %g", o.Label, *o.Min)
		}
		if o.Max != nil && f > *o.Max {
			return fmt.Errorf("%s: must be ≤ %g", o.Label, *o.Max)
		}
	default: // string-ish
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: expected text", o.Label)
		}
		if o.Required && s == "" {
			return fmt.Errorf("%s: required", o.Label)
		}
		if o.Type == OptEnumOne && s != "" && !contains(o.Enum, s) {
			return fmt.Errorf("%s: %q is not an allowed value", o.Label, s)
		}
		if o.Regex != "" && s != "" {
			re, err := regexp.Compile(o.Regex)
			if err != nil {
				return fmt.Errorf("%s: bad regex in schema: %v", o.Label, err)
			}
			if !re.MatchString(s) {
				return fmt.Errorf("%s: %q doesn't match %s", o.Label, s, o.Regex)
			}
		}
	}
	return nil
}

// validateInput validates every *visible* option's value; hidden options (failing
// their showIf) are skipped so a disabled field can't block submission.
func (s *FlavorSchema) validateInput(values map[string]any) error {
	for _, o := range s.Options {
		if !o.visible(values) {
			continue
		}
		v, present := values[o.Key]
		if !present {
			v = o.defaultValue()
		}
		if err := o.validateValue(v); err != nil {
			return err
		}
	}
	return nil
}

// defaults returns a fresh value map seeded from every option's default.
func (s *FlavorSchema) defaults() map[string]any {
	out := make(map[string]any, len(s.Options))
	for _, o := range s.Options {
		out[o.Key] = o.defaultValue()
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
