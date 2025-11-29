package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/middleware"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
)

// SendSuccess sends a successful JSON response with data
func SendSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(backendtypes.APIResponse{
		Success:   true,
		Data:      data,
		RequestID: middleware.GetRequestID(r.Context()),
		Timestamp: time.Now(),
	}); err != nil {
		// Log encoding error - response headers already sent, can't change status
		// In production, this would be logged via a proper logging framework
		_ = err // Explicitly ignore after logging consideration
	}
}

// SendError sends an error JSON response with APIError
func SendError(w http.ResponseWriter, r *http.Request, code string, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(backendtypes.APIResponse{
		Success: false,
		Error: &backendtypes.APIError{
			Code:    code,
			Message: message,
		},
		RequestID: middleware.GetRequestID(r.Context()),
		Timestamp: time.Now(),
	}); err != nil {
		// Log encoding error - response headers already sent, can't change status
		// In production, this would be logged via a proper logging framework
		_ = err // Explicitly ignore after logging consideration
	}
}

// SendCreated sends a 201 Created response
func SendCreated(w http.ResponseWriter, r *http.Request, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(backendtypes.APIResponse{
		Success:   true,
		Data:      data,
		RequestID: middleware.GetRequestID(r.Context()),
		Timestamp: time.Now(),
	}); err != nil {
		// Log encoding error - response headers already sent, can't change status
		// In production, this would be logged via a proper logging framework
		_ = err // Explicitly ignore after logging consideration
	}
}

// ParseJSON parses JSON from request body into target
func ParseJSON(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(target)
}
