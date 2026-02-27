package engine_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/engine"
)

// -----------------------------------------------------------------------------
// Mocks
// -----------------------------------------------------------------------------

// MockFetcher simulates the network layer for unit tests using `testify/mock`.
type MockFetcher struct {
	mock.Mock
}

// Fetch implements the engine.VCardFetcher interface.
func (m *MockFetcher) Fetch(ctx context.Context, url, user, pass string) (io.ReadCloser, error) {
	args := m.Called(ctx, url, user, pass)
	// Return nil interface safely
	if r := args.Get(0); r != nil {
		return r.(io.ReadCloser), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockClock controls time for deterministic testing.
type MockClock struct {
	CurrentTime time.Time
}

func (m MockClock) Now() time.Time {
	return m.CurrentTime
}

// -----------------------------------------------------------------------------
// Test Cases
// -----------------------------------------------------------------------------

func TestRunSync_Local_Success(t *testing.T) {
	// Scenario: A local vCard with one valid contact having a birthday today.
	vcardContent := `BEGIN:VCARD
VERSION:4.0
FN:John Doe
BDAY:2000-01-01
END:VCARD`

	// Create a temporary file to simulate local storage
	tmpFile, err := os.CreateTemp("", "test_vcard_*.vcf")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(vcardContent)
	assert.NoError(t, err)
	_ = tmpFile.Close()

	// Set "Now" to John Doe's birthday
	fixedTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	gen := &engine.Generator{
		Clock: MockClock{CurrentTime: fixedTime},
		// No fetcher needed for local mode
	}

	cfg := engine.SyncConfig{
		Mode:      config.SourceModeLocal,
		LocalPath: tmpFile.Name(),
	}

	icsData, contacts, count, err := gen.RunSync(context.Background(), cfg)

	assert.NoError(t, err)
	assert.Equal(t, 1, count, "Should identify one birthday today")

	// Verify Contacts List DTO
	assert.Len(t, contacts, 1)
	assert.Equal(t, "John Doe", contacts[0].Name)
	assert.Equal(t, 25, contacts[0].AgeNext) // Born 2000, Now 2025 -> 25 years old

	icsStr := string(icsData)
	assert.Contains(t, icsStr, "BEGIN:VCALENDAR", "Should start with VCALENDAR")
	assert.Contains(t, icsStr, "SUMMARY:Birthday: John Doe", "Should contain the event summary")
}

func TestRunSync_Web_LeapYear_EdgeCase(t *testing.T) {
	// Scenario: A contact born on Feb 29th (Leapling).
	// We test if it correctly shows up on March 1st in a non-leap year (2025).
	vcardContent := `BEGIN:VCARD
VERSION:3.0
FN:Leap Baby
BDAY:2000-02-29
END:VCARD`

	mockFetcher := new(MockFetcher)
	mockFetcher.On("Fetch", mock.Anything, "http://example.com", "", "").
		Return(io.NopCloser(strings.NewReader(vcardContent)), nil)

	// 2025 is NOT a leap year. Feb 29 -> March 1 in Go's time.Date normalization.
	fixedTime := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)

	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: fixedTime},
		Fetcher: mockFetcher,
	}

	cfg := engine.SyncConfig{
		Mode:   config.SourceModeWeb,
		WebURL: "http://example.com",
	}

	_, contacts, count, err := gen.RunSync(context.Background(), cfg)

	assert.NoError(t, err)
	assert.Equal(t, 1, count, "Leapling should have birthday on March 1st in non-leap year")

	// Verify NextOccurrence logic for leapling
	assert.Len(t, contacts, 1)
	// Expect NextOccurrence to be normalized to March 1st, 2025
	expectedNext := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedNext, contacts[0].NextOccurrence)

	mockFetcher.AssertExpectations(t)
}

