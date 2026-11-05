package crawler

import (
	"encoding/json"
	"time"

	"code/internal/engine"
)

type reportDTO struct {
	RootURL     string          `json:"root_url"`
	Depth       int             `json:"depth"`
	GeneratedAt string          `json:"generated_at"`
	Pages       []pageReportDTO `json:"pages"`
}

type pageReportDTO struct {
	URL          string          `json:"url"`
	Depth        int             `json:"depth"`
	HTTPStatus   int             `json:"http_status"`
	Status       string          `json:"status"`
	Error        string          `json:"error"`
	SEO          pageSEODTO      `json:"seo"`
	BrokenLinks  []brokenLinkDTO `json:"broken_links"`
	DiscoveredAt string          `json:"discovered_at"`
}

type pageSEODTO struct {
	HasTitle       bool   `json:"has_title"`
	HasDescription bool   `json:"has_description"`
	HasH1          bool   `json:"has_h1"`
	Title          string `json:"title"`
	Description    string `json:"description"`
}

type brokenLinkDTO struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Err        string `json:"error,omitempty"`
}

func formatReport(report engine.Report) ([]byte, error) {
	pages := make([]pageReportDTO, len(report.Pages))

	for i, page := range report.Pages {
		pages[i] = toPageReportDTO(page)
	}

	reportDTO := reportDTO{
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

func toPageReportDTO(page engine.PageReport) pageReportDTO {
	brokenLinks := make([]brokenLinkDTO, len(page.BrokenLinks))

	for i, bl := range page.BrokenLinks {
		brokenLinks[i] = toBrokenLinkDTO(bl)
	}

	seo := pageSEODTO{
		HasTitle:       page.SEO.HasTitle,
		HasDescription: page.SEO.HasDescription,
		HasH1:          page.SEO.HasH1,
		Title:          page.SEO.Title,
		Description:    page.SEO.Description,
	}

	return pageReportDTO{
		URL:          page.URL,
		Depth:        page.Depth,
		HTTPStatus:   page.HTTPStatus,
		Status:       page.Status,
		Error:        page.Error,
		SEO:          seo,
		BrokenLinks:  brokenLinks,
		DiscoveredAt: formatTime(page.DiscoveredAt),
	}
}

func toBrokenLinkDTO(bl engine.BrokenLink) brokenLinkDTO {
	err := ""
	if bl.Err != nil {
		err = bl.Err.Error()
	}

	return brokenLinkDTO{
		URL:        bl.URL,
		StatusCode: bl.StatusCode,
		Err:        err,
	}
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
