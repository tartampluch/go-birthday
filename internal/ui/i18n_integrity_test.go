package ui_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tartampluch/go-birthday/internal/config"
)

// TestI18nIntegrity ensures that every translation key defined in config.go
// actually exists in the locale JSON files.
func TestI18nIntegrity(t *testing.T) {
	definedKeys := make(map[string]bool)

	keysToCheck := []string{
		config.TKeyWinTitle,
		config.TKeyWinContacts,
		config.TKeyMenuRefresh,
		config.TKeyMenuSettings,
		config.TKeyTrayStatus,
		config.TKeyTrayStatusZero, // Correctly added
		config.TKeyNotifStart,
		config.TKeyNotifSuccess,
		config.TKeyNotifError,
		config.TKeyModeCardDAV,
		config.TKeyModeLocal,
		config.TKeyLblLanguage,
		config.TKeyHelpLanguage,
		config.TKeyLblMinutes,
		config.TKeyLblRefresh,
		config.TKeyHelpInterval,
		config.TKeyLblPort,
		config.TKeyHelpPort,
		config.TKeyLblGeneral,
		config.TKeyLblEnableRem,
		config.TKeyUnitDays,
		config.TKeyUnitHours,
		config.TKeyUnitMinutes,
		config.TKeyDirBefore,
		config.TKeyDirAfter,
		config.TKeyLblNotif,
		config.TKeyBtnSave,
		config.TKeyBtnCancel,
		config.TKeyLblFooter,
		config.TKeyBtnBrowse,
		config.TKeyLblURL,
		config.TKeyHelpURL,
		config.TKeyLblUser,
		config.TKeyLblPass,
		config.TKeyLblSource,
		config.TKeyLblStartDay,
		config.TKeyEvtSummary,
		config.TKeyEvtSummaryAge,
		config.TKeyEvtSummaryBirth,
		config.TKeyErrPortReq,
		config.TKeyErrPortNum,
		config.TKeyErrPortRange,
		// New Columns & Formats
		config.TKeyColName,
		config.TKeyColDate,
		config.TKeyColAge,
		config.TKeyFormatDate,
		config.TKeyAgeBirth, // Correctly added
	}

	for _, k := range keysToCheck {
		definedKeys[k] = true
	}

	// Adjust path if running test from internal/ui or root
	path := "locales/active.en.json"
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Fallback for running tests from different CWD
		path = filepath.Join("..", "..", "internal", "ui", "locales", "active.en.json")
		content, err = os.ReadFile(path)
	}
	require.NoError(t, err, "Must load active.en.json")

	var jsonMap map[string]interface{}
	err = json.Unmarshal(content, &jsonMap)
	require.NoError(t, err, "JSON must be valid")

	// Verify consistency
	for key := range definedKeys {
		_, exists := jsonMap[key]
		assert.Truef(t, exists, "Key '%s' defined in config.go is missing in active.en.json", key)
	}

	// Check for orphan keys in JSON (keys that exist in JSON but not in Go)
	for jsonKey := range jsonMap {
		if strings.HasPrefix(jsonKey, "_") {
			continue
		}
		_, exists := definedKeys[jsonKey]
		if !exists {
			t.Logf("Warning: Key '%s' exists in JSON but is not checked in the test suite (might be unused)", jsonKey)
		}
	}
}
