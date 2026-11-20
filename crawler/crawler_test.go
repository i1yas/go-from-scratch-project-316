package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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
	rootPageContent := loadFixture(t, "normal-page.html")
	nestedPageContent := loadFixture(t, "page-without-links.html")
	wantReport := loadFixture(t, "normal-page-output.txt")

	wantTime, err := time.Parse(time.RFC3339, testTime)
	require.NoError(t, err)

	server := tu.SetupTestServer(t, testHost,
		map[string]tu.TestServerRoute{
			"/":                 {Status: 200, Body: rootPageContent, ContentType: "text/html"},
			"/link":             {Status: 200, Body: nestedPageContent, ContentType: "text/html"},
			"/broken":           {Status: 500},
			"/static/logo.png":  {Status: 200, Body: make([]byte, 12345)},
			"/static/style.css": {Status: 200, Body: make([]byte, 456)},
			"/static/script.js": {Status: 200, Body: make([]byte, 567)},
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
			"/": {Status: 200, Body: pageContent, ContentType: "text/html"},
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

func TestAnalyze_Indent(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")

	server := tu.SetupTestServer(t, testHost,
		map[string]tu.TestServerRoute{
			"/": {Status: 200, Body: pageContent, ContentType: "text/html"},
		},
		nil,
	)

	t.Run("with indent", func(t *testing.T) {
		gotRaw, err := Analyze(t.Context(), Options{
			URL:        testRootURL,
			IndentJSON: true,
			HTTPClient: server.Client,
		})
		require.NoError(t, err)

		var wantRaw bytes.Buffer
		require.NoError(t, json.Indent(&wantRaw, gotRaw, "", "  "))

		require.Equal(t, wantRaw.String(), string(gotRaw))
	})

	t.Run("no indent", func(t *testing.T) {
		gotRaw, err := Analyze(t.Context(), Options{
			URL:        testRootURL,
			IndentJSON: false,
			HTTPClient: server.Client,
		})
		require.NoError(t, err)

		var wantRaw bytes.Buffer
		require.NoError(t, json.Compact(&wantRaw, gotRaw))

		require.Equal(t, wantRaw.String(), string(gotRaw))
	})
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

func TestAnalyze_PartialReportAfterContextCancel(t *testing.T) {
	blockReq := make(chan struct{})
	defer close(blockReq)

	nestedURLVisited := make(chan struct{})

	pageContent := []byte(`
	<html>
		<body>
			<a href="/link">Link</a>
		</body>
	</html>
	`)

	wantTime, err := time.Parse(time.RFC3339, testTime)
	require.NoError(t, err)

	want := loadFixture(t, "partial-report-output.txt")

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Fn: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)

			_, err := w.Write(pageContent)
			require.NoError(t, err)
		}},
		"/link": {Fn: func(_ http.ResponseWriter, _ *http.Request) {
			close(nestedURLVisited)

			<-blockReq
		}},
	}, nil)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
	defer cancel()

	rootURL := "http://test.com"

	go func() {
		<-nestedURLVisited
		cancel()
	}()

	got, err := Analyze(ctx, Options{
		URL:        rootURL,
		Depth:      1,
		HTTPClient: server.Client,
		Clock:      func() time.Time { return wantTime },
	})
	require.NoError(t, err)

	require.JSONEq(t, string(want), string(got))
}

func TestAnalyze_InvalidOptions(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		err  error
	}{
		{
			name: "http client not provided",
			opts: Options{},
			err:  ErrHTTPClientNotProvided,
		},
		{
			name: "invalid root url",
			opts: Options{
				URL:        "invalid",
				HTTPClient: &http.Client{},
			},
			err: ErrInvalidRootURL,
		},
		{
			name: "invalid depth",
			opts: Options{
				URL:        testRootURL,
				HTTPClient: &http.Client{},
				Depth:      -1,
			},
			err: ErrInvalidDepth,
		},
		{
			name: "invalid retires",
			opts: Options{
				URL:        testRootURL,
				HTTPClient: &http.Client{},
				Retries:    -1,
			},
			err: ErrInvalidRetries,
		},
		{
			name: "invalid delay",
			opts: Options{
				URL:        testRootURL,
				HTTPClient: &http.Client{},
				Delay:      -time.Second,
			},
			err: ErrInvalidDelay,
		},
		{
			name: "invalid timeout",
			opts: Options{
				URL:        testRootURL,
				HTTPClient: &http.Client{},
				Timeout:    -time.Second,
			},
			err: ErrInvalidTimeout,
		},
		{
			name: "invalid concurrency",
			opts: Options{
				URL:         testRootURL,
				HTTPClient:  &http.Client{},
				Concurrency: -1,
			},
			err: ErrInvalidConcurrency,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Analyze(t.Context(), tc.opts)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", "fixture", name)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}
