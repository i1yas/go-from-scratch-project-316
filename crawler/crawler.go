package crawler

import (
	"context"
	"time"

	"code/internal/engine"
	"code/internal/formatter"
)

var (
	ErrInvalidRootURL        = engine.ErrInvalidRootURL
	ErrHTTPClientNotProvided = engine.ErrHTTPClientNotProvided
)

// Options contains options for crawling
type Options engine.Options

// Analyze runs crawler and returns JSON report
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	e, err := engine.New(engine.Options(opts))
	if err != nil {
		return nil, err
	}

	pages, err := e.AnalyzePage(ctx, opts.URL)
	if err != nil {
		return nil, err
	}

	report := engine.Report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now(),
		Pages:       pages,
	}

	return formatter.Format(report)
}
