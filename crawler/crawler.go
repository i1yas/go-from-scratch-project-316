package crawler

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Options contains options for crawling
type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	IndentJSON  bool
	HTTPClient  *http.Client
}

// Analyze runs crawler and returns JSON report
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	return []byte{}, errors.New("TODO: implement Analyze function")
}
