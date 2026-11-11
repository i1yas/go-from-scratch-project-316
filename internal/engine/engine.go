package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

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
	rootURL         *url.URL
	userAgent       string
	delay           time.Duration
	client          *http.Client
	depth           int
	retries         int
	pages           map[string]*PageReport
	brokenLinks     map[string]*BrokenLink
	lastRequestTime *time.Time
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
		rootURL:     rootURL,
		userAgent:   opts.UserAgent,
		delay:       opts.Delay,
		client:      client,
		depth:       opts.Depth,
		retries:     opts.Retries,
		pages:       map[string]*PageReport{},
		brokenLinks: map[string]*BrokenLink{},
	}, nil
}

// Analyze crawls website
func (e *Engine) Analyze(ctx context.Context) (Report, error) {
	err := e.analyzePage(ctx, e.rootURL)
	if err != nil && !errors.Is(err, context.Canceled) {
		return Report{}, err
	}

	pages := make([]PageReport, 0)

	for _, page := range e.pages {
		if page.isIncluded {
			pages = append(pages, *page)
		}
	}

	return Report{
		RootURL:     e.rootURL.String(),
		Depth:       e.depth,
		GeneratedAt: time.Now(),
		Pages:       pages,
	}, err
}

func (e *Engine) analyzePage(ctx context.Context, pageURL *url.URL) error {
	_, pageIsAnalyzed := e.pages[pageURL.String()]
	if pageIsAnalyzed {
		return nil
	}

	parent := parentPageFromContext(ctx)

	depth := 0
	if parent != nil {
		depth = parent.Depth + 1
	}

	page := PageReport{
		URL:          pageURL,
		Depth:        depth,
		DiscoveredAt: time.Now(),
	}
	e.pages[pageURL.String()] = &page

	resp, err := e.fetch(ctx, pageURL)

	switch {
	case errors.Is(err, context.Canceled):
		return err
	case err != nil:
		if parent == nil {
			page.Error = err.Error()
			page.isIncluded = true

			return nil
		}

		bl := BrokenLink{
			URL: page.URL.String(),
			Err: err,
		}
		e.brokenLinks[pageURL.String()] = &bl
		parent.BrokenLinks = append(parent.BrokenLinks, bl)

		return nil
	default:
		return e.processPageResponse(
			ctx,
			&page,
			resp,
		)
	}
}

func (e *Engine) fetch(ctx context.Context, pageURL *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL.String(), nil)
	if err != nil {
		return nil, err
	}

	if e.userAgent != "" {
		req.Header.Set("User-Agent", e.userAgent)
	}

	retriesLeft := e.retries

	for {
		e.waitRequestDelay(ctx)

		t := time.Now()
		e.lastRequestTime = &t

		resp, err := e.client.Do(req)
		if errors.Is(err, context.Canceled) {
			return nil, err
		}

		shouldRetry := retriesLeft > 0 &&
			(err != nil || isRetriableStatus(resp.StatusCode))

		if shouldRetry {
			retriesLeft--
			continue
		}

		return resp, err
	}
}

func (e *Engine) waitRequestDelay(ctx context.Context) {
	if e.lastRequestTime == nil {
		return
	}

	delay := e.delay - time.Since(*e.lastRequestTime)
	if delay < 0 {
		return
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
		return
	}
}

func (e *Engine) processPageResponse(
	ctx context.Context,
	page *PageReport,
	resp *http.Response,
) error {
	parent := parentPageFromContext(ctx)
	pageURL := page.URL

	if resp.StatusCode >= 400 {
		if parent == nil {
			page.HTTPStatus = resp.StatusCode
			page.Status = statusText(resp.Status)
			page.isIncluded = true

			return nil
		}

		bl := BrokenLink{
			URL:        pageURL.String(),
			StatusCode: resp.StatusCode,
		}
		e.brokenLinks[pageURL.String()] = &bl
		parent.BrokenLinks = append(parent.BrokenLinks, bl)

		return nil
	}

	if e.isForeignLink(pageURL) {
		return nil
	}

	page.HTTPStatus = resp.StatusCode
	page.Status = statusText(resp.Status)

	respContentType := resp.Header.Get("Content-Type")
	if !isHTMLContentType(respContentType) {
		page.isIncluded = true
		return nil
	}

	err := e.processDocument(ctx, resp.Body, page)
	if errors.Is(err, context.Canceled) {
		page.isIncluded = true
		return err
	}

	if err != nil {
		return err
	}

	page.isIncluded = true

	return nil
}

func (e *Engine) processDocument(ctx context.Context, rawDocument io.Reader, page *PageReport) error {
	doc, err := goquery.NewDocumentFromReader(rawDocument)
	if err != nil {
		return nil
	}

	page.SEO = analyzeSEO(doc)

	if page.Depth == e.depth {
		return nil
	}

	links, err := htmlparser.ExtractLinks(doc, e.rootURL)
	if err != nil {
		return nil
	}

	for _, link := range links {
		bl, isBroken := e.brokenLinks[link.URL.String()]
		if isBroken {
			page.BrokenLinks = append(page.BrokenLinks, *bl)
			continue
		}

		if link.Type != htmlparser.LinkTypeAnchor {
			continue
		}

		err := e.analyzePage(withParentPage(ctx, page), link.URL)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) isForeignLink(link *url.URL) bool {
	return e.rootURL.Host != link.Host
}

type parentPageKey struct{}

func withParentPage(ctx context.Context, page *PageReport) context.Context {
	return context.WithValue(ctx, parentPageKey{}, page)
}

func parentPageFromContext(ctx context.Context) *PageReport {
	page, ok := ctx.Value(parentPageKey{}).(*PageReport)
	if !ok {
		return nil
	}

	return page
}
