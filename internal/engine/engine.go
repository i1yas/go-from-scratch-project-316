package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"code/internal/htmlparser"
	"code/internal/weburl"
)

var (
	ErrInvalidRootURL        = errors.New("invalid root url")
	ErrHTTPClientNotProvided = errors.New("http client not provided")
)

// Options contains engine options
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

// Engine is crawler engine
type Engine struct {
	rootURL   *url.URL
	userAgent string
	client    *http.Client
	depth     int
}

// New constructs new crawler engine
func New(opts Options) (*Engine, error) {
	rootURL, err := weburl.Parse(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("%w '%s': %w", ErrInvalidRootURL, opts.URL, err)
	}

	if opts.HTTPClient == nil {
		return nil, ErrHTTPClientNotProvided
	}

	client := opts.HTTPClient
	client.Timeout = opts.Timeout

	return &Engine{
		rootURL:   rootURL,
		userAgent: opts.UserAgent,
		client:    client,
		depth:     opts.Depth,
	}, nil
}

// Analyze crawls website
func (e *Engine) Analyze(ctx context.Context) (Report, error) {
	pages, err := e.analyzePage(ctx, e.rootURL)
	if err != nil {
		return Report{}, err
	}

	return Report{
		RootURL:     e.rootURL.String(),
		Depth:       e.depth,
		GeneratedAt: time.Now(),
		Pages:       pages,
	}, nil
}

func (e *Engine) analyzePage(ctx context.Context, pageURL *url.URL) ([]PageReport, error) {
	pages := []PageReport{}
	page := PageReport{
		URL:          pageURL,
		Depth:        0,
		DiscoveredAt: time.Now(),
	}

	resp, err := e.fetch(ctx, pageURL)

	switch {
	case errors.Is(err, context.Canceled):
		return nil, err
	case err != nil:
		page.Error = err.Error()
	default:
		page.HTTPStatus = resp.StatusCode
		page.Status = statusText(resp)

		respContentType := resp.Header.Get("Content-Type")
		isHTML := strings.HasPrefix(respContentType, "text/html")

		if isHTML {
			e.processDocument(resp.Body, &page)
		}
	}

	pages = append(pages, page)

	return pages, nil
}

func (e *Engine) fetch(ctx context.Context, pageURL *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL.String(), nil)
	if err != nil {
		return nil, err
	}

	if e.userAgent != "" {
		req.Header.Set("User-Agent", e.userAgent)
	}

	return e.client.Do(req)
}

func (e *Engine) processDocument(rawDocument io.Reader, page *PageReport) {
	doc, err := goquery.NewDocumentFromReader(rawDocument)
	if err != nil {
		return
	}

	links, err := htmlparser.ExtractLinks(doc, e.rootURL)
	if err != nil {
		return
	}

	page.SEO = htmlparser.AnalyzeSEO(doc)

	for _, link := range links {
		resp, err := e.client.Get(link.String())
		if err != nil {
			page.BrokenLinks = append(page.BrokenLinks, BrokenLink{
				URL: link.String(),
				Err: err,
			})

			continue
		}

		if resp.StatusCode >= 400 {
			page.BrokenLinks = append(page.BrokenLinks, BrokenLink{
				URL:        link.String(),
				StatusCode: resp.StatusCode,
			})
			continue
		}
	}
}

// Report contains crawling report
type Report struct {
	RootURL     string
	Depth       int
	GeneratedAt time.Time
	Pages       []PageReport
}

// PageReport contains page report
type PageReport struct {
	URL          *url.URL
	Depth        int
	HTTPStatus   int
	Status       string
	Error        string
	SEO          htmlparser.SEO
	BrokenLinks  []BrokenLink
	DiscoveredAt time.Time
}

// BrokenLink contains information about broken link
type BrokenLink struct {
	URL        string
	StatusCode int
	Err        error
}

func statusText(resp *http.Response) string {
	for i, r := range resp.Status {
		if unicode.IsSpace(r) {
			return strings.ToLower(resp.Status[i+1:])
		}
	}

	return ""
}
