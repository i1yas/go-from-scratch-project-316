package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"code/internal/engine"
	"code/internal/formatter"
)

var (
	ErrInvalidRootURL        = engine.ErrInvalidRootURL
	ErrHTTPClientNotProvided = engine.ErrHTTPClientNotProvided
	ErrInvalidDepth          = engine.ErrInvalidDepth
	ErrInvalidRetries        = engine.ErrInvalidRetries
	ErrInvalidDelay          = engine.ErrInvalidDelay
	ErrInvalidTimeout        = engine.ErrInvalidTimeout
	ErrInvalidConcurrency    = engine.ErrInvalidConcurrency
	ErrFailedToFormatReport  = errors.New("failed to format report")
)

// Options contains options for analyzing website
type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	IndentJSON  bool

	HTTPClient *http.Client
	Clock      func() time.Time
}

type (
	// Report contains collected infromation about website
	Report formatter.Report
	// PageReport contains collected information about page
	PageReport formatter.PageReport
	// Asset contains information about asset from page
	Asset formatter.Asset
	// BrokenLink contains information about broken link from page
	BrokenLink formatter.BrokenLink
)

// Analyze runs crawler on website and returns JSON report
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	e, err := engine.New(engine.Options(engine.Options{
		URL:         opts.URL,
		Depth:       opts.Depth,
		Retries:     opts.Retries,
		Delay:       opts.Delay,
		Timeout:     opts.Timeout,
		UserAgent:   opts.UserAgent,
		Concurrency: opts.Concurrency,

		HTTPClient: opts.HTTPClient,
		Clock:      opts.Clock,
	}))
	if err != nil {
		return nil, err
	}

	report, err := e.Analyze(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}

	output, err := formatter.Format(report, opts.IndentJSON)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToFormatReport, err)
	}

	return output, nil
}
