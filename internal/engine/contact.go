package engine

import "time"

// BirthdayEntry represents a lightweight contact record optimized for UI display.
// It decouples the UI from the heavy vCard parsing logic.
type BirthdayEntry struct {
	// UID is a unique identifier (hash) used for stability in lists.
	UID string

	// Name is the display name (Formatted Name or Structured Name).
	Name string

	// DateOfBirth is the original parsed date.
	DateOfBirth time.Time

	// YearKnown indicates if the vCard contained a year or just --MM-DD.
	YearKnown bool

	// NextOccurrence is the calculated date of the birthday for the current or next year.
	// This is the primary sorting key for the "Upcoming Birthdays" view.
	NextOccurrence time.Time

	// AgeNext is the age the person will turn at NextOccurrence.
	// Only valid if YearKnown is true.
	AgeNext int
}
