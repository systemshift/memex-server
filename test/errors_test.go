package test

import (
	"fmt"
	"testing"

	"memex/pkg/sdk"
	"memex/pkg/types"
)

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name        string
		makeResp    func() types.Response
		wantStatus  types.ResponseStatus
		wantError   string
		wantSuccess bool
	}{
		{
			name: "error response",
			makeResp: func() types.Response {
				return sdk.ErrorResponse(fmt.Errorf("test error"))
			},
			wantStatus:  types.StatusError,
			wantError:   "test error",
			wantSuccess: false,
		},
		{
			name: "success response",
			makeResp: func() types.Response {
				return sdk.SuccessResponse("test data")
			},
			wantStatus:  types.StatusSuccess,
			wantError:   "",
			wantSuccess: true,
		},
		{
			name: "not found response",
			makeResp: func() types.Response {
				return sdk.NotFoundResponse("test item")
			},
			wantStatus:  types.StatusError,
			wantError:   "not found: test item",
			wantSuccess: false,
		},
		{
			name: "invalid input response",
			makeResp: func() types.Response {
				return sdk.InvalidInputResponse("missing field")
			},
			wantStatus:  types.StatusError,
			wantError:   "invalid input: missing field",
			wantSuccess: false,
		},
		{
			name: "not supported response",
			makeResp: func() types.Response {
				return sdk.NotSupportedResponse("test operation")
			},
			wantStatus:  types.StatusError,
			wantError:   "not supported: test operation",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.makeResp()

			if resp.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", resp.Status, tt.wantStatus)
			}
			if resp.Error != tt.wantError {
				t.Errorf("Error = %v, want %v", resp.Error, tt.wantError)
			}
			if (resp.Status == types.StatusSuccess) != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", resp.Status == types.StatusSuccess, tt.wantSuccess)
			}
		})
	}
}

func TestErrorChecking(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		check     func(error) bool
		wantMatch bool
	}{
		{
			name:      "not found error",
			err:       fmt.Errorf("%w: test", sdk.ErrNotFound),
			check:     sdk.IsNotFound,
			wantMatch: true,
		},
		{
			name:      "unauthorized error",
			err:       fmt.Errorf("%w: test", sdk.ErrUnauthorized),
			check:     sdk.IsUnauthorized,
			wantMatch: true,
		},
		{
			name:      "invalid input error",
			err:       fmt.Errorf("%w: test", sdk.ErrInvalidInput),
			check:     sdk.IsInvalidInput,
			wantMatch: true,
		},
		{
			name:      "not supported error",
			err:       fmt.Errorf("%w: test", sdk.ErrNotSupported),
			check:     sdk.IsNotSupported,
			wantMatch: true,
		},
		{
			name:      "not initialized error",
			err:       fmt.Errorf("%w: test", sdk.ErrNotInitalized),
			check:     sdk.IsNotInitialized,
			wantMatch: true,
		},
		{
			name:      "different error",
			err:       fmt.Errorf("some other error"),
			check:     sdk.IsNotFound,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.check(tt.err); got != tt.wantMatch {
				t.Errorf("%v match = %v, want %v", tt.name, got, tt.wantMatch)
			}
		})
	}
}
