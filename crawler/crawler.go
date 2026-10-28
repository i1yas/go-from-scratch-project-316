package crawler

import (
	"code/internal/weburl"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

var (
	ErrInvalidRootURL = errors.New("invalid root url")
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
		URL:   pageURL,
		Depth: 0,
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

type reportDTO struct {
	RootURL     string       `json:"root_url"`
	Depth       int          `json:"depth"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []pageReport `json:"pages"`
}

type pageReport struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}

func statusText(resp *http.Response) string {
	for i, r := range resp.Status {
		if unicode.IsSpace(r) {
			return strings.ToLower(resp.Status[i+1:])
		}
	}

	return ""
}

func formatReport(report report) ([]byte, error) {
	reportDTO := reportDTO{
		RootURL:     report.RootURL,
		Depth:       report.Depth,
		GeneratedAt: report.GeneratedAt.UTC().Format(time.RFC3339),
		Pages:       report.Pages,
	}

	reportJSON, err := json.MarshalIndent(reportDTO, "", "  ")
	if err != nil {
		return nil, err
	}

	return reportJSON, nil
}
