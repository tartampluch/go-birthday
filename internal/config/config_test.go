package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tartampluch/go-birthday/internal/config"
)

// TestConstants_Integrity ensures critical constants are not empty or malformed.
// This prevents accidental deletion of keys required for runtime or UI logic.
func TestConstants_Integrity(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"AppName", config.AppName},
		{"AppID", config.AppID},
		{"Version", config.Version},
		{"UserAgent", config.UserAgent},
		{"ICalVersion", config.ICalVersion},
		{"ICalProdid", config.ICalProdid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.value, "Critical constant %s should not be empty", tt.name)
		})
	}
}

// TestDefaults_Sanity checks that default values make sense logically.
func TestDefaults_Sanity(t *testing.T) {
	assert.Greater(t, config.DefaultRefreshMin, 0, "Default refresh interval must be positive")
	assert.Equal(t, 2000, config.DefaultLeapYear, "Default leap year must be 2000 for consistency")

	// Verify Timeout parsing works as expected
	assert.Equal(t, 30*time.Second, config.HTTPTimeout)
}

// TestUserAgent_Format ensures the UA string follows the standard format.
func TestUserAgent_Format(t *testing.T) {
	assert.True(t, strings.HasPrefix(config.UserAgent, "Go-Birthday/"), "UserAgent must start with AppName/")
}

// TestTimeoutsAndLimits ensures that operational constraints are reasonable.
func TestTimeoutsAndLimits(t *testing.T) {
	t.Parallel()

	// Timeouts
	assert.Greater(t, config.HTTPTimeout, 0*time.Second, "HTTPTimeout must be positive")
	// Since we allow 256MB downloads, 1 minute is a reasonable timeout.
	assert.LessOrEqual(t, config.HTTPTimeout, 2*time.Minute, "HTTPTimeout should not be excessively long")
	assert.Greater(t, config.ShutdownTimeout, 0*time.Second, "ShutdownTimeout must be positive")

	// Limits
	assert.Greater(t, config.MaxHTTPResponseSize, 0, "MaxHTTPResponseSize must be positive")
	// The limit should be generous enough for photos but prevent infinite streams.
	// 256MB is our target, ensuring it's not set back to a low value like 10MB unintentionally.
	assert.GreaterOrEqual(t, int64(config.MaxHTTPResponseSize), int64(50*1024*1024), "MaxHTTPResponseSize should be at least 50MB for real-world usage")
	assert.Less(t, int64(config.MaxHTTPResponseSize), int64(1*1024*1024*1024), "MaxHTTPResponseSize should stay under 1GB to protect RAM")
}
