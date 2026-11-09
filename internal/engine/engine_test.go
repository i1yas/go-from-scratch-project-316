package engine

import (
	"context"
	"io"
	"net/http"
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
		HTTPClient: server.client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEningeAnalyze_RootURLFailed(t *testing.T) {
	server := setupTestServer(t, "test.com",
		map[string]testServerResponse{
			"/": {status: 500},
		},
		nil,
	)

	rootURL, err := url.Parse("http://test.com")
	require.NoError(t, err)

	e, err := New(Options{
		URL:        rootURL.String(),
		HTTPClient: server.client,
	})
	require.NoError(t, err)

	want := Report{
		RootURL: rootURL.String(),
		Depth:   0,
		Pages: []PageReport{
			{
				URL:        rootURL,
				Depth:      0,
				HTTPStatus: 500,
				Status:     "internal server error",
			},
		},
	}

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

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      5,
		HTTPClient: server.client,
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

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      0,
		UserAgent:  "test",
		HTTPClient: server.client,
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

	var serverCallsWithoutDelay int
	var serverCallsWithDelay int

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

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			UserAgent:  "test",
			HTTPClient: server.client,
		})
		require.NoError(t, err)

		go func() {
			defer close(analyzeEnded)

			_, err = e.Analyze(ctx)
			require.NoError(t, err, context.Canceled)
		}()

		time.Sleep(analyzeDuration)

		cancel()

		serverCallsWithoutDelay = server.totalCallsCount()

		<-analyzeEnded
	})

	t.Run("with delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		delay := time.Millisecond
		maxCalls := int(analyzeDuration/delay) + 1 // NOTE: first call does not have delay

		require.Less(t, maxCalls, linkCount)

		analyzeEnded := make(chan struct{})

		server := setupTestServer(t, host,
			map[string]testServerResponse{
				"/": {status: 200, body: pageContent},
			},
			nil,
		)

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			Delay:      delay,
			UserAgent:  "test",
			HTTPClient: server.client,
		})
		require.NoError(t, err)

		go func() {
			defer close(analyzeEnded)

			_, err = e.Analyze(ctx)
			require.ErrorIs(t, err, context.Canceled)
		}()

		time.Sleep(analyzeDuration)

		cancel()

		<-analyzeEnded

		totalCallsCount := server.totalCallsCount()
		serverCallsWithDelay = totalCallsCount

		require.Greater(t, totalCallsCount, 0)
		require.LessOrEqual(t, totalCallsCount, maxCalls)

		serverMinDelay, ok := server.minDelayBetweenRequests()
		require.True(t, ok)

		require.GreaterOrEqual(t, serverMinDelay, delay)
	})

	require.Greater(t, serverCallsWithoutDelay, serverCallsWithDelay)

	t.Run("stop delay after cancel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		defer cancel()

		delay := time.Hour

		analyzeEnded := make(chan struct{})

		server := setupTestServer(t, host,
			map[string]testServerResponse{
				"/": {status: 200, body: pageContent},
			},
			nil,
		)

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			Delay:      delay,
			UserAgent:  "test",
			HTTPClient: server.client,
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

		<-analyzeEnded

		delayAfterCancel := time.Since(cancelStart)
		require.Less(t, delayAfterCancel, time.Second*5)

		require.Greater(t, server.totalCallsCount(), 0)
	})
}

func TestEngineAnalyze_RetriesOnRootURL(t *testing.T) {
	cases := []struct {
		name         string
		serverFails  int
		retries      int
		wantCalls    int
		wantStatus   int
		wantError    string
		onServerFail func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:        "enough retries, success",
			serverFails: 2,
			retries:     3,
			wantCalls:   3,
			wantStatus:  200,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
		},
		{
			name:        "just enough retries, success",
			serverFails: 3,
			retries:     3,
			wantCalls:   4,
			wantStatus:  200,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
		},
		{
			name:        "not enough retires, record status",
			serverFails: 4,
			retries:     3,
			wantCalls:   4,
			wantStatus:  500,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
		},
		{
			name:         "not enough retires, record error",
			serverFails:  4,
			retries:      3,
			wantCalls:    4,
			wantError:    io.EOF.Error(),
			onServerFail: hijackAndCloseHandler(t),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverFailsLeft := tc.serverFails

			server := setupTestServer(t, "test.com", map[string]testServerResponse{
				"/": {fn: func(w http.ResponseWriter, r *http.Request) {
					if serverFailsLeft > 0 {
						serverFailsLeft--

						tc.onServerFail(w, r)

						return
					}

					w.WriteHeader(200)
				}},
			}, nil)

			e, err := New(Options{
				URL:        "http://test.com",
				HTTPClient: server.client,
				Retries:    tc.retries,
			})
			require.NoError(t, err)

			got, err := e.Analyze(t.Context())
			require.NoError(t, err)

			require.Equal(t, 1, len(got.Pages), "pages amount")

			gotPage := got.Pages[0]
			require.Equal(t, gotPage.HTTPStatus, tc.wantStatus, "status")
			require.Contains(t, gotPage.Error, tc.wantError, "error")
			require.Equal(t, tc.wantCalls, server.totalCallsCount())
		})
	}
}

