package crawler

import (
	"context"

	"code/internal/engine"
	"code/internal/formatter"
)

var (
	ErrInvalidRootURL        = engine.ErrInvalidRootURL
	ErrHTTPClientNotProvided = engine.ErrHTTPClientNotProvided
)

// Options contains options for crawling
type Options engine.Options

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

// Analyze runs crawler and returns JSON report
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	e, err := engine.New(engine.Options(opts))
	if err != nil {
		return nil, err
	}

	report, err := e.Analyze(ctx)
	if err != nil {
		return nil, err
	}

	return formatter.Format(report, opts.IndentJSON)
}
