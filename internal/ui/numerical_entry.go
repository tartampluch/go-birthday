package ui

import (
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

// NumericalEntry is a custom Entry widget that only accepts numeric input.
// It embeds widget.Entry to inherit all standard behavior.
type NumericalEntry struct {
	widget.Entry
}

// NewNumericalEntry creates a new instance of NumericalEntry.
func NewNumericalEntry() *NumericalEntry {
	entry := &NumericalEntry{}
	entry.ExtendBaseWidget(entry)
	return entry
}

// TypedRune intercepts text input events.
// It filters characters to allow only digits (0-9).
func (e *NumericalEntry) TypedRune(r rune) {
	if r >= '0' && r <= '9' {
		e.Entry.TypedRune(r)
	}
	// Ignore non-numeric characters.
	// Note: Shortcuts like Ctrl+V (Paste) are handled by TypedShortcut/TypedKey,
	// so non-numeric data could still be pasted. The Validator handles that case.
}

// Keyboard overrides the default keyboard type.
// This ensures that on mobile devices, a numeric keypad is shown.
func (e *NumericalEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}
