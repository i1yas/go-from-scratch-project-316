package formatter

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"code/internal/engine"
)

// Report contains formatted report
type Report struct {
	RootURL     string       `json:"root_url"`
	Depth       int          `json:"depth"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []PageReport `json:"pages"`
}

// PageReport contains formatted page report
type PageReport struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status,omitempty"`
	Error        string       `json:"error,omitempty"`
	SEO          PageSEO      `json:"seo"`
	Assets       []Asset      `json:"assets,omitempty"`
	BrokenLinks  []BrokenLink `json:"broken_links,omitempty"`
	DiscoveredAt string       `json:"discovered_at"`
}

// PageSEO contains formatted page seo
type PageSEO struct {
	HasTitle       bool   `json:"has_title"`
	HasDescription bool   `json:"has_description"`
	HasH1          bool   `json:"has_h1"`
	Title          string `json:"title"`
	Description    string `json:"description"`
}

// BrokenLink contains formatted info about broken link
type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Asset contains formatted info about asset
type Asset struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	SizeBytes  int64  `json:"size_bytes"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Format format Report from engine
func Format(report engine.Report, isIndented bool) ([]byte, error) {
	reportDTO := convertReport(report)

	if !isIndented {
		reportJSON, err := json.Marshal(reportDTO)
		if err != nil {
			return nil, err
		}

		return reportJSON, nil
	}

	reportJSON, err := json.MarshalIndent(reportDTO, "", "  ")
	if err != nil {
		return nil, err
	}

	return reportJSON, nil
}

func convertReport(report engine.Report) Report {
	pages := make([]PageReport, len(report.Pages))

	for i, page := range report.Pages {
		pages[i] = convertPageReport(page)
	}

	slices.SortFunc(pages, comparePages)

	return Report{
		RootURL:     report.RootURL.String(),
		Depth:       report.Depth,
		GeneratedAt: formatTime(report.GeneratedAt),
		Pages:       pages,
	}
}

func convertPageReport(page engine.PageReport) PageReport {
	assets := make([]Asset, 0, len(page.Assets))
	brokenLinks := make([]BrokenLink, 0, len(page.BrokenLinks))

	for _, asset := range page.Assets {
		assets = append(assets, convertAsset(*asset))
	}

	for _, bl := range page.BrokenLinks {
		brokenLinks = append(brokenLinks, convertBrokenLink(*bl))
	}

	seo := PageSEO{
		HasTitle:       page.SEO.HasTitle,
		HasDescription: page.SEO.HasDescription,
		HasH1:          page.SEO.HasH1,
		Title:          page.SEO.Title,
		Description:    page.SEO.Description,
	}

	slices.SortFunc(assets, compareAssets)
	slices.SortFunc(brokenLinks, compareBrokenLinks)

	report := PageReport{
		URL:          page.URL.String(),
		Depth:        page.Depth,
		HTTPStatus:   page.HTTPStatus,
		Status:       page.Status,
		SEO:          seo,
		Assets:       assets,
		BrokenLinks:  brokenLinks,
		DiscoveredAt: formatTime(page.DiscoveredAt),
	}

	if page.Error != nil {
		report.Error = page.Error.Error()
	}

	return report
}

func convertAsset(asset engine.Asset) Asset {
	result := Asset{
		URL:        asset.URL.String(),
		Type:       string(asset.Type),
		StatusCode: asset.StatusCode,
		SizeBytes:  asset.SizeBytes,
	}

	if asset.Error != nil {
		result.Error = asset.Error.Error()
	}

	return result
}

func convertBrokenLink(bl engine.BrokenLink) BrokenLink {
	result := BrokenLink{
		URL:        bl.URL.String(),
		StatusCode: bl.StatusCode,
	}

	if bl.Error != nil {
		result.Error = bl.Error.Error()
	}

	return result
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func comparePages(a, b PageReport) int {
	return strings.Compare(a.URL, b.URL)
}

func compareAssets(a, b Asset) int {
	return strings.Compare(a.URL, b.URL)
}

func compareBrokenLinks(a, b BrokenLink) int {
	return strings.Compare(a.URL, b.URL)
}
