// Package steps is the registry of configurable step types (photo of the ID, checklist, notes…).
// Flows never check a step type by name: they ask the registry. Adding a type = implement StepType,
// register it here, define its schemas in packages/shared and its renderer in the app (see CLAUDE.md).
package steps

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// Submission is what a person sends to complete a step.
type Submission struct {
	Data  json.RawMessage `json:"data"`  // type-specific
	Files []string        `json:"files"` // S3 keys already uploaded for this booking
}

type StepType interface {
	Key() string
	// ValidateConfig checks the step definition (workflow_steps.config).
	ValidateConfig(config json.RawMessage) error
	// ValidateData checks a submission against the config; returns a message for the user (es-MX).
	ValidateData(config json.RawMessage, s Submission) error
	// Bucket where its files go ("" = no files).
	Bucket(config json.RawMessage) string
}

var registry = map[string]StepType{}

func register(t StepType) { registry[t.Key()] = t }

func Lookup(key string) (StepType, bool) {
	t, ok := registry[key]
	return t, ok
}

// UserError is a validation problem worth showing to the person completing the step.
type UserError string

func (e UserError) Error() string { return string(e) }

func init() {
	register(files{key: "photo", defaultBucket: "rental-evidence"})
	register(files{key: "file_upload", defaultBucket: "private-docs"})
	register(text{})
	register(number{})
	register(checklist{})
}

// ---------------------------------------------------------------- photo / file_upload

type filesConfig struct {
	Bucket   string `json:"bucket"`
	MinFiles int    `json:"min_files"`
	MaxFiles int    `json:"max_files"`
}

var buckets = []string{"private-docs", "rental-evidence"}

type files struct{ key, defaultBucket string }

func (f files) Key() string { return f.key }

func (f files) cfg(raw json.RawMessage) filesConfig {
	c := filesConfig{Bucket: f.defaultBucket, MinFiles: 1, MaxFiles: 6}
	_ = json.Unmarshal(raw, &c)
	if c.MinFiles < 1 {
		c.MinFiles = 1
	}
	if c.MaxFiles < c.MinFiles {
		c.MaxFiles = c.MinFiles
	}
	return c
}

func (f files) ValidateConfig(raw json.RawMessage) error {
	if c := f.cfg(raw); !slices.Contains(buckets, c.Bucket) {
		return fmt.Errorf("bucket %q not allowed", c.Bucket)
	}
	return nil
}

func (f files) ValidateData(raw json.RawMessage, s Submission) error {
	c := f.cfg(raw)
	switch n := len(s.Files); {
	case n < c.MinFiles:
		return UserError(fmt.Sprintf("Agrega al menos %d foto(s).", c.MinFiles))
	case n > c.MaxFiles:
		return UserError(fmt.Sprintf("Máximo %d foto(s).", c.MaxFiles))
	}
	return nil
}

func (f files) Bucket(raw json.RawMessage) string { return f.cfg(raw).Bucket }

// ---------------------------------------------------------------- text

type text struct{}

func (text) Key() string                          { return "text" }
func (text) ValidateConfig(json.RawMessage) error { return nil }
func (text) Bucket(json.RawMessage) string        { return "" }

func (text) ValidateData(_ json.RawMessage, s Submission) error {
	var d struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(s.Data, &d) != nil || strings.TrimSpace(d.Text) == "" {
		return UserError("Escribe el texto.")
	}
	if utf8.RuneCountInString(d.Text) > 2000 {
		return UserError("El texto es muy largo.")
	}
	return nil
}

// ---------------------------------------------------------------- number

type number struct{}

func (number) Key() string                          { return "number" }
func (number) ValidateConfig(json.RawMessage) error { return nil }
func (number) Bucket(json.RawMessage) string        { return "" }

func (number) ValidateData(raw json.RawMessage, s Submission) error {
	var c struct{ Min, Max *float64 }
	_ = json.Unmarshal(raw, &c)
	var d struct {
		Value *float64 `json:"value"`
	}
	if json.Unmarshal(s.Data, &d) != nil || d.Value == nil {
		return UserError("Escribe un número.")
	}
	if (c.Min != nil && *d.Value < *c.Min) || (c.Max != nil && *d.Value > *c.Max) {
		return UserError("El número está fuera del rango permitido.")
	}
	return nil
}

// ---------------------------------------------------------------- checklist

type checklist struct{}

func (checklist) Key() string                   { return "checklist" }
func (checklist) Bucket(json.RawMessage) string { return "" }

func (checklist) items(raw json.RawMessage) []string {
	var c struct {
		Items []string `json:"items"`
	}
	_ = json.Unmarshal(raw, &c)
	return c.Items
}

func (k checklist) ValidateConfig(raw json.RawMessage) error {
	if len(k.items(raw)) == 0 {
		return fmt.Errorf("checklist needs items")
	}
	return nil
}

// Every item must be checked (true).
func (k checklist) ValidateData(raw json.RawMessage, s Submission) error {
	var d struct {
		Checked map[string]bool `json:"checked"`
	}
	_ = json.Unmarshal(s.Data, &d)
	for _, it := range k.items(raw) {
		if !d.Checked[it] {
			return UserError(fmt.Sprintf("Falta revisar: %s.", it))
		}
	}
	return nil
}
