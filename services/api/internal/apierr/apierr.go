// Package apierr writes the API's single error shape: {code, message, details}.
package apierr

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func Write(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, Error{Code: code, Message: message})
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