func TestRunSync_ContactListNextOccurrence(t *testing.T) {
	// Scenario: Verify NextOccurrence logic for various dates relative to Now (2025-06-01)
	vcardContent := `BEGIN:VCARD
VERSION:3.0
FN:Past Birthday
BDAY:1990-01-01
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Future Birthday
BDAY:1990-12-31
END:VCARD
BEGIN:VCARD
VERSION:3.0
FN:Today Birthday
BDAY:1990-06-01
END:VCARD`

	mockFetcher := new(MockFetcher)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader(vcardContent)), nil)

	// Current date: June 1, 2025
	now := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: now},
		Fetcher: mockFetcher,
	}

	cfg := engine.SyncConfig{Mode: config.SourceModeWeb, WebURL: "http://test.local"}
	_, contacts, _, err := gen.RunSync(context.Background(), cfg)
	assert.NoError(t, err)
	assert.Len(t, contacts, 3)

	contactMap := make(map[string]engine.BirthdayEntry)
	for _, c := range contacts {
		contactMap[c.Name] = c
	}

	// 1. Past Birthday (Jan 1) -> Should be next year (2026)
	c1 := contactMap["Past Birthday"]
	assert.Equal(t, 2026, c1.NextOccurrence.Year())
	assert.Equal(t, time.Month(1), c1.NextOccurrence.Month())

	// 2. Future Birthday (Dec 31) -> Should be this year (2025)
	c2 := contactMap["Future Birthday"]
	assert.Equal(t, 2025, c2.NextOccurrence.Year())
	assert.Equal(t, time.Month(12), c2.NextOccurrence.Month())

	// 3. Today Birthday (June 1) -> Should be today (2025)
	c3 := contactMap["Today Birthday"]
	assert.Equal(t, 2025, c3.NextOccurrence.Year())
	assert.Equal(t, time.Month(6), c3.NextOccurrence.Month())
	// FIX: Day should be 1 (June 1st), not 6 (June)
	assert.Equal(t, 1, c3.NextOccurrence.Day())
}

func TestRunSync_Web_NetworkError(t *testing.T) {
	// Scenario: The fetcher returns a network error (e.g., DNS fail, 404).
	mockFetcher := new(MockFetcher)
	expectedErr := errors.New("network unreachable")

	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, expectedErr)

	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: time.Now()},
		Fetcher: mockFetcher,
	}

	cfg := engine.SyncConfig{
		Mode:   config.SourceModeWeb,
		WebURL: "http://bad-url.com",
	}

	icsData, contacts, count, err := gen.RunSync(context.Background(), cfg)

	assert.Error(t, err)
	// Verify error wrapping/propagation
	assert.True(t, errors.Is(err, expectedErr) || strings.Contains(err.Error(), expectedErr.Error()))
	assert.Nil(t, icsData)
	assert.Nil(t, contacts)
	assert.Equal(t, 0, count)
}

func TestRunSync_WithReminders(t *testing.T) {
	// Scenario: A valid vCard and a request for a 1-day reminder.
	vcardContent := "BEGIN:VCARD\nVERSION:3.0\nFN:Alarm Test\nBDAY:1990-01-01\nEND:VCARD"

	mockFetcher := new(MockFetcher)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader(vcardContent)), nil)

	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
		Fetcher: mockFetcher,
	}

	// ReminderTrigger "-P1D" means 1 day before
	cfg := engine.SyncConfig{
		Mode:            config.SourceModeWeb,
		WebURL:          "http://test.local",
		ReminderTrigger: "-P1D",
	}

	icsData, _, _, err := gen.RunSync(context.Background(), cfg)
	assert.NoError(t, err)

	icsStr := string(icsData)
	assert.Contains(t, icsStr, "BEGIN:VALARM", "ICS should contain an alarm component")
	assert.Contains(t, icsStr, "TRIGGER:-P1D", "Alarm trigger should match configuration")
	assert.Contains(t, icsStr, "ACTION:DISPLAY", "Alarm action should be DISPLAY")
}

func TestRunSync_GeneratesYearRange(t *testing.T) {
	// Scenario: Verify that we generate events for Prev Year, Current Year, Next Year (Total 3).
	// Current Date: 2025-01-01. Birth: 1990-12-31.
	vcardContent := "BEGIN:VCARD\nVERSION:3.0\nFN:Range Test\nBDAY:1990-12-31\nEND:VCARD"

	mockFetcher := new(MockFetcher)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader(vcardContent)), nil)

	// Current date: Jan 1, 2025
	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		Fetcher: mockFetcher,
	}

	cfg := engine.SyncConfig{Mode: config.SourceModeWeb, WebURL: "http://test.local"}

	icsData, _, _, err := gen.RunSync(context.Background(), cfg)
	assert.NoError(t, err)

	icsStr := string(icsData)

	// Verify events for 2024, 2025, 2026
	assert.Contains(t, icsStr, "DTSTART;VALUE=DATE:20241231", "Should include previous year")
	assert.Contains(t, icsStr, "DTSTART;VALUE=DATE:20251231", "Should include current year")
	assert.Contains(t, icsStr, "DTSTART;VALUE=DATE:20261231", "Should include next year")

	// Should generate exactly 3 events
	assert.Equal(t, 3, strings.Count(icsStr, "BEGIN:VEVENT"), "Should generate exactly 3 events (Prev, Curr, Next)")
}

