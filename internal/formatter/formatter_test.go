package formatter

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"code/internal/engine"
)

func TestFormatReport(t *testing.T) {
	cases := []struct {
		name  string
		input engine.Report
		want  []byte
	}{
		{
			name: "basic case",
			input: engine.Report{
				RootURL:     testURL(t, "http://test.com"),
				Depth:       10,
				GeneratedAt: testTime(t),
				Pages: []engine.PageReport{
					{
						URL:        testURL(t, "http://test.com"),
						Depth:      0,
						HTTPStatus: 200,
						Status:     "ok",
						Error:      nil,
						SEO: engine.PageSEO{
							HasTitle:       true,
							Title:          "Test title",
							HasDescription: true,
							Description:    "Test description",
							HasH1:          true,
						},
						Assets: map[string]*engine.Asset{
							"http://test.com/static/style.css": {
								URL:        testURL(t, "http://test.com/static/style.css"),
								Type:       "style",
								SizeBytes:  456,
								StatusCode: 200,
								Error:      nil,
							},
							"http://test.com/static/script.js": {
								URL:        testURL(t, "http://test.com/static/script.js"),
								Type:       "script",
								SizeBytes:  123,
								StatusCode: 200,
								Error:      nil,
							},
						},
						BrokenLinks:  map[string]*engine.BrokenLink{},
						DiscoveredAt: testTime(t),
					},
					{
						URL:          testURL(t, "http://test.com/empty"),
						Depth:        1,
						HTTPStatus:   200,
						Status:       "ok",
						Error:        nil,
						SEO:          engine.PageSEO{},
						Assets:       map[string]*engine.Asset{},
						BrokenLinks:  map[string]*engine.BrokenLink{},
						DiscoveredAt: testTime(t),
					},
				},
			},
			want: loadFixture(t, "basic-case-report.json"),
		},
		{
			name: "page with broken links",
			input: engine.Report{
				RootURL:     testURL(t, "http://test.com"),
				Depth:       10,
				GeneratedAt: testTime(t),
				Pages: []engine.PageReport{
					{
						URL:        testURL(t, "http://test.com"),
						Depth:      0,
						HTTPStatus: 200,
						Status:     "ok",
						Error:      nil,
						Assets:     map[string]*engine.Asset{},
						BrokenLinks: map[string]*engine.BrokenLink{
							"http://test.com/static/style.css": {
								URL:        testURL(t, "http://test.com/static/style.css"),
								StatusCode: 404,
								Error:      nil,
							},
							"http://test.com/static/script.js": {
								URL:        testURL(t, "http://test.com/static/script.js"),
								StatusCode: 0,
								Error:      errors.New("some error details"),
							},
						},
						DiscoveredAt: testTime(t),
					},
				},
			},
			want: loadFixture(t, "page-with-broken-links-report.json"),
		},
		{
			name: "page error",
			input: engine.Report{
				RootURL:     testURL(t, "http://test.com"),
				Depth:       10,
				GeneratedAt: testTime(t),
				Pages: []engine.PageReport{
					{
						URL:          testURL(t, "http://test.com"),
						Depth:        0,
						Status:       "error",
						Error:        errors.New("some error details"),
						DiscoveredAt: testTime(t),
					},
				},
			},
			want: loadFixture(t, "page-error-report.json"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Format(tc.input, false)
			require.NoError(t, err)

			require.JSONEq(t, string(tc.want), string(got))
		})
	}
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	fixturePath := filepath.Join("testdata", "fixture", name)

	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	return data
}

func testURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	return u
}

func testTime(t *testing.T) time.Time {
	t.Helper()

	tt, err := time.Parse(time.RFC3339, "2024-06-01T12:34:56Z")
	require.NoError(t, err)

	return tt
}
