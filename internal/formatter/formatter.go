package formatter

import (
	"encoding/json"
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
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	SEO          PageSEO      `json:"seo"`
	BrokenLinks  []BrokenLink `json:"broken_links"`
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
	Err        string `json:"error,omitempty"`
}

// Format format Report from engine
func Format(report engine.Report) ([]byte, error) {
	pages := make([]PageReport, len(report.Pages))

	for i, page := range report.Pages {
		pages[i] = toPageReport(page)
	}

	reportDTO := Report{
		RootURL:     report.RootURL,
		Depth:       report.Depth,
		GeneratedAt: formatTime(report.GeneratedAt),
		Pages:       pages,
	}

	reportJSON, err := json.MarshalIndent(reportDTO, "", "  ")
	if err != nil {
		return nil, err
	}

	return reportJSON, nil
}

func toPageReport(page engine.PageReport) PageReport {
	brokenLinks := make([]BrokenLink, len(page.BrokenLinks))

	for i, bl := range page.BrokenLinks {
		brokenLinks[i] = toBrokenLink(bl)
	}

	seo := PageSEO{
		HasTitle:       page.SEO.HasTitle,
		HasDescription: page.SEO.HasDescription,
		HasH1:          page.SEO.HasH1,
		Title:          page.SEO.Title,
		Description:    page.SEO.Description,
	}

	return PageReport{
		URL:          page.URL.String(),
		Depth:        page.Depth,
		HTTPStatus:   page.HTTPStatus,
		Status:       page.Status,
		Error:        page.Error,
		SEO:          seo,
		BrokenLinks:  brokenLinks,
		DiscoveredAt: formatTime(page.DiscoveredAt),
	}
}

func toBrokenLink(bl engine.BrokenLink) BrokenLink {
	err := ""
	if bl.Err != nil {
		err = bl.Err.Error()
	}

	return BrokenLink{
		URL:        bl.URL,
		StatusCode: bl.StatusCode,
		Err:        err,
	}
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
