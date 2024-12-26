package sdk

import (
	"encoding/json"
	"io"

	"memex/pkg/types"
)

// ReadInput reads and parses input from a reader
func ReadInput(r io.Reader) (map[string]interface{}, error) {
	if r == nil {
		return nil, nil
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r).Decode(&data); err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

// WriteOutput writes a response to a writer
func WriteOutput(w io.Writer, resp types.Response) error {
	if w == nil {
		return nil
	}
	return json.NewEncoder(w).Encode(resp)
}

// WriteError writes an error response to a writer
func WriteError(w io.Writer, err error) error {
	if w == nil {
		return nil
	}
	resp := types.Response{
		Status: types.StatusError,
		Error:  err.Error(),
	}
	return WriteOutput(w, resp)
}
