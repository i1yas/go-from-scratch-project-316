package crawler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"code/internal/formatter"
)

func TestAnalyze_WellFormedPage(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	url := "http://test.com"

	server := setupTestServer(t, "test.com",
		map[string]testServerResponse{
			"/":        {status: 200, body: pageContent},
			"/working": {status: 200},
			"/broken":  {status: 500},
		},
		nil,
	)

	client := server.Client()

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

func TestAnalyze_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")
	url := "http://test.com"

	server := setupTestServer(t, "test.com",
		map[string]testServerResponse{
			"/": {status: 200, body: pageContent},
		},
		nil,
	)

	client := server.Client()

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
		HTTPClient: client,
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

	require.Equal(t, len(want.Pages), len(got.Pages))

	for i := range want.Pages {
		wantPage := want.Pages[i]
		gotPage := got.Pages[i]

		assert.Equal(t, wantPage.URL, gotPage.URL)
		assert.Equal(t, wantPage.Depth, gotPage.Depth)
		assert.Contains(t, gotPage.Error, wantPage.Error)
		assert.Equal(t, wantPage.HTTPStatus, gotPage.HTTPStatus)
		assert.Equal(t, wantPage.Status, gotPage.Status)
		assert.Equal(t, wantPage.SEO, gotPage.SEO)

		require.ElementsMatch(t, wantPage.BrokenLinks, gotPage.BrokenLinks)

		requireTimeIsValid(t, gotPage.DiscoveredAt)
	}
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

type testServerResponse struct {
	status int
	body   []byte
}

func setupTestServer(
	t *testing.T,
	host string,
	internalLinksMapper map[string]testServerResponse,
	foreignLinksMapper map[string]testServerResponse,
) *httptest.Server {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			respConf, ok := foreignLinksMapper[r.RequestURI]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.WriteHeader(respConf.status)
			_, err := w.Write(respConf.body)
			require.NoError(t, err)

			return
		}

		respConf, ok := internalLinksMapper[r.RequestURI]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(respConf.status)
		_, err := w.Write(respConf.body)
		require.NoError(t, err)
	}))

	return server
}
