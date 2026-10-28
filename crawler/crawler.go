package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode"
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
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		break
	}

	report := report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now(),
		Pages:       []pageReport{},
	}

	client := opts.HTTPClient
	client.Timeout = opts.Timeout

	page := pageReport{
		URL:   opts.URL,
		Depth: 0,
	}

	req, err := http.NewRequest("GET", opts.URL, nil)
	if err != nil {
		return nil, err
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		page.Error = err.Error()
	} else {
		page.HTTPStatus = resp.StatusCode
		page.Status = statusText(resp)
	}

	report.Pages = append(report.Pages, page)

	return formatReport(report)
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
