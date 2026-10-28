package crawler

import (
	"encoding/json"
	"time"
)

type reportDTO struct {
	RootURL     string          `json:"root_url"`
	Depth       int             `json:"depth"`
	GeneratedAt string          `json:"generated_at"`
	Pages       []pageReportDTO `json:"pages"`
}

type pageReportDTO struct {
	URL          string `json:"url"`
	Depth        int    `json:"depth"`
	HTTPStatus   int    `json:"http_status"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	DiscoveredAt string `json:"discovered_at"`
}

func formatReport(report report) ([]byte, error) {
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

func toPageReportDTO(page pageReport) pageReportDTO {
	return pageReportDTO{
		URL:          page.URL,
		Depth:        page.Depth,
		HTTPStatus:   page.HTTPStatus,
		Status:       page.Status,
		Error:        page.Error,
		DiscoveredAt: formatTime(page.DiscoveredAt),
	}
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
