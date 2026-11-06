package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyze_WellFormedPage(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"

	rootURL, err := url.Parse("http://" + host)
	require.NoError(t, err)

	nestedURL, err := url.Parse("http://" + host + "/working")
	require.NoError(t, err)

	server := setupTestServer(t, host,
		map[string]testServerResponse{
			"/":        {status: 200, body: pageContent},
			"/working": {status: 200},
			"/broken":  {status: 500},
		},
		nil,
	)

	client := server.Client()

	want := Report{
		RootURL: rootURL.String(),
		Depth:   1,
		Pages: []PageReport{
			{
				URL:        rootURL,
				Depth:      0,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: PageSEO{
					HasTitle:       true,
					HasDescription: true,
					HasH1:          true,
					Title:          "Test <title>",
					Description:    `test <meta name="description">`,
				},
				BrokenLinks: []BrokenLink{
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
				URL:        nestedURL,
				Depth:      1,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: PageSEO{
					HasTitle:       false,
					HasDescription: false,
					HasH1:          false,
				},
				Error: "",
			},
		},
	}

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestAnalyze_RestrictDepth(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"

	rootURL, err := url.Parse("http://" + host)
	require.NoError(t, err)

	server := setupTestServer(t, host,
		map[string]testServerResponse{
			"/":        {status: 200, body: pageContent},
			"/working": {status: 200},
			"/broken":  {status: 500},
		},
		nil,
	)

	client := server.Client()

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      0,
		UserAgent:  "test",
		HTTPClient: client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, len(got.Pages), "page count")
}

func TestAnalyze_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")

	rootURL, err := url.Parse("http://test.com")
	require.NoError(t, err)

	server := setupTestServer(t, "test.com",
		map[string]testServerResponse{
			"/": {status: 200, body: pageContent},
		},
		nil,
	)

	client := server.Client()

	want := Report{
		RootURL: rootURL.String(),
		Depth:   1,
		Pages: []PageReport{
			{
				URL:        rootURL,
				Depth:      0,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: PageSEO{
					HasTitle: true,
					Title:    "Document",
				},
				Error: "",
			},
		},
	}

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
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

	rootURL, err := url.Parse("http://test.com")
	require.NoError(t, err)

	want := Report{
		RootURL: rootURL.String(),
		Depth:   1,
		Pages: []PageReport{
			{
				URL:   rootURL,
				Depth: 0,
				Error: io.EOF.Error(),
			},
		},
	}

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestAnalyze_ErrorOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := httptest.NewTestServer(
		t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	e, err := New(Options{
		URL:        "http://test.com",
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	_, err = e.Analyze(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestEngine_InvalidOptions(t *testing.T) {
	t.Run("invalid root url", func(t *testing.T) {
		server := httptest.NewTestServer(
			t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		)

		_, err := New(Options{
			URL:        "invalid",
			HTTPClient: server.Client(),
		})
		require.ErrorIs(t, err, ErrInvalidRootURL)
	})

	t.Run("missing http client", func(t *testing.T) {
		_, err := New(Options{
			URL: "http://test.com",
		})
		require.ErrorIs(t, err, ErrHTTPClientNotProvided)
	})
}

func requireReportMatches(t *testing.T, want, got Report) {
	t.Helper()

	require.Equal(t, want.RootURL, got.RootURL, "root url")
	require.Equal(t, want.Depth, got.Depth, "report depth")
	require.NotZero(t, got.GeneratedAt, "report generation time is set")

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

		require.NotZero(t, gotPage.DiscoveredAt, "discovered time is set")
	}
}

func comparePageReports(a, b PageReport) int {
	return strings.Compare(a.URL.String(), b.URL.String())
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
