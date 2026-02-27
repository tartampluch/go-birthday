package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/tartampluch/go-birthday/internal/config"
)

// TestApp_LoadSyncConfig_Reminders tests the conversion of UI preferences to Engine config.
// By being in package 'ui', we can test the private method 'loadSyncConfig'.
func TestApp_LoadSyncConfig_Reminders(t *testing.T) {
	a := test.NewApp()
	// Mock Context and minimal dependencies
	app := &GoBirthdayApp{
		App:         a,
		Preferences: a.Preferences(),
	}

	tests := []struct {
		name        string
		enabled     bool
		val         int
		unit        string
		direction   string
		wantTrigger string // Expected ISO8601 string
	}{
		{
			name:        "Disabled",
			enabled:     false,
			wantTrigger: "",
		},
		{
			name:        "1 Day Before",
			enabled:     true,
			val:         1,
			unit:        config.UnitDays,
			direction:   config.DirBefore,
			wantTrigger: "-P1D",
		},
		{
			name:        "2 Hours After",
			enabled:     true,
			val:         2,
			unit:        config.UnitHours,
			direction:   config.DirAfter,
			wantTrigger: "P2H",
		},
		{
			name:        "30 Minutes Before",
			enabled:     true,
			val:         30,
			unit:        config.UnitMinutes,
			direction:   config.DirBefore,
			wantTrigger: "-P30M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Preferences
			app.Preferences.SetBool(config.PrefReminderEnabled, tt.enabled)
			app.Preferences.SetInt(config.PrefReminderValue, tt.val)
			app.Preferences.SetString(config.PrefReminderUnit, tt.unit)
			app.Preferences.SetString(config.PrefReminderDir, tt.direction)

			// Execute Logic
			cfg := app.loadSyncConfig()

			// Verify
			assert.Equal(t, tt.wantTrigger, cfg.ReminderTrigger)
		})
	}
}
