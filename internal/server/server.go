package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/tartampluch/go-birthday/internal/config"
)

// cacheItem stores the rendered calendar and its metadata for HTTP caching.
type cacheItem struct {
	data         []byte
	etag         string
	lastModified string // RFC1123 format required by HTTP headers
}

// CalendarServer handles serving the generated ICS file via HTTP.
type CalendarServer struct {
	// cache uses atomic.Pointer for lock-free reads.
	// Since the calendar is read frequently by clients but updated infrequently
	// (only on sync), this provides better performance than a RWMutex
	// by eliminating contention on the hot path (HTTP GET).
	cache atomic.Pointer[cacheItem]
	Port  string
}

// NewCalendarServer creates a new instance of the server.
func NewCalendarServer(port string) *CalendarServer {
	return &CalendarServer{
		Port: port,
	}
}

// Start initializes the HTTP server and blocks until the context is cancelled.
func (s *CalendarServer) Start(ctx context.Context) error {
	if s.Port == "" {
		return fmt.Errorf(config.ErrPortRequired)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(config.RouteRoot, s.handleCalendarRequest)

	srv := &http.Server{
		// Use defined constant for separator
		Addr:         config.LocalhostBindAddr + config.AddrSeparator + s.Port,
		Handler:      mux,
		ReadTimeout:  config.ServerReadTimeout,
		WriteTimeout: config.ServerWriteTimeout,
		IdleTimeout:  config.ServerIdleTimeout,
	}

	serverError := make(chan error, config.ChannelBufferSize)

	go func() {
		slog.Info(config.MsgServerListen,
			config.LogKeyComponent, config.CompServer,
			config.LogKeyPort, s.Port,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverError <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info(config.MsgServerStop, config.LogKeyComponent, config.CompServer)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("%s: %w", config.ErrServerShutdown, err)
		}
		return nil

	case err := <-serverError:
		return fmt.Errorf("%s: %w", config.ErrServerStartup, err)
	}
}

// Update atomically replaces the served content.
func (s *CalendarServer) Update(data []byte) {
	hash := sha256.Sum256(data)
	// Use centralized format string for ETag consistency.
	etag := fmt.Sprintf(config.FormatETag, hex.EncodeToString(hash[:]))

	lastMod := time.Now().UTC().Format(http.TimeFormat)

	item := &cacheItem{
		data:         data,
		etag:         etag,
		lastModified: lastMod,
	}

	// Atomic store ensures that any concurrent reader sees either the old or the new complete item,
	// never a partial state.
	s.cache.Store(item)

	slog.Debug(config.MsgCacheUpdated,
		config.LogKeyComponent, config.CompServer,
		config.LogKeySizeBytes, len(data),
		config.LogKeyETag, etag,
	)
}

// handleCalendarRequest serves the ICS content with HTTP caching support.
func (s *CalendarServer) handleCalendarRequest(w http.ResponseWriter, r *http.Request) {
	// 1. Method Validation
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set(config.HeaderAllow, config.AllowedMethods)
		http.Error(w, config.HTTPMsgMethodNotAll, http.StatusMethodNotAllowed)
		return
	}

	// 2. Load Data (Atomic / Lock-Free)
	item := s.cache.Load()

	// 3. Readiness Check
	if item == nil {
		w.Header().Set(config.HeaderRetryAfter, config.RetryAfterSeconds)
		http.Error(w, config.HTTPMsgInitializing, http.StatusServiceUnavailable)
		return
	}

	// 4. Set Response Headers
	w.Header().Set(config.HeaderContentType, config.MimeTextCalendar)
	w.Header().Set(config.HeaderXContentType, config.MimeNoSniff)
	w.Header().Set(config.HeaderCacheControl, config.CacheControlPrivate)
	w.Header().Set(config.HeaderETag, item.etag)
	w.Header().Set(config.HeaderLastModified, item.lastModified)

	// 5. Check Conditional Headers (Browser Caching)
	if match := r.Header.Get(config.HeaderIfNoneMatch); match == item.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if since := r.Header.Get(config.HeaderIfModifiedSince); since != "" {
		if clientTime, err := time.Parse(http.TimeFormat, since); err == nil {
			if serverTime, err := time.Parse(http.TimeFormat, item.lastModified); err == nil {
				// If server content is not newer than client cache, return 304.
				if !serverTime.After(clientTime) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}
	}

	// 6. Serve Content
	if r.Method == http.MethodGet {
		if _, err := io.Copy(w, bytes.NewReader(item.data)); err != nil {
			slog.Error(config.ErrWriteResp,
				config.LogKeyComponent, config.CompServer,
				config.LogKeyError, err,
			)
		}
	}
}
