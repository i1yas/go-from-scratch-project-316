package crawler

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	gotRaw, err := Analyze(t.Context(), Options{
		URL:        testRootURL,
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	var got formatter.Report

	err = json.Unmarshal(gotRaw, &got)
	require.NoError(t, err)

	require.Equal(t, testRootURL, got.RootURL)
	require.Equal(t, 1, len(got.Pages))

	require.Equal(t, testRootURL, got.Pages[0].URL)
	require.Contains(t, got.Pages[0].Error, io.EOF.Error())
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

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", "fixture", name)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}
