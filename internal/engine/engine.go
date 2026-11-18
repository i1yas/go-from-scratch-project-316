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

var ErrInvalidRootURL = errors.New("invalid root url")

// Engine is crawler engine
type Engine struct {
	rootURL         *url.URL
	userAgent       string
	delay           time.Duration
	depth           int
	retries         int
	pages           map[string]*PageReport
	assets          map[string]*Asset
	brokenLinks     map[string]*BrokenLink
	lastRequestTime *time.Time

	client *http.Client
	clock  func() time.Time
}

// New constructs new crawler engine
func New(opts Options) (*Engine, error) {
	if opts.Concurrency == 0 {
		opts.Concurrency = 1
	}

	if opts.Clock == nil {
		opts.Clock = time.Now
	}

	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	rootURL, err := weburl.Parse(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("%w '%s': %w", ErrInvalidRootURL, opts.URL, err)
	}

	client := opts.HTTPClient
	client.Timeout = opts.Timeout

	return &Engine{
		rootURL:     rootURL,
		userAgent:   opts.UserAgent,
		delay:       opts.Delay,
		depth:       opts.Depth,
		retries:     opts.Retries,
		pages:       map[string]*PageReport{},
		assets:      map[string]*Asset{},
		brokenLinks: map[string]*BrokenLink{},
		client:      client,
		clock:       opts.Clock,
	}, nil
}

// Analyze crawls website
func (e *Engine) Analyze(ctx context.Context) (Report, error) {
	err := e.analyzeLink(ctx, pageLink{
		url: e.rootURL,
		typ: htmlparser.LinkTypeAnchor,
	})
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
		RootURL:     e.rootURL,
		Depth:       e.depth,
		GeneratedAt: e.clock(),
		Pages:       pages,
	}, err
}

type pageLink struct {
	url    *url.URL
	typ    htmlparser.LinkType
	parent *PageReport
}

func (e *Engine) analyzeLink(ctx context.Context, ln pageLink) error {
	switch ln.typ {
	case htmlparser.LinkTypeAnchor:
		depth := 0
		if ln.parent != nil {
			depth = ln.parent.Depth + 1
		}

		return e.analyzePage(ctx, &PageReport{
			URL:          ln.url,
			Depth:        depth,
			DiscoveredAt: e.clock(),
			Assets:       map[string]*Asset{},
			BrokenLinks:  map[string]*BrokenLink{},
			parent:       ln.parent,
		})
	default:
		return e.analyzeAsset(ctx, &Asset{
			URL:    ln.url,
			Type:   linkTypeToAssetType(ln.typ),
			parent: ln.parent,
		})
	}
}

func (e *Engine) analyzePage(ctx context.Context, page *PageReport) error {
	pageURL := page.URL
	parent := page.parent

	analyzedPage, pageIsAnalyzed := e.pages[pageURL.String()]
	if pageIsAnalyzed {
		analyzedPage.Depth = min(analyzedPage.Depth, page.Depth)
		return nil
	}

	e.pages[pageURL.String()] = page

	resp, err := e.fetch(ctx, pageURL)

	defer func() {
		if resp != nil {
			resp.Body.Close() // nolint: errcheck
		}
	}()

	switch {
	case errors.Is(err, context.Canceled):
		return err
	case err != nil:
		if parent == nil {
			page.Error = err
			page.Status = "error"
			page.isIncluded = true

			page.Assets = nil
			page.BrokenLinks = nil

			return nil
		}

		bl := BrokenLink{
			URL:   page.URL,
			Error: err,
		}
		e.brokenLinks[pageURL.String()] = &bl
		parent.BrokenLinks[pageURL.String()] = &bl

		return nil
	default:
		return e.processPageResponse(
			ctx,
			page,
			resp,
		)
	}
}

func (e *Engine) analyzeAsset(ctx context.Context, asset *Asset) error {
	foundAsset, ok := e.assets[asset.URL.String()]
	if ok {
		asset.parent.Assets[asset.URL.String()] = foundAsset
		return nil
	}

	resp, err := e.fetch(ctx, asset.URL)

	e.assets[asset.URL.String()] = asset

	switch {
	case errors.Is(err, context.Canceled):
		return err
	case err != nil:
		asset.Error = err
		asset.parent.Assets[asset.URL.String()] = asset

		return nil
	default:
		bodySize, err := responseBodySize(resp)
		if err != nil {
			asset.Error = fmt.Errorf("failed to calculate response body size: %w", err)
			asset.parent.Assets[asset.URL.String()] = asset

			return nil
		}

		asset.SizeBytes = bodySize

		asset.StatusCode = resp.StatusCode

		asset.parent.Assets[asset.URL.String()] = asset

		return nil
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

		t := e.clock()
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

	delay := e.delay - e.clock().Sub(*e.lastRequestTime)
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
	parent := page.parent
	pageURL := page.URL

	if resp.StatusCode >= 400 {
		if parent == nil {
			page.HTTPStatus = resp.StatusCode
			page.Status = statusText(resp.Status)
			page.isIncluded = true

			return nil
		}

		bl := BrokenLink{
			URL:        pageURL,
			StatusCode: resp.StatusCode,
		}
		e.brokenLinks[pageURL.String()] = &bl
		parent.BrokenLinks[pageURL.String()] = &bl

		return nil
	}

	if e.isForeignLink(pageURL) {
		return nil
	}

	page.HTTPStatus = resp.StatusCode
	page.Status = statusText(resp.Status)

	respContentType := resp.Header.Get("Content-Type")

	docType, isDocument := documentTypeFromContentType(respContentType)
	if !isDocument {
		page.isIncluded = true
		return nil
	}

	err := e.processDocument(ctx, resp.Body, docType, page)
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

func (e *Engine) processDocument(
	ctx context.Context,
	rawDocument io.Reader,
	docType documentType,
	page *PageReport,
) error {
	doc, err := goquery.NewDocumentFromReader(rawDocument)
	if err != nil {
		return nil
	}

	page.SEO = analyzeSEO(doc, docType)

	if page.Depth == e.depth {
		return nil
	}

	if docType != documentTypeHTML {
		return nil
	}

	links := htmlparser.ExtractLinks(doc, e.rootURL)

	for _, ln := range links {
		if err := weburl.ValidateURL(ln.URL); err != nil {
			continue
		}

		weburl.NormalizeURL(ln.URL)

		bl, isBroken := e.brokenLinks[ln.URL.String()]
		if isBroken {
			page.BrokenLinks[ln.URL.String()] = bl
			continue
		}

		err := e.analyzeLink(ctx, pageLink{
			url:    ln.URL,
			typ:    ln.Type,
			parent: page,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) isForeignLink(link *url.URL) bool {
	return e.rootURL.Host != link.Host
}
