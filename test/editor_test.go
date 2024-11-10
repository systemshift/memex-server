package test

import (
	"testing"

	"memex/internal/memex"
)

// TestEditor tests the non-interactive parts of the editor
func TestEditor(t *testing.T) {
	t.Run("New Editor", func(t *testing.T) {
		editor := memex.NewEditor()
		if editor == nil {
			t.Fatal("NewEditor returned nil")
		}
	})

	// Note: Most editor functionality is interactive and requires terminal input/output
	// For comprehensive testing, we would need:
	// 1. Mock terminal input/output
	// 2. Simulate keystrokes
	// 3. Capture screen output
	// These would be integration tests rather than unit tests
}

// TestEditorHelpers tests the helper functions
func TestEditorHelpers(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		wantCtrl byte
		isCntrl  bool
	}{
		{
			name:     "Regular character",
			input:    'a',
			wantCtrl: 0x01, // Ctrl-A
			isCntrl:  false,
		},
		{
			name:     "Control character",
			input:    0x01, // Ctrl-A
			wantCtrl: 0x01,
			isCntrl:  true,
		},
		{
			name:     "Space",
			input:    ' ',
			wantCtrl: 0x00,
			isCntrl:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ctrl function
			if got := memex.Ctrl(tt.input); got != tt.wantCtrl {
				t.Errorf("ctrl(%v) = %v, want %v", tt.input, got, tt.wantCtrl)
			}

			// Test iscntrl function
			if got := memex.IsCntrl(tt.input); got != tt.isCntrl {
				t.Errorf("iscntrl(%v) = %v, want %v", tt.input, got, tt.isCntrl)
			}
		})
	}
}
