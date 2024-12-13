package test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"memex/pkg/sdk/module"
	"memex/pkg/sdk/types"
	"memex/pkg/sdk/utils"
)

// Test module implementation
type testModule struct {
	*module.BaseModule
}

func newTestModule() *testModule {
	return &testModule{
		BaseModule: module.NewBaseModule("test", "Test Module", "A test module"),
	}
}

func TestCommandHandler(t *testing.T) {
	mod := newTestModule()
	handler := module.NewBaseHandler(mod)

	tests := []struct {
		name     string
		cmd      types.Command
		wantCode int
		wantErr  bool
	}{
		{
			name: "get id",
			cmd: types.Command{
				Name: types.CmdID,
			},
			wantCode: types.StatusSuccess,
		},
		{
			name: "get name",
			cmd: types.Command{
				Name: types.CmdName,
			},
			wantCode: types.StatusSuccess,
		},
		{
			name: "get description",
			cmd: types.Command{
				Name: types.CmdDescription,
			},
			wantCode: types.StatusSuccess,
		},
		{
			name: "get help",
			cmd: types.Command{
				Name: types.CmdHelp,
			},
			wantCode: types.StatusSuccess,
		},
		{
			name: "unknown command",
			cmd: types.Command{
				Name: "unknown",
			},
			wantCode: types.StatusUnsupported,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handler.Handle(tt.cmd)

			if resp.Status != tt.wantCode {
				t.Errorf("got status %d, want %d", resp.Status, tt.wantCode)
			}

			if tt.wantErr && resp.Error == "" {
				t.Error("expected error message")
			}

			if !tt.wantErr && resp.Error != "" {
				t.Errorf("unexpected error: %s", resp.Error)
			}

			// Check specific responses
			switch tt.cmd.Name {
			case types.CmdID:
				if resp.Data != "test" {
					t.Errorf("got ID %q, want %q", resp.Data, "test")
				}
			case types.CmdName:
				if resp.Data != "Test Module" {
					t.Errorf("got name %q, want %q", resp.Data, "Test Module")
				}
			case types.CmdDescription:
				if resp.Data != "A test module" {
					t.Errorf("got description %q, want %q", resp.Data, "A test module")
				}
			}
		})
	}
}

func TestIOUtils(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid json",
			input:   `{"command": "test", "args": ["arg1", "arg2"]}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{"command": "test", "args": ["arg1", "arg2"]`, // Missing closing brace
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewBufferString(tt.input)
			data, err := utils.ReadInput(r)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.input != "" && data == nil {
					t.Error("expected data, got nil")
				}
			}
		})
	}
}

func TestModuleExecution(t *testing.T) {
	mod := newTestModule()
	handler := module.NewBaseHandler(mod)

	// Test command execution
	cmd := types.Command{
		Name:    types.CmdHelp,
		Args:    []string{"arg1", "arg2"},
		Input:   &bytes.Buffer{},
		Output:  &bytes.Buffer{},
		Error:   &bytes.Buffer{},
		Context: context.Background(),
	}

	resp := handler.Handle(cmd)
	if resp.Status != types.StatusSuccess {
		t.Errorf("command execution failed: %s", resp.Error)
	}

	// Test response writing
	var buf bytes.Buffer
	if err := utils.WriteOutput(&buf, resp); err != nil {
		t.Errorf("writing response: %v", err)
	}

	// Verify response can be parsed back
	var parsed types.Response
	if err := json.NewDecoder(&buf).Decode(&parsed); err != nil {
		t.Errorf("parsing response: %v", err)
	}

	if parsed.Status != resp.Status {
		t.Errorf("got status %d, want %d", parsed.Status, resp.Status)
	}
}
