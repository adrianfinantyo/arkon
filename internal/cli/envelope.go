package cli

import (
	"encoding/json"
	"io"
)

// Envelope defines the standard typed JSON envelope for all Arkon CLI outputs.
type Envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Error   *Error `json:"error"`
}

// Error defines structured machine-readable error details.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// WriteJSON emits an Envelope to the given writer with standard two-space indentation.
func WriteJSON(w io.Writer, success bool, data any, err *Error) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(Envelope{
		Success: success,
		Data:    data,
		Error:   err,
	})
}
