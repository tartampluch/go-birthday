package ui_test

import (
	"testing"

	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/test"
	"github.com/tartampluch/go-birthday/internal/ui"
)

func TestNumericalEntry_TypedRune(t *testing.T) {
	// Initialize the custom widget using Fyne's test infrastructure.
	entry := ui.NewNumericalEntry()
	window := test.NewWindow(entry)
	defer window.Close()

	tests := []struct {
		name     string
		input    rune
		accepted bool
	}{
		{"Digit_Zero", '0', true},
		{"Digit_Nine", '9', true},
		{"Digit_Five", '5', true},
		{"Letter_a", 'a', false},
		{"Letter_Z", 'Z', false},
		{"Symbol_Dash", '-', false},
		{"Symbol_Space", ' ', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous content
			entry.SetText("")

			// Simulate typing
			test.Type(entry, string(tt.input))

			got := entry.Text
			if tt.accepted {
				if got != string(tt.input) {
					t.Errorf("expected input %q to be accepted, got text %q", tt.input, got)
				}
			} else {
				if got != "" {
					t.Errorf("expected input %q to be rejected, got text %q", tt.input, got)
				}
			}
		})
	}
}

func TestNumericalEntry_Keyboard(t *testing.T) {
	entry := ui.NewNumericalEntry()

	// Verify it requests the Number keyboard on mobile devices
	if got := entry.Keyboard(); got != mobile.NumberKeyboard {
		t.Errorf("expected keyboard type %v, got %v", mobile.NumberKeyboard, got)
	}
}

// TestNumericalEntry_Paste_Validation simulates a paste event.
// Note: The TypedRune filter only catches keystrokes.
// A robust implementation should also validate on change or blur,
// but here we document/test current behavior (it accepts non-digits on paste
// unless a Validator is attached, which is handled in ui.go logic, not the widget itself).
func TestNumericalEntry_DirectSetText(t *testing.T) {
	entry := ui.NewNumericalEntry()

	// Direct setting bypasses TypedRune
	entry.SetText("abc")
	if entry.Text != "abc" {
		t.Error("SetText should allow arbitrary text (validation happens separately)")
	}
}
