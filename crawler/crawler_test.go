package crawler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"code/internal/formatter"
	tu "code/internal/testutils"
)

func TestAnalyze_WellFormedPage(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"
	url := "http://" + host

	server := tu.SetupTestServer(t, host,
		map[string]tu.TestServerRoute{
			"/":        {Status: 200, Body: pageContent},
			"/working": {Status: 200},
			"/broken":  {Status: 500},
		},
		nil,
	)

	want := formatter.Report{
		RootURL: url,
		Depth:   1,
		Pages: []formatter.PageReport{
			{
				URL:        url,
				Depth:      0,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: formatter.PageSEO{
					HasTitle:       true,
					HasDescription: true,
					HasH1:          true,
					Title:          "Test <title>",
					Description:    `test <meta name="description">`,
				},
				BrokenLinks: []formatter.BrokenLink{
					{
						URL:        "http://test.com/broken",
						StatusCode: 500,
					},
					{
						URL:        "https://example.com/page?param1=x&param2=y",
						StatusCode: 404,
					},
				},
				Error: "",
			},
			{
				URL:        url + "/working",
				Depth:      1,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: formatter.PageSEO{
					HasTitle:       false,
					HasDescription: false,
					HasH1:          false,
				},
				Error: "",
			},
		},
	}

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        url,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	requireReportEquals(t, want, gotRaw)
}

func TestAnalyze_RestrictDepth(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"
	url := "http://" + host

	server := tu.SetupTestServer(t, host,
		map[string]tu.TestServerRoute{
			"/":        {Status: 200, Body: pageContent},
			"/working": {Status: 200},
			"/broken":  {Status: 500},
		},
		nil,
	)

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        url,
		Depth:      0,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	var got formatter.Report

	err = json.Unmarshal(gotRaw, &got)
	require.NoError(t, err)

	require.Equal(t, 1, len(got.Pages), "page count")
}

func TestAnalyze_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")
	url := "http://test.com"

	server := tu.SetupTestServer(t, "test.com",
		map[string]tu.TestServerRoute{
			"/": {Status: 200, Body: pageContent},
		},
		nil,
	)

	want := formatter.Report{
		RootURL: url,
		Depth:   1,
		Pages: []formatter.PageReport{
			{
				URL:        url,
				Depth:      0,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: formatter.PageSEO{
					HasTitle: true,
					Title:    "Document",
				},
				Error: "",
			},
		},
	}

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        url,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	requireReportEquals(t, want, gotRaw)
}

func TestAnalyze_NetworkError(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("hijacking not supported")
		}

		conn, _, err := hj.Hijack()
		if err != nil {
			t.Error(err)
			return
		}

		require.NoError(t, conn.Close())
	}))

	client := server.Client()

	url := "http://test.com"

	want := formatter.Report{
		RootURL: url,
		Depth:   1,
		Pages: []formatter.PageReport{
			{
				URL:   url,
				Depth: 0,
				Error: io.EOF.Error(),
			},
		},
	}

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        url,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: client,
	})
	require.NoError(t, err)

	requireReportEquals(t, want, gotRaw)
}

func TestAnalyze_ErrorOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := httptest.NewTestServer(
		t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	_, err := Analyze(ctx, Options{
		URL:        "http://test.com",
		HTTPClient: server.Client(),
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestAnalyze_InvalidOptions(t *testing.T) {
	t.Run("invalid root url", func(t *testing.T) {
		server := httptest.NewTestServer(
			t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		)

		_, err := Analyze(t.Context(), Options{
			URL:        "invalid",
			HTTPClient: server.Client(),
		})
		require.ErrorIs(t, err, ErrInvalidRootURL)
	})

	t.Run("missing http client", func(t *testing.T) {
		_, err := Analyze(t.Context(), Options{
			URL: "http://test.com",
		})
		require.ErrorIs(t, err, ErrHTTPClientNotProvided)
	})
}

func requireReportEquals(t *testing.T, want formatter.Report, gotRaw []byte) {
	t.Helper()

	var got formatter.Report

	err := json.Unmarshal(gotRaw, &got)
	require.NoError(t, err)

	require.Equal(t, want.RootURL, got.RootURL)
	require.Equal(t, want.Depth, got.Depth)
	requireTimeIsValid(t, got.GeneratedAt)

	require.Equal(t, len(want.Pages), len(got.Pages), "amount of pages")

	slices.SortFunc(got.Pages, comparePageReports)
	slices.SortFunc(want.Pages, comparePageReports)

	for i := range want.Pages {
		wantPage := want.Pages[i]
		gotPage := got.Pages[i]

		assert.Equal(t, wantPage.URL, gotPage.URL, "url")
		assert.Equal(t, wantPage.Depth, gotPage.Depth, "depth")
		assert.Contains(t, gotPage.Error, wantPage.Error, "error")
		assert.Equal(t, wantPage.HTTPStatus, gotPage.HTTPStatus, "http status")
		assert.Equal(t, wantPage.Status, gotPage.Status, "status")
		assert.Equal(t, wantPage.SEO, gotPage.SEO, "seo")

		require.ElementsMatch(t, wantPage.BrokenLinks, gotPage.BrokenLinks, "broken links")

		requireTimeIsValid(t, gotPage.DiscoveredAt)
	}
}

func comparePageReports(a, b formatter.PageReport) int {
	return strings.Compare(a.URL, b.URL)
}

func requireTimeIsValid(t *testing.T, value string) {
	t.Helper()

	_, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", "fixture", name)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}
