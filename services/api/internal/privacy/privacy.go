// Package privacy holds the version of the privacy notices (the platform's and each business's).
// The text lives in the app (packages/shared); privacy.json is read by both, so a customer's
// acceptance always records the version the app showed.
package privacy

import (
	_ "embed"
	"encoding/json"
)

//go:embed privacy.json
var raw []byte

// Version of the current notices (YYYY-MM-DD); change it whenever their text changes.
var Version = func() string {
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &v); err != nil || v.Version == "" {
		panic("privacy.json: missing version")
	}
	return v.Version
}()
