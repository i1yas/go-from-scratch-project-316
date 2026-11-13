package crawler

import (
	"context"
	"net/http"
	"time"

	"code/internal/engine"
	"code/internal/formatter"
)

var (
	ErrInvalidRootURL        = engine.ErrInvalidRootURL
	ErrHTTPClientNotProvided = engine.ErrHTTPClientNotProvided
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
	if err != nil {
		return nil, err
	}

	return formatter.Format(report, opts.IndentJSON)
}