func TestEngineAnalyze_RetriesOnNestedURL(t *testing.T) {
	cases := []struct {
		name            string
		serverFails     int
		retries         int
		wantCalls       int
		wantPages       int
		wantBrokenLinks int
		wantStatus      int
		wantError       error
		onServerFail    func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:            "enough retries, no broken links",
			serverFails:     2,
			retries:         3,
			wantCalls:       3,
			wantPages:       2,
			wantBrokenLinks: 0,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
		},
		{
			name:            "just enough retries, no broken links",
			serverFails:     3,
			retries:         3,
			wantCalls:       4,
			wantPages:       2,
			wantBrokenLinks: 0,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
		},
		{
			name:            "not enough retires, record status 500",
			serverFails:     4,
			retries:         3,
			wantCalls:       4,
			wantBrokenLinks: 1,
			wantPages:       1,
			wantStatus:      500,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
		},
		{
			name:            "not enough retires, record status 429",
			serverFails:     4,
			retries:         3,
			wantCalls:       4,
			wantBrokenLinks: 1,
			wantPages:       1,
			wantStatus:      429,
			onServerFail: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(429)
			},
		},
		{
			name:            "not enough retires, record error",
			serverFails:     4,
			retries:         3,
			wantCalls:       4,
			wantPages:       1,
			wantBrokenLinks: 1,
			wantError:       io.EOF,
			onServerFail:    hijackAndCloseHandler(t),
		},
	}

	pageContent := loadFixture(t, "page-with-one-link.html")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverFailsLeft := tc.serverFails

			server := setupTestServer(t, "test.com", map[string]testServerResponse{
				"/": {status: 200, body: pageContent},
				"/link": {fn: func(w http.ResponseWriter, r *http.Request) {
					if serverFailsLeft > 0 {
						serverFailsLeft--

						tc.onServerFail(w, r)

						return
					}

					w.WriteHeader(200)
				}},
			}, nil)

			e, err := New(Options{
				URL:        "http://test.com",
				HTTPClient: server.client,
				Depth:      1,
				Retries:    tc.retries,
			})
			require.NoError(t, err)

			got, err := e.Analyze(t.Context())
			require.NoError(t, err)

			require.Equal(t, tc.wantPages, len(got.Pages), "pages amount")

			gotPage, ok := got.findPageByRawURL("http://test.com")
			require.True(t, ok, "page found")

			require.Equal(t, tc.wantBrokenLinks, len(gotPage.BrokenLinks), "amount of broken links")

			if tc.wantBrokenLinks > 0 {
				gotBrokenLink := gotPage.BrokenLinks[0]
				require.Equal(t, tc.wantStatus, gotBrokenLink.StatusCode, "status")
				require.ErrorIs(t, gotBrokenLink.Err, tc.wantError)
			}
		})
	}
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
		HTTPClient: server.client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEngineAnalyze_NetworkError(t *testing.T) {
	server := setupTestServer(t, "test.com", map[string]testServerResponse{
		"/": {fn: hijackAndCloseHandler(t)},
	}, nil)

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
		HTTPClient: server.client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEngineAnalyze_CancelContextBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := setupTestServer(t, "test.com", nil, nil)

	e, err := New(Options{
		URL:        "http://test.com",
		HTTPClient: server.client,
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
		HTTPClient: server.client,
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
		_, err := New(Options{
			URL:        "invalid",
			HTTPClient: &http.Client{},
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
