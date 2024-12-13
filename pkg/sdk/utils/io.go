package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"memex/pkg/sdk/types"
)

// ReadInput reads and parses JSON input
func ReadInput(r io.Reader) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		if err == io.EOF {
			return nil, nil // No input is valid
		}
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return data, nil
}

// WriteOutput writes a response as JSON
func WriteOutput(w io.Writer, resp types.Response) error {
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// WriteError writes an error response
func WriteError(w io.Writer, err error) error {
	resp := types.Response{
		Status: types.StatusError,
		Error:  err.Error(),
	}
	return WriteOutput(w, resp)
}

// RunModule runs a module with the given command handler
func RunModule(handler types.CommandHandler) error {
	// Read command from stdin
	input, err := ReadInput(os.Stdin)
	if err != nil {
		return WriteError(os.Stderr, err)
	}

	// Parse command
	cmd := types.Command{
		Name:    input["command"].(string),
		Args:    parseArgs(input["args"]),
		Input:   os.Stdin,
		Output:  os.Stdout,
		Error:   os.Stderr,
		Context: context.Background(),
	}

	// Handle command
	resp := handler.Handle(cmd)

	// Write response
	return WriteOutput(os.Stdout, resp)
}

// Helper function to parse command arguments
func parseArgs(args interface{}) []string {
	if args == nil {
		return nil
	}

	switch v := args.(type) {
	case []interface{}:
		result := make([]string, len(v))
		for i, arg := range v {
			result[i] = fmt.Sprint(arg)
		}
		return result
	case string:
		return []string{v}
	default:
		return []string{fmt.Sprint(args)}
	}
}
