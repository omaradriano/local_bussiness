package steps

import (
	"encoding/json"
	"testing"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

// Contract every registered type must honor: a sane config is valid, an empty submission
// is rejected, and a complete one is accepted.
func TestEveryTypeHonorsTheContract(t *testing.T) {
	good := map[string]struct {
		config string
		ok     Submission
	}{
		"photo":       {`{"bucket":"rental-evidence","max_files":3}`, Submission{Files: []string{"k1"}}},
		"file_upload": {`{"bucket":"private-docs"}`, Submission{Files: []string{"k1"}}},
		"text":        {`{}`, Submission{Data: raw(`{"text":"Tonos 21 y 34"}`)}},
		"number":      {`{"min":0,"max":100}`, Submission{Data: raw(`{"value":42}`)}},
		"checklist":   {`{"items":["Broca","Cable"]}`, Submission{Data: raw(`{"checked":{"Broca":true,"Cable":true}}`)}},
	}
	for key := range registry {
		c, ok := good[key]
		if !ok {
			t.Fatalf("type %q registered without contract cases", key)
		}
		st, _ := Lookup(key)
		if err := st.ValidateConfig(raw(c.config)); err != nil {
			t.Errorf("%s: valid config rejected: %v", key, err)
		}
		if err := st.ValidateData(raw(c.config), Submission{}); err == nil {
			t.Errorf("%s: empty submission accepted", key)
		}
		if err := st.ValidateData(raw(c.config), c.ok); err != nil {
			t.Errorf("%s: complete submission rejected: %v", key, err)
		}
	}
}

func TestFilesLimitsAndBuckets(t *testing.T) {
	photo, _ := Lookup("photo")
	cfg := raw(`{"bucket":"private-docs","max_files":2}`)
	if photo.Bucket(cfg) != "private-docs" || photo.Bucket(raw(`{}`)) != "rental-evidence" {
		t.Fatal("bucket from config or default")
	}
	if err := photo.ValidateData(cfg, Submission{Files: []string{"a", "b", "c"}}); err == nil {
		t.Fatal("over max_files accepted")
	}
	if err := photo.ValidateConfig(raw(`{"bucket":"catalog"}`)); err == nil {
		t.Fatal("public catalog bucket accepted for private evidence")
	}
}

func TestChecklistNeedsEveryItem(t *testing.T) {
	k, _ := Lookup("checklist")
	cfg := raw(`{"items":["Broca","Cable"]}`)
	if err := k.ValidateData(cfg, Submission{Data: raw(`{"checked":{"Broca":true}}`)}); err == nil || err.Error() != "Falta revisar: Cable." {
		t.Fatalf("got %v", err)
	}
	if err := k.ValidateConfig(raw(`{}`)); err == nil {
		t.Fatal("checklist without items accepted")
	}
}

func TestNumberRange(t *testing.T) {
	n, _ := Lookup("number")
	if err := n.ValidateData(raw(`{"min":0,"max":10}`), Submission{Data: raw(`{"value":11}`)}); err == nil {
		t.Fatal("out of range accepted")
	}
}

func TestUnknownType(t *testing.T) {
	if _, ok := Lookup("signature"); ok {
		t.Fatal("signature is not implemented yet")
	}
}