func TestRunSync_BabyBornThisYear(t *testing.T) {
	// Scenario: Baby born on 2025-05-01. Current date is 2025-01-01.
	// Expected: 2024 (skipped), 2025 (Birth), 2026 (1 year).

	vcardContent := "BEGIN:VCARD\nVERSION:3.0\nFN:Baby\nBDAY:2025-05-01\nEND:VCARD"

	mockFetcher := new(MockFetcher)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader(vcardContent)), nil)

	// Current date: Jan 1, 2025
	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		Fetcher: mockFetcher,
		FormatSummary: func(name string, age int, yearKnown bool) string {
			if age == 0 {
				return fmt.Sprintf("Birthday: %s (Birth)", name)
			}
			return fmt.Sprintf("Birthday: %s (%d)", name, age)
		},
	}

	cfg := engine.SyncConfig{Mode: config.SourceModeWeb, WebURL: "http://test.local"}

	icsData, _, _, err := gen.RunSync(context.Background(), cfg)
	assert.NoError(t, err)

	icsStr := string(icsData)

	// Check 2024 (should NOT exist)
	assert.NotContains(t, icsStr, "DTSTART;VALUE=DATE:20240501", "Should NOT generate event before birth")

	// Check 2025 (Birth)
	assert.Contains(t, icsStr, "DTSTART;VALUE=DATE:20250501")
	assert.Contains(t, icsStr, "SUMMARY:Birthday: Baby (Birth)", "Should indicate birth event")

	// Check 2026 (1 year old)
	assert.Contains(t, icsStr, "DTSTART;VALUE=DATE:20260501")
	assert.Contains(t, icsStr, "SUMMARY:Birthday: Baby (1)", "Should indicate 1 year old")

	// Should generate exactly 2 events (2025, 2026), skipping 2024
	assert.Equal(t, 2, strings.Count(icsStr, "BEGIN:VEVENT"))
}

func TestRunSync_FutureBirth(t *testing.T) {
	// Scenario: Due date is in 2027. Current date is 2025.
	// Should not generate any events for 2024, 2025, 2026.
	vcardContent := "BEGIN:VCARD\nVERSION:3.0\nFN:Future Baby\nBDAY:2027-01-01\nEND:VCARD"

	mockFetcher := new(MockFetcher)
	mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader(vcardContent)), nil)

	gen := &engine.Generator{
		Clock:   MockClock{CurrentTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		Fetcher: mockFetcher,
	}

	cfg := engine.SyncConfig{Mode: config.SourceModeWeb, WebURL: "http://test.local"}

	icsData, _, _, err := gen.RunSync(context.Background(), cfg)
	assert.NoError(t, err)

	icsStr := string(icsData)
	assert.NotContains(t, icsStr, "BEGIN:VEVENT", "Should generate no events for unborn person in future years")
}

func TestRunSync_DateFormats_TableDriven(t *testing.T) {
	// Comprehensive test for various date formats encountered in the wild.
	tests := []struct {
		name      string
		bdayValue string
		expectEvt bool
	}{
		{"ISO8601 Standard", "1990-10-25", true},
		{"Basic Format", "19901025", true},
		{"RFC3339", "1990-10-25T00:00:00Z", true},
		{"Truncated (Month-Day)", "--10-25", true},
		{"Truncated Basic", "--1025", true},
		{"Garbage Data", "not-a-date", false},
		{"Empty Date", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "BEGIN:VCARD\nVERSION:3.0\nFN:Test\nBDAY:" + tt.bdayValue + "\nEND:VCARD"

			mockFetcher := new(MockFetcher)
			mockFetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(io.NopCloser(strings.NewReader(content)), nil)

			gen := &engine.Generator{
				Clock:   MockClock{CurrentTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
				Fetcher: mockFetcher,
			}

			ics, _, _, _ := gen.RunSync(context.Background(), engine.SyncConfig{Mode: config.SourceModeWeb, WebURL: "http://x"})

			icsStr := string(ics)
			if tt.expectEvt {
				assert.Contains(t, icsStr, "BEGIN:VEVENT", "Valid date should produce an event")
			} else {
				assert.NotContains(t, icsStr, "BEGIN:VEVENT", "Invalid date should be skipped silently")
			}
		})
	}
}

func TestRunSync_ContextCancellation(t *testing.T) {
	// Scenario: User quits app or timeout occurs during sync.
	ctx, cancel := context.WithCancel(context.Background())

	tmpFile, err := os.CreateTemp("", "cancel_test_*.vcf")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	cancel() // Cancel immediately before processing starts

	gen := &engine.Generator{
		Clock: MockClock{CurrentTime: time.Now()},
	}

	_, _, _, err = gen.RunSync(ctx, engine.SyncConfig{
		Mode:      config.SourceModeLocal,
		LocalPath: tmpFile.Name(), // Use valid path
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err, "Should return context canceled error")
}
