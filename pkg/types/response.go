package types

// ResponseStatus represents the status of a response
type ResponseStatus string

const (
	// StatusSuccess indicates a successful response
	StatusSuccess ResponseStatus = "success"
	// StatusError indicates an error response
	StatusError ResponseStatus = "error"
)

// Response represents a standard response format
type Response struct {
	Status ResponseStatus `json:"status"`          // Response status (success/error)
	Data   interface{}    `json:"data,omitempty"`  // Response data (for success)
	Error  string         `json:"error,omitempty"` // Error message (for errors)
	Meta   interface{}    `json:"meta,omitempty"`  // Optional metadata
}
