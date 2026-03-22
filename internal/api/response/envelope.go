// Package response provides JSON response helpers following the standard
// envelope format: { "data": ..., "error": ..., "meta": ... }.
package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the standard JSON response wrapper.
type Envelope struct {
	Data  any         `json:"data"`
	Error *ErrorBody  `json:"error"`
	Meta  *MetaBody   `json:"meta,omitempty"`
}

// ErrorBody provides machine-readable and human-readable error details.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MetaBody carries pagination or supplementary info.
type MetaBody struct {
	Total int `json:"total,omitempty"`
}

// JSON writes a success response with the given data.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Envelope{Data: data})
}

// Error writes an error response with the given code and message.
func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Envelope{
		Error: &ErrorBody{Code: code, Message: message},
	})
}

// NotFound writes a 404 error response.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, "not_found", message)
}

// BadRequest writes a 400 error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, "bad_request", message)
}

// InternalError writes a 500 error response.
func InternalError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, "internal_error", message)
}
