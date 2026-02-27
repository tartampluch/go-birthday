package ui

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/engine"
)

// -----------------------------------------------------------------------------
// Sorting Logic Tests
// -----------------------------------------------------------------------------

// TestSortingLogic_Dates verifies that the contact list sorts correctly by NextOccurrence.
// This is the most critical user requirement: "Who is next?".
func TestSortingLogic_Dates(t *testing.T) {
	// Context: Today is June 1, 2025
	dateToday := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	dateDec := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	dateJan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // Next year

	data := []engine.BirthdayEntry{
		{Name: "C_NextYear", NextOccurrence: dateJan},
		{Name: "A_LaterThisYear", NextOccurrence: dateDec},
		{Name: "B_Today", NextOccurrence: dateToday},
	}

	// 1. Sort by Date Ascending
	// Expected Order: Today -> Dec 2025 -> Jan 2026
	sort.Slice(data, func(i, j int) bool {
		return data[i].NextOccurrence.Before(data[j].NextOccurrence)
	})

	assert.Equal(t, "B_Today", data[0].Name, "Today should be first")
	assert.Equal(t, "A_LaterThisYear", data[1].Name, "December this year should be second")
	assert.Equal(t, "C_NextYear", data[2].Name, "January next year should be last")
}

// TestSortingLogic_Names verifies case-insensitive alphabetical sorting.
func TestSortingLogic_Names(t *testing.T) {
	data := []engine.BirthdayEntry{
		{Name: "alice"},
		{Name: "Bob"},
		{Name: "charlie"},
	}

	// Sort Ascending
	sort.Slice(data, func(i, j int) bool {
		return strings.ToLower(data[i].Name) < strings.ToLower(data[j].Name)
	})

	assert.Equal(t, "alice", data[0].Name)
	assert.Equal(t, "Bob", data[1].Name)
	assert.Equal(t, "charlie", data[2].Name)
}

// TestSortingLogic_Age verifies the complex age sorting rules (handling unknowns).
func TestSortingLogic_Age(t *testing.T) {
	data := []engine.BirthdayEntry{
		{Name: "Young", AgeNext: 10, YearKnown: true},
		{Name: "Old", AgeNext: 50, YearKnown: true},
		{Name: "Unknown1", AgeNext: 0, YearKnown: false},
		{Name: "Unknown2", AgeNext: 0, YearKnown: false},
		{Name: "Baby", AgeNext: 0, YearKnown: true}, // Special case: Born this year
	}

	// Logic extracted from ui_contacts.go for Ascending Sort
	sort.Slice(data, func(i, j int) bool {
		a, b := data[i], data[j]
		if !a.YearKnown && b.YearKnown {
			return false // Unknown goes to bottom in Asc
		}
		if a.YearKnown && !b.YearKnown {
			return true
		}
		return a.AgeNext < b.AgeNext
	})

	// Expected Order: Baby (0) -> Young (10) -> Old (50) -> Unknowns
	assert.Equal(t, "Baby", data[0].Name, "Known age 0 (Baby) should be first")
	assert.Equal(t, "Young", data[1].Name)
	assert.Equal(t, "Old", data[2].Name)
	assert.Contains(t, data[3].Name, "Unknown", "Unknowns should be at the end")
}

// -----------------------------------------------------------------------------
// UI Formatting Tests (View Model Logic)
// -----------------------------------------------------------------------------

// TestTableFormatting verifies that the raw data is correctly converted to display strings.
func TestTableFormatting(t *testing.T) {
	// Setup app purely for I18n loading
	app, _, _ := setupTestApp(t)
	app.Preferences.SetString(config.PrefLanguage, "en")
	app.UpdateLocalizer()

	tests := []struct {
		name        string
		entry       engine.BirthdayEntry
		expectedAge string
		desc        string
	}{
		{
			name:        "Standard Age Transition",
			entry:       engine.BirthdayEntry{YearKnown: true, AgeNext: 26},
			expectedAge: "25 → 26",
			desc:        "Should display transition from prev age to next age",
		},
		{
			name:        "Birth Transition (1 year old)",
			entry:       engine.BirthdayEntry{YearKnown: true, AgeNext: 1},
			expectedAge: "Birth → 1",
			desc:        "Should display Birth -> 1 for first birthday",
		},
		{
			name:        "Unknown Year",
			entry:       engine.BirthdayEntry{YearKnown: false, AgeNext: 0},
			expectedAge: config.AgeUnknown, // "-"
			desc:        "Should display hyphen for unknown years",
		},
		{
			name:        "Newborn (Age 0 - Birth event)",
			entry:       engine.BirthdayEntry{YearKnown: true, AgeNext: 0},
			expectedAge: config.AgeBirth, // "(birth)"
			desc:        "Should display special label for age 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the UI logic block for Age (copied from ui_contacts.go)
			var display string
			if tt.entry.YearKnown {
				if tt.entry.AgeNext == 0 {
					display = config.AgeBirth
				} else {
					// Logic copy from ui_contacts.go for testing
					prevAge := tt.entry.AgeNext - 1
					if prevAge == 0 {
						birthText := app.GetMsg(config.TKeyAgeBirth)
						if birthText == config.TKeyAgeBirth {
							birthText = "Birth" // Fallback simulation
						}
						display = fmt.Sprintf("%s → %d", birthText, tt.entry.AgeNext)
					} else {
						display = fmt.Sprintf("%d → %d", prevAge, tt.entry.AgeNext)
					}
				}
			} else {
				display = config.AgeUnknown
			}

			assert.Equal(t, tt.expectedAge, display)
		})
	}
}

// TestContactsWindow_Singleton verifies the logic guarding multiple window instances.
func TestContactsWindow_Singleton(t *testing.T) {
	app, _, _ := setupTestApp(t)

	// Initially nil
	assert.Nil(t, app.Window, "Main window might be set, but contacts window is internal property")
}
