package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEngineAnalyze_WellFormedPage(t *testing.T) {
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
		map[string]testServerResponse{
			"/home": {status: 200, body: pageContent},
		},
	)

	client := server.srv.Client()

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

func TestEngineAnalyze_PagesFetchedOnce(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"

	rootURL, err := url.Parse("http://" + host)
	require.NoError(t, err)

	server := setupTestServer(t, host,
		map[string]testServerResponse{
			"/":        {status: 200, body: pageContent},
			"/working": {status: 200, body: pageContent},
			"/broken":  {status: 500},
		},
		nil,
	)

	client := server.srv.Client()

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      5,
		HTTPClient: client,
	})
	require.NoError(t, err)

	_, err = e.Analyze(t.Context())
	require.NoError(t, err)

	_, err = e.Analyze(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, server.calls["/"])
	require.Equal(t, 1, server.calls["/working"])
	require.Equal(t, 1, server.calls["/broken"])
}

func TestEngineAnalyze_RestrictDepth(t *testing.T) {
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

	client := server.srv.Client()

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

func TestEngineAnalyze_Delay(t *testing.T) {
	linkCount := 10
	pageContent := generateDocumentWithLinks(t, linkCount)

	host := "test.com"

	rootURL, err := url.Parse("http://" + host)
	require.NoError(t, err)

	analyzeDuration := time.Millisecond * 5

	t.Run("no delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		analyzeEnded := make(chan struct{})

		server := setupTestServer(t, host,
			map[string]testServerResponse{
				"/": {status: 200, body: pageContent},
			},
			nil,
		)

		client := server.srv.Client()

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			UserAgent:  "test",
			HTTPClient: client,
		})
		require.NoError(t, err)

		go func() {
			defer close(analyzeEnded)

			_, err = e.Analyze(ctx)
			require.NoError(t, err, context.Canceled)
		}()

		time.Sleep(analyzeDuration)

		cancel()

		require.Equal(t, linkCount+1, server.totalCallsCount())

		<-analyzeEnded
	})

	t.Run("with delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		delay := time.Millisecond
		maxCalls := int(analyzeDuration / delay)

		require.Less(t, maxCalls, linkCount)

		analyzeEnded := make(chan struct{})

		server := setupTestServer(t, host,
			map[string]testServerResponse{
				"/": {status: 200, body: pageContent},
			},
			nil,
		)

		client := server.srv.Client()

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			Delay:      delay,
			UserAgent:  "test",
			HTTPClient: client,
		})
		require.NoError(t, err)

		go func() {
			defer close(analyzeEnded)

			_, err = e.Analyze(ctx)
			require.ErrorIs(t, err, context.Canceled)
		}()

		time.Sleep(analyzeDuration)

		cancelStart := time.Now()

		cancel()

		totalCallsCount := server.totalCallsCount()

		require.Greater(t, totalCallsCount, 0)
		require.LessOrEqual(t, totalCallsCount, maxCalls)

		serverMinDelay, ok := server.minDelayBetweenRequests()
		require.True(t, ok)

		require.GreaterOrEqual(t, serverMinDelay, delay)

		<-analyzeEnded

		delayAfterCancel := time.Since(cancelStart)
		require.Less(t, delayAfterCancel, delay)
	})
}

func TestEngineAnalyze_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")

	rootURL, err := url.Parse("http://test.com")
	require.NoError(t, err)

	server := setupTestServer(t, "test.com",
		map[string]testServerResponse{
			"/": {status: 200, body: pageContent},
		},
		nil,
	)

	client := server.srv.Client()

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

func TestEngineAnalyze_NetworkError(t *testing.T) {
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

func TestEngineAnalyze_CancelContextBeforeStart(t *testing.T) {
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

func TestEngineAnalyze_CancelContextAfterLinksCollected(t *testing.T) {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(t.Context())

	pageContent := loadFixture(t, "well-formed-page.html")

	reqStarted := make(chan struct{})

	blockReq := make(chan struct{})
	defer close(blockReq)

	server := setupTestServer(t, "test.com", map[string]testServerResponse{
		"/": {status: 200, body: pageContent},
		"/working": {fn: func(w http.ResponseWriter, _ *http.Request) {
			close(reqStarted)
			<-blockReq
			w.WriteHeader(http.StatusOK)
		}},
	}, nil)

	rootURL, err := url.Parse("http://test.com")
	require.NoError(t, err)

	e, err := New(Options{
		URL:        rootURL.String(),
		HTTPClient: server.srv.Client(),
		Depth:      1,
	})
	require.NoError(t, err)

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
						StatusCode: 404,
					},
					{
						URL:        "https://example.com/page?param1=x&param2=y",
						StatusCode: 404,
					},
					{
						URL:        "https://example.com/home",
						StatusCode: 404,
					},
				},
				Error: "",
			},
		},
	}

	wg.Go(func() {
		got, err := e.Analyze(ctx)
		require.ErrorIs(t, err, context.Canceled)

		requireReportMatches(t, want, got)
	})

	<-reqStarted

	cancel()

	wg.Wait()
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
