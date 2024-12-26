package sdk

import (
	"errors"
	"fmt"

	"memex/pkg/types"
)

// Common error types
var (
	ErrNotFound      = fmt.Errorf("not found")
	ErrUnauthorized  = fmt.Errorf("unauthorized")
	ErrInvalidInput  = fmt.Errorf("invalid input")
	ErrNotSupported  = fmt.Errorf("not supported")
	ErrNotInitalized = fmt.Errorf("not initialized")
)

// ErrorResponse creates a standard error response
func ErrorResponse(err error) types.Response {
	return types.Response{
		Status: types.StatusError,
		Error:  err.Error(),
	}
}

// SuccessResponse creates a standard success response
func SuccessResponse(data interface{}) types.Response {
	return types.Response{
		Status: types.StatusSuccess,
		Data:   data,
	}
}

// NotFoundResponse creates a standard not found response
func NotFoundResponse(item string) types.Response {
	return ErrorResponse(fmt.Errorf("%w: %s", ErrNotFound, item))
}

// InvalidInputResponse creates a standard invalid input response
func InvalidInputResponse(reason string) types.Response {
	return ErrorResponse(fmt.Errorf("%w: %s", ErrInvalidInput, reason))
}

// NotSupportedResponse creates a standard not supported response
func NotSupportedResponse(operation string) types.Response {
	return ErrorResponse(fmt.Errorf("%w: %s", ErrNotSupported, operation))
}

// WithMeta adds metadata to a response
func WithMeta(resp types.Response, meta interface{}) types.Response {
	resp.Meta = meta
	return resp
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if an error is an unauthorized error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsInvalidInput checks if an error is an invalid input error
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsNotSupported checks if an error is a not supported error
func IsNotSupported(err error) bool {
	return errors.Is(err, ErrNotSupported)
}

// IsNotInitialized checks if an error is a not initialized error
func IsNotInitialized(err error) bool {
	return errors.Is(err, ErrNotInitalized)
}
