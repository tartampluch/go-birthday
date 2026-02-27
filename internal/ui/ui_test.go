package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/server"
)

// -----------------------------------------------------------------------------
// Mocks
// -----------------------------------------------------------------------------

// MockFetcher simulates the engine.VCardFetcher interface using testify/mock.
type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) Fetch(ctx context.Context, url, user, pass string) (io.ReadCloser, error) {
	args := m.Called(ctx, url, user, pass)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

// MockClock controls time for deterministic testing.
type MockClock struct {
	CurrentTime time.Time
}

func (m MockClock) Now() time.Time {
	return m.CurrentTime
}

// MockTray implements minimal system tray functionality for headless testing.
type MockTray struct {
	Menu *fyne.Menu
}

func (m *MockTray) SetSystemTrayMenu(menu *fyne.Menu) {
	m.Menu = menu
}

func (m *MockTray) SetSystemTrayIcon(icon fyne.Resource) {}
func (m *MockTray) SetSystemTrayWindow(w fyne.Window)    {}
func (m *MockTray) Run()                                 {}
func (m *MockTray) Quit()                                {}

// -----------------------------------------------------------------------------
// Test Setup Helper
// -----------------------------------------------------------------------------

// setupTestApp initializes a headless Fyne app with mocked dependencies.
func setupTestApp(t *testing.T) (*GoBirthdayApp, *MockFetcher, *MockTray) {
	// Initialize headless driver
	a := test.NewApp()

	// Use port "0" to bind to any free port during tests
	srv := server.NewCalendarServer("0")
	fetcher := new(MockFetcher)
	mockTray := &MockTray{}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	app := NewGoBirthdayApp(a, ctx, srv, fetcher)

	// Inject mocks
	app.Tray = mockTray

	// Default MockClock to a neutral date if not overridden by test
	app.Clock = MockClock{CurrentTime: time.Now()}

	// Manually load I18n as Run() is skipped
	app.SetupI18n()

	return app, fetcher, mockTray
}

// -----------------------------------------------------------------------------
// Localization Tests
// -----------------------------------------------------------------------------

func TestLocalization_Switching(t *testing.T) {
	app, _, _ := setupTestApp(t)

	// Case 1: English (Default)
	app.Preferences.SetString(config.PrefLanguage, "en")
	app.UpdateLocalizer()

	// Use constant key TKeyMenuSettings
	assert.Equal(t, "Settings...", app.GetMsg(config.TKeyMenuSettings))

	// Case 2: French
	app.Preferences.SetString(config.PrefLanguage, "fr")
	app.UpdateLocalizer()
	assert.Equal(t, "Paramètres...", app.GetMsg(config.TKeyMenuSettings))
}

func TestLocalization_SummaryFormatter(t *testing.T) {
	app, _, _ := setupTestApp(t)
	app.Preferences.SetString(config.PrefLanguage, "en")
	app.UpdateLocalizer()

	formatter := app.buildSummaryFormatter()

	// Scenario 1: Age is known (> 0)
	res := formatter("Alice", 30, true)
	assert.Contains(t, res, "Alice")
	assert.Contains(t, res, "30")

	// Scenario 2: Age unknown (0 but year unknown)
	// FallbackSummary -> "Birthday: %s"
	res = formatter("Bob", 0, false)
	assert.Contains(t, res, "Bob")
	assert.NotContains(t, res, "(0)", "Should not display age 0 if year unknown")

	// Scenario 3: Birth (Age 0 and year known)
	// Requires TKeyEvtSummaryBirth or FallbackSummaryBirth
	// The localized string in active.en.json is "... (birth)"
	res = formatter("Baby", 0, true)
	assert.Contains(t, res, "Baby")
	assert.Contains(t, res, "birth", "Should indicate birth for age 0 when year is known")
}

// -----------------------------------------------------------------------------
// Configuration & Preferences Tests
// -----------------------------------------------------------------------------

func TestConfiguration_Mapping(t *testing.T) {
	app, _, _ := setupTestApp(t)

	// Set Fyne Preferences
	app.Preferences.SetString(config.PrefSourceMode, config.SourceModeWeb)
	app.Preferences.SetString(config.PrefCardDAVURL, "https://secure.example.com")
	app.Preferences.SetString(config.PrefUsername, "admin")

	// Configure Reminders: 2 Days Before
	app.Preferences.SetBool(config.PrefReminderEnabled, true)
	app.Preferences.SetInt(config.PrefReminderValue, 2)
	app.Preferences.SetString(config.PrefReminderUnit, config.UnitDays)
	app.Preferences.SetString(config.PrefReminderDir, config.DirBefore)

	// Map to internal Config
	cfg := app.loadSyncConfig()

	// Assertions
	assert.Equal(t, config.SourceModeWeb, cfg.Mode)
	assert.Equal(t, "https://secure.example.com", cfg.WebURL)
	assert.Equal(t, "admin", cfg.WebUser)

	// -P2D matches ISO8601 for "2 Days Before"
	expectedTrigger := fmt.Sprintf("%s%d%s", config.ISONegativePrefix, 2, config.ISODay)
	assert.Equal(t, expectedTrigger, cfg.ReminderTrigger)
}

func TestConfiguration_WorkerSignal(t *testing.T) {
	app, _, _ := setupTestApp(t)
	app.watchPreferences()

	// Capture signal
	signalReceived := make(chan bool)
	go func() {
		select {
		case key := <-app.configChan:
			// Corrected syntax: using '{' instead of ':'
			if key == config.PrefInterval {
				signalReceived <- true
			}
		case <-time.After(500 * time.Millisecond):
			signalReceived <- false
		}
	}()

	// Trigger change
	app.Preferences.SetInt(config.PrefInterval, 120)

	assert.True(t, <-signalReceived, "Changing interval should notify background worker")
}

// -----------------------------------------------------------------------------
// Sync Logic Integration Tests
// -----------------------------------------------------------------------------

func TestPerformSync_Success(t *testing.T) {
	app, fetcher, mockTray := setupTestApp(t)
	app.setupTrayMenu()

	// Mock Date: Jan 1st, 2025
	// This ensures the birthday matches "Today"
	app.Clock = MockClock{CurrentTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)}

	// Mock Data
	vcard := "BEGIN:VCARD\nVERSION:3.0\nFN:Success User\nBDAY:19900101\nEND:VCARD"
	fetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(bytes.NewBufferString(vcard)), nil)

	app.Preferences.SetString(config.PrefSourceMode, config.SourceModeWeb)
	app.Preferences.SetString(config.PrefCardDAVURL, "http://test.local")

	app.performSync(true)

	fetcher.AssertExpectations(t)

	require.NotNil(t, mockTray.Menu)
	assert.Contains(t, app.TrayStatusItem.Label, "1", "Tray label should reflect 1 birthday found")

	// Verify Contacts were populated
	app.ContactsMut.RLock()
	assert.Len(t, app.Contacts, 1)
	assert.Equal(t, "Success User", app.Contacts[0].Name)
	app.ContactsMut.RUnlock()
}

