package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tartampluch/go-birthday/internal/config"
)

// -----------------------------------------------------------------------------
// Unit Tests (White-Box Testing of Handler Logic)
// -----------------------------------------------------------------------------

// TestHandler_ServingContent verifies that the handler correctly writes
// the standard HTTP headers and body content when data is available.
func TestHandler_ServingContent(t *testing.T) {
	// Setup
	srv := NewCalendarServer("0") // Port irrelevant for handler test
	expectedICS := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR")

	// Pre-load data into the atomic cache
	srv.Update(expectedICS)

	// Execute
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.handleCalendarRequest(w, req)

	// Assertions
	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, config.MimeTextCalendar, resp.Header.Get(config.HeaderContentType))
	assert.Equal(t, config.MimeNoSniff, resp.Header.Get(config.HeaderXContentType))
	assert.Contains(t, resp.Header.Get(config.HeaderCacheControl), "no-cache")

	// ETag should be generated automatically
	assert.NotEmpty(t, resp.Header.Get(config.HeaderETag))

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, expectedICS, body)
}

// TestHandler_Caching verifies that the server respects ETag headers (If-None-Match)
// and returns 304 Not Modified to save bandwidth.
func TestHandler_Caching(t *testing.T) {
	srv := NewCalendarServer("0")
	data := []byte("DATA_VERSION_1")
	srv.Update(data)

	// Step 1: Initial Request to get the ETag
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	w1 := httptest.NewRecorder()
	srv.handleCalendarRequest(w1, req1)

	etag := w1.Result().Header.Get(config.HeaderETag)
	require.NotEmpty(t, etag, "Server must provide an ETag")

	// Step 2: Second Request providing the known ETag
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set(config.HeaderIfNoneMatch, etag)
	w2 := httptest.NewRecorder()

	srv.handleCalendarRequest(w2, req2)
	resp2 := w2.Result()
	defer func() { _ = resp2.Body.Close() }()

	// Assertions
	assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
	body, _ := io.ReadAll(resp2.Body)
	assert.Empty(t, body, "Body must be empty on 304 Not Modified")
}

// TestHandler_MethodNotAllowed ensures strictly GET and HEAD are accepted.
func TestHandler_MethodNotAllowed(t *testing.T) {
	srv := NewCalendarServer("0")

	// Try POST
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()

	srv.handleCalendarRequest(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(config.HeaderAllow))
}

// TestHandler_Initializing verifies the 503 behavior when data is not yet ready.
func TestHandler_Initializing(t *testing.T) {
	srv := NewCalendarServer("0")
	// Note: We intentionally do NOT call srv.Update() here.

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.handleCalendarRequest(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, config.RetryAfterSeconds, resp.Header.Get(config.HeaderRetryAfter))
}

// -----------------------------------------------------------------------------
// Concurrency Tests (Race Detection)
// -----------------------------------------------------------------------------

// TestServer_RaceCondition validates the thread-safety of atomic.Pointer usage.
// It runs high-frequency writers and readers concurrently to trigger race conditions.
// Run this with `go test -race`.
func TestServer_RaceCondition(t *testing.T) {
	srv := NewCalendarServer("0")
	var wg sync.WaitGroup

	// Duration of the stress test
	duration := 500 * time.Millisecond
	end := time.Now().Add(duration)

	// Writer Routines: Stress atomic.Pointer.Store
	// We spawn multiple writers to ensure multiple updates happen efficiently.
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			i := 0
			for time.Now().Before(end) {
				data := fmt.Sprintf("VERSION:%d-%d", id, i)
				srv.Update([]byte(data))
				i++
				// Tiny sleep to yield processor and allow interleaving
				time.Sleep(1 * time.Microsecond)
			}
		}(w)
	}

	// Reader Routines: Stress atomic.Pointer.Load through the handler
	for r := 0; r < 20; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(end) {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				w := httptest.NewRecorder()

				srv.handleCalendarRequest(w, req)

				// Validates that we don't get partial writes or crashes.
				code := w.Code
				if code != http.StatusOK && code != http.StatusServiceUnavailable {
					t.Errorf("Unexpected status code during race test: %d", code)
				}
			}
		}()
	}

	wg.Wait()
}

// -----------------------------------------------------------------------------
// Integration Tests (Real TCP Lifecycle)
// -----------------------------------------------------------------------------

// TestServer_Lifecycle spins up the actual TCP listener to verify network binding
// and graceful shutdown logic.
func TestServer_Lifecycle(t *testing.T) {
	// Use a port that is unlikely to be in use.
	// In a CI environment, passing "0" to the listener is safer, but
	// since our struct takes "Port" as a string and builds the string manually,
	// we use a high port.
	const port = "18099"

	srv := NewCalendarServer(port)
	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error, 1)

	// Start server in background
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Construct the URL
	url := "http://127.0.0.1:" + port + "/"

	// Wait for server to be responsive (TCP bind takes a few milliseconds)
	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	}, 2*time.Second, 50*time.Millisecond, "Server failed to bind/listen in time")

	// 1. Check Initial State (503)
	resp, err := http.Get(url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	_ = resp.Body.Close()

	// 2. Update Data
	srv.Update([]byte("BEGIN:VCALENDAR\nEND:VCALENDAR"))

	// 3. Check Served Content (200)
	resp, err = http.Get(url)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, config.MimeTextCalendar, resp.Header.Get(config.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "BEGIN:VCALENDAR")

	// 4. Test Shutdown
	cancel() // Trigger context cancellation

	select {
	case err := <-errChan:
		// Start() returns nil on graceful shutdown
		assert.NoError(t, err, "Server should shutdown gracefully without error")
	case <-time.After(5 * time.Second):
		t.Fatal("Server shutdown timed out")
	}
}
