package engine

import (
	"net/url"
	"time"
)

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
	SEO          PageSEO
	BrokenLinks  []BrokenLink
	DiscoveredAt time.Time
}

// BrokenLink contains information about broken link
type BrokenLink struct {
	URL        string
	StatusCode int
	Err        error
}
