package crawler

import (
	"context"
	"encoding/json"
	"io"
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

const (
	testHost    = "example.com"
	testRootURL = "https://example.com"
	testTime    = "2024-06-01T12:34:56Z"
)

func TestAnalyze_NormalPage(t *testing.T) {
	pageContent := loadFixture(t, "normal-page.html")
	wantReport := loadFixture(t, "normal-page-output.txt")

	wantTime, err := time.Parse(time.RFC3339, testTime)
	require.NoError(t, err)

	server := tu.SetupTestServer(t, testHost,
		map[string]tu.TestServerRoute{
			"/":                {Status: 200, Body: pageContent},
			"/static/logo.png": {Status: 200, Body: make([]byte, 12345)},
		},
		nil,
	)

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        testRootURL,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
		Clock:      func() time.Time { return wantTime },
	})
	require.NoError(t, err)

	require.JSONEq(t, string(wantReport), string(gotRaw))
}

func TestAnalyze_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")
	wantReport := loadFixture(t, "page-without-links-output.txt")

	wantTime, err := time.Parse(time.RFC3339, testTime)
	require.NoError(t, err)

	server := tu.SetupTestServer(t, testHost,
		map[string]tu.TestServerRoute{
			"/": {Status: 200, Body: pageContent},
		},
		nil,
	)

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        testRootURL,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
		Clock:      func() time.Time { return wantTime },
	})
	require.NoError(t, err)

	require.JSONEq(t, string(wantReport), string(gotRaw))
}

func TestAnalyze_NetworkError(t *testing.T) {
	server := tu.SetupTestServer(t, testHost, map[string]tu.TestServerRoute{
		"/": {Fn: tu.HijackAndCloseHandler(t)},
	}, nil)

	want := formatter.Report{
		RootURL: testRootURL,
		Depth:   1,
		Pages: []formatter.PageReport{
			{
				URL:   testRootURL,
				Depth: 0,
				Error: io.EOF.Error(),
			},
		},
	}

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        testRootURL,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	requireReportEquals(t, want, gotRaw)
}

func TestAnalyze_ErrorOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := tu.SetupTestServer(t, testHost, nil, nil)

	_, err := Analyze(ctx, Options{
		URL:        testRootURL,
		HTTPClient: server.Client,
	})
	require.ErrorIs(t, err, context.Canceled)
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
