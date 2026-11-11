package engine

import (
	"net/url"
	"time"

	"code/internal/htmlparser"
)

// Report contains crawling report
type Report struct {
	RootURL     string
	Depth       int
	GeneratedAt time.Time
	Pages       []PageReport
}

func (r Report) findPageByRawURL(pageURL string) (PageReport, bool) {
	for _, p := range r.Pages {
		if p.URL.String() == pageURL {
			return p, true
		}
	}

	return PageReport{}, false
}

// PageReport contains page report
type PageReport struct {
	URL          *url.URL
	Depth        int
	HTTPStatus   int
	Status       string
	Error        error
	SEO          PageSEO
	Assets       map[string]*Asset
	BrokenLinks  []BrokenLink
	DiscoveredAt time.Time
	isIncluded   bool
}

// BrokenLink contains information about broken link
type BrokenLink struct {
	URL        string
	StatusCode int
	Error      error
}

// AssetType is asset type: image, style, script or other
type AssetType string

const (
	// AssetTypeImage is type for image asset
	AssetTypeImage AssetType = "image"
	// AssetTypeStyle is type CSS style
	AssetTypeStyle AssetType = "style"
	// AssetTypeScript is type for js script asset
	AssetTypeScript AssetType = "script"
	// AssetTypeOther is unknown asset type
	AssetTypeOther AssetType = "other"
)

// Asset contains information about asset from page
type Asset struct {
	URL        *url.URL
	Type       AssetType
	StatusCode int
	SizeBytes  int64
	Error      error
}

func linkTypeToAssetType(linkType htmlparser.LinkType) AssetType {
	switch linkType {
	case htmlparser.LinkTypeImage:
		return AssetTypeImage
	case htmlparser.LinkTypeStyle:
		return AssetTypeStyle
	case htmlparser.LinkTypeScript:
		return AssetTypeScript
	default:
		return AssetTypeOther
	}
}
