package crawler

import (
	"code/internal/weburl"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

var (
	ErrInvalidRootURL        = errors.New("invalid root url")
	ErrHTTPClientNotProvided = errors.New("http client not provided")
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
	crw, err := newCrawler(opts)
	if err != nil {
		return nil, err
	}

	pages, err := crw.fetch(ctx, opts.URL)
	if err != nil {
		return nil, err
	}

	report := report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now(),
		Pages:       pages,
	}

	return formatReport(report)
}

type crawler struct {
	rootURL   *url.URL
	userAgent string
	client    *http.Client
}

func newCrawler(opts Options) (*crawler, error) {
	rootURL, err := weburl.Parse(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("%w '%s': %w", ErrInvalidRootURL, opts.URL, err)
	}

	if opts.HTTPClient == nil {
		return nil, ErrHTTPClientNotProvided
	}

	client := opts.HTTPClient
	client.Timeout = opts.Timeout

	return &crawler{
		rootURL:   rootURL,
		userAgent: opts.UserAgent,
		client:    client,
	}, nil
}

func (crw *crawler) fetch(ctx context.Context, pageURL string) ([]pageReport, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}

	if crw.userAgent != "" {
		req.Header.Set("User-Agent", crw.userAgent)
	}

	page := pageReport{
		URL:          pageURL,
		Depth:        0,
		DiscoveredAt: time.Now(),
	}

	resp, err := crw.client.Do(req)

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}

	if err != nil {
		page.Error = err.Error()
	} else {
		page.HTTPStatus = resp.StatusCode
		page.Status = statusText(resp)
	}

	return []pageReport{page}, nil
}

type report struct {
	RootURL     string
	Depth       int
	GeneratedAt time.Time
	Pages       []pageReport
}

type pageReport struct {
	URL          string
	Depth        int
	HTTPStatus   int
	Status       string
	Error        string
	DiscoveredAt time.Time
}

func statusText(resp *http.Response) string {
	for i, r := range resp.Status {
		if unicode.IsSpace(r) {
			return strings.ToLower(resp.Status[i+1:])
		}
	}

	return ""
}
