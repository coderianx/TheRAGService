// Package httpx provides shared HTTP response helpers so that all handlers
// and middlewares return a consistent JSON shape across the API.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse is the standard error body returned by the API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("[ERROR] Write JSON response error: ", err)
	}
}

// WriteError writes the standard error response with the given status code.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}
