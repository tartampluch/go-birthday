package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/tartampluch/go-birthday/internal/config"
)

// VCardFetcher defines the contract for retrieving vCard data.
// This interface allows for mocking in tests and decoupling from the network layer.
type VCardFetcher interface {
	Fetch(ctx context.Context, url, user, pass string) (io.ReadCloser, error)
}

// HTTPFetcher implements VCardFetcher using the standard net/http library.
type HTTPFetcher struct {
	Client *http.Client
}

// NewHTTPFetcher creates a new instance of HTTPFetcher with configured timeouts.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		Client: &http.Client{
			Timeout: config.HTTPTimeout,
		},
	}
}

// Fetch retrieves vCard data from a remote URL.
// It sanitizes the URL for logging purposes to avoid leaking sensitive tokens.
// It enforces a maximum response size limit.
func (f *HTTPFetcher) Fetch(ctx context.Context, targetURL, user, pass string) (io.ReadCloser, error) {
	// Parse the URL to validate it and sanitize it for logs.
	u, err := url.Parse(targetURL)
	if err != nil {
		// Use centralized error message for invalid URL structure.
		return nil, fmt.Errorf("%s: %w", config.ErrInvalidURL, err)
	}

	// Security check: ensure strictly HTTP or HTTPS using config constants.
	if u.Scheme != config.SchemeHTTP && u.Scheme != config.SchemeHTTPS {
		return nil, fmt.Errorf("%s: %s", config.ErrProtocol, u.Scheme)
	}

	// Construct a safe URL for logging (stripping query parameters which might contain tokens).
	safeURL := u.Scheme + "://" + u.Host + u.Path

	// Create a logger with context fields.
	log := slog.With(
		slog.String(config.LogKeyComponent, config.CompFetcher),
		slog.String(config.LogKeyURL, safeURL),
	)

	log.Debug("Initiating vCard download")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use the centralized User-Agent string from config to ensure consistency.
	req.Header.Set(config.HeaderUserAgent, config.UserAgent)

	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error during fetch: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() // Ensure we don't leak resources on error.
		log.Warn("Server returned error status",
			slog.Int(config.LogKeyStatus, resp.StatusCode),
		)
		return nil, fmt.Errorf("server returned unexpected status: %d %s", resp.StatusCode, resp.Status)
	}

	log.Info("vCards downloading",
		slog.Int64("content_length", resp.ContentLength),
	)

	// Return a ReadCloser that limits the number of bytes read to protect against large payloads.
	return &limitedReadCloser{
		Reader: io.LimitReader(resp.Body, config.MaxHTTPResponseSize),
		Closer: resp.Body,
	}, nil
}

// limitedReadCloser wraps an io.Reader (Limited) and the original io.Closer.
// This ensures we can close the network connection properly while limiting the read size.
type limitedReadCloser struct {
	io.Reader
	io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (n int, err error) {
	return l.Reader.Read(p)
}

func (l *limitedReadCloser) Close() error {
	return l.Closer.Close()
}
