package engine_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/engine"
)

// TestHTTPFetcher_Fetch_Success verifies a complete successful download flow.
// It checks correct headers (User-Agent, Basic Auth) and response body integrity.
func TestHTTPFetcher_Fetch_Success(t *testing.T) {
	// 1. Setup Validation Data
	expectedUser := "testuser"
	expectedPass := "securepass"
	expectedBody := "BEGIN:VCARD\nVERSION:3.0\nFN:Test\nEND:VCARD"

	// 2. Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Basic Auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "Basic auth header should be present")
		assert.Equal(t, expectedUser, user, "Username mismatch")
		assert.Equal(t, expectedPass, pass, "Password mismatch")

		// Verify User-Agent matches the config constant
		assert.Equal(t, config.UserAgent, r.Header.Get("User-Agent"), "User-Agent mismatch")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer ts.Close()

	// 3. Execution
	fetcher := engine.NewHTTPFetcher()
	rc, err := fetcher.Fetch(context.Background(), ts.URL, expectedUser, expectedPass)

	// 4. Assertions
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, string(body))
}

// TestHTTPFetcher_Fetch_Errors verifies proper error handling for non-200 statuses.
func TestHTTPFetcher_Fetch_Errors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    string
	}{
		{"NotFound", http.StatusNotFound, "404"},
		{"ServerError", http.StatusInternalServerError, "500"},
		{"Unauthorized", http.StatusUnauthorized, "401"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer ts.Close()

			fetcher := engine.NewHTTPFetcher()
			rc, err := fetcher.Fetch(context.Background(), ts.URL, "", "")

			assert.Error(t, err)
			assert.Nil(t, rc)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestHTTPFetcher_Fetch_Timeout ensures the client respects context deadlines.
func TestHTTPFetcher_Fetch_Timeout(t *testing.T) {
	// Simulate a server that hangs
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	fetcher := engine.NewHTTPFetcher()

	// Create a context that expires very quickly (before server responds)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := fetcher.Fetch(ctx, ts.URL, "", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "Should return context deadline exceeded error")
}

// TestHTTPFetcher_Fetch_InvalidURL ensures malformed URLs are caught early.
func TestHTTPFetcher_Fetch_InvalidURL(t *testing.T) {
	fetcher := engine.NewHTTPFetcher()

	// Testing with a control character that makes URL parsing fail
	_, err := fetcher.Fetch(context.Background(), string([]byte{0x7f}), "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), config.ErrInvalidURL)
}

// TestHTTPFetcher_Fetch_ProtocolSecurity enforces HTTP/HTTPS only.
func TestHTTPFetcher_Fetch_ProtocolSecurity(t *testing.T) {
	fetcher := engine.NewHTTPFetcher()

	_, err := fetcher.Fetch(context.Background(), "ftp://example.com/file.vcf", "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), config.ErrProtocol)
}