func TestPerformSync_Failure(t *testing.T) {
	app, fetcher, _ := setupTestApp(t)
	app.setupTrayMenu()

	fetcher.On("Fetch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("connection refused"))

	app.Preferences.SetString(config.PrefSourceMode, config.SourceModeWeb)
	app.Preferences.SetString(config.PrefCardDAVURL, "http://test.local")

	app.performSync(true)

	fetcher.AssertExpectations(t)
	assert.Equal(t, config.FallbackTrayError, app.TrayStatusItem.Label)
}

func TestTrayStatusUpdate_Logic(t *testing.T) {
	app, _, mockTray := setupTestApp(t)
	app.setupTrayMenu()

	// Force EN locale for predictable strings
	app.Preferences.SetString(config.PrefLanguage, "en")
	app.UpdateLocalizer()

	// 1. Error Case
	app.updateTrayStatus(-1)
	assert.Equal(t, config.FallbackTrayError, app.TrayStatusItem.Label)

	// 2. Zero Case (Explicit check for "No birthdays today")
	app.updateTrayStatus(0)
	assert.Equal(t, "No birthdays today", app.TrayStatusItem.Label, "Should use explicit zero string")

	// 3. Positive Case
	app.updateTrayStatus(10)
	assert.Contains(t, app.TrayStatusItem.Label, "10")

	// Ensure refresh was called on the menu
	assert.NotNil(t, mockTray.Menu)
}
