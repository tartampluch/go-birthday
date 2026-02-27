package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCalculateNextOccurrence verifies the core temporal logic of the application.
// It covers standard dates, boundaries (end of year), and leap year complexities.
func TestCalculateNextOccurrence(t *testing.T) {
	// Reference "Now": June 15th, 2025 (Non-Leap Year)
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		birthDate    time.Time
		yearKnown    bool
		expectedDate time.Time
		expectedAge  int
		desc         string
	}{
		{
			name:         "Birthday in the past (this year)",
			birthDate:    time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			yearKnown:    true,
			expectedDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedAge:  36, // 2026 - 1990
			desc:         "Jan 1 is before June 15, so next occurrence is 2026",
		},
		{
			name:         "Birthday in the future (this year)",
			birthDate:    time.Date(1990, 12, 31, 0, 0, 0, 0, time.UTC),
			yearKnown:    true,
			expectedDate: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			expectedAge:  35, // 2025 - 1990
			desc:         "Dec 31 is after June 15, so next occurrence is 2025",
		},
		{
			name:         "Birthday is Today",
			birthDate:    time.Date(1990, 6, 15, 0, 0, 0, 0, time.UTC),
			yearKnown:    true,
			expectedDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			expectedAge:  35,
			desc:         "If birthday is today, it counts as the next occurrence (for 'Today' list)",
		},
		{
			name:         "Year Unknown - Past",
			birthDate:    time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), // Year 0 usually implies unknown in parsing logic
			yearKnown:    false,
			expectedDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedAge:  0,
			desc:         "Logic should calculate correct date but return age 0",
		},
		{
			name:         "Leapling - Non-Leap Year (Feb 29 -> Mar 1)",
			birthDate:    time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC),
			yearKnown:    true,
			expectedDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			expectedAge:  26,
			desc:         "Born Feb 29. Next occurrence relative to June 2025 is March 1st 2026 (Go normalizes non-leap Feb 29 to Mar 1)",
		},
		{
			name:      "Leapling - Upcoming Leap Year",
			birthDate: time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC),
			yearKnown: true,
			// Let's change context "Now" for this specific case to check Feb 2028 logic
			// But since helper uses 'now' param, we can't easily change it inside table-driven without lambda.
			// We stick to the 2025/2026 logic here.
			expectedDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			expectedAge:  26,
			desc:         "Standard behavior for non-leap target year",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, age := calculateNextOccurrence(now, tt.birthDate, tt.yearKnown)
			assert.Equal(t, tt.expectedDate, next, tt.desc)
			assert.Equal(t, tt.expectedAge, age, "Age calculation mismatch")
		})
	}
}

// TestCalculateNextOccurrence_LeapYear verifies behavior when *current* year is a leap year.
func TestCalculateNextOccurrence_LeapYearContext(t *testing.T) {
	// Reference "Now": Jan 1st, 2024 (Leap Year)
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	birthDate := time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC) // Leapling

	next, _ := calculateNextOccurrence(now, birthDate, true)

	// In 2024, Feb 29 exists. It should be preserved.
	expected := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, next, "In a leap year, the birthday should be Feb 29, not Mar 1")
}
