package engine

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	tu "code/internal/testutils"
)

func TestEngineAnalyze_WellFormedPage(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"

	rootURL := testURL(t, "http://"+host)
	nestedURL := testURL(t, "http://"+host+"/working")

	server := tu.SetupTestServer(t, host,
		map[string]tu.TestServerRoute{
			"/":        {Status: 200, Body: pageContent},
			"/working": {Status: 200},
			"/broken":  {Status: 500},
		},
		map[string]tu.TestServerRoute{
			"/home": {Status: 200, Body: pageContent},
		},
	)

	want := Report{
		RootURL: rootURL,
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
				BrokenLinks: map[string]*BrokenLink{
					"http://test.com/broken": {
						URL:        testURL(t, "http://test.com/broken"),
						StatusCode: 500,
					},
					"https://example.com/page?param1=x&param2=y": {
						URL:        testURL(t, "https://example.com/page?param1=x&param2=y"),
						StatusCode: 404,
					},
				},
				Error: nil,
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
				Error: nil,
			},
		},
	}

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEngineAnalyze_RootURLNormalized(t *testing.T) {
	inputRootURL := "http://TEST.com/page//#section"
	wantRootURL := "http://test.com/page"

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Status: 200},
	}, nil)

	e, err := New(Options{
		URL:        inputRootURL,
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	require.Equal(t, wantRootURL, got.RootURL.String())
	require.Equal(t, 1, len(got.Pages))
	require.Equal(t, wantRootURL, got.Pages[0].URL.String())
}

func TestEngineAnalyze_CustomUserAgent(t *testing.T) {
	wantUserAgent := "custom-user-agent"

	gotUserAgent := make(chan string)
	analyzeEnded := make(chan struct{})

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Fn: func(w http.ResponseWriter, r *http.Request) {
			gotUserAgent <- r.UserAgent()

			w.WriteHeader(http.StatusOK)
		}},
	}, nil)

	e, err := New(Options{
		URL:        "http://test.com",
		HTTPClient: server.Client,
		UserAgent:  wantUserAgent,
	})
	require.NoError(t, err)

	go func() {
		_, err := e.Analyze(t.Context())
		require.NoError(t, err)

		close(analyzeEnded)
	}()

	select {
	case v := <-gotUserAgent:
		require.Equal(t, wantUserAgent, v)
	case <-time.After(time.Second * 5):
		t.Fatal("waited request too long")
	}

	select {
	case <-analyzeEnded:
		break
	case <-time.After(time.Second * 5):
		t.Fatal("waited analyze too long")
	}
}

func TestEngineAnalyze_RssFeed(t *testing.T) {
	rssPageContent := loadFixture(t, "rss-feed.xml")

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Status: 200, Body: generatePageWithLinks([]string{"/rss"})},
		"/rss": {Fn: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/rss+xml")
			w.WriteHeader(http.StatusOK)

			_, err := w.Write(rssPageContent)
			require.NoError(t, err)
		}},
	}, nil)

	e, err := New(Options{
		URL:        "http://test.com",
		Depth:      1,
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	want := Report{
		RootURL: testURL(t, "http://test.com"),
		Depth:   1,
		Pages: []PageReport{
			{
				URL:        testURL(t, "http://test.com"),
				Depth:      0,
				Status:     "ok",
				HTTPStatus: 200,
			},
			{
				URL:        testURL(t, "http://test.com/rss"),
				Depth:      1,
				Status:     "ok",
				HTTPStatus: 200,
				SEO: PageSEO{
					HasTitle: true,
					Title:    "Test RSS-feed",
				},
			},
		},
	}

	requireReportMatches(t, want, got)
}

func TestEngineAnalyze_BadSEO(t *testing.T) {
	pageContent := loadFixture(t, "bad-seo.html")

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Status: 200, Body: pageContent},
	}, nil)

	rootURL := testURL(t, "http://test.com")

	e, err := New(Options{
		URL:        rootURL.String(),
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	want := Report{
		RootURL: rootURL,
		Depth:   0,
		Pages: []PageReport{
			{
				URL:        rootURL,
				Depth:      0,
				HTTPStatus: 200,
				Status:     "ok",
				SEO: PageSEO{
					HasTitle:       false,
					HasDescription: false,
					HasH1:          false,
					Title:          "",
					Description:    "",
				},
				Error: nil,
			},
		},
	}

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEngine_CustomClock(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Status: 200, Body: pageContent},
	}, nil)

	wantTime := time.Now().Add(time.Minute * 111)

	e, err := New(Options{
		URL:        "http://test.com",
		HTTPClient: server.Client,
		Clock: func() time.Time {
			return wantTime
		},
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	require.Equal(t, wantTime, got.GeneratedAt)
	require.Equal(t, 1, len(got.Pages))
	require.Equal(t, wantTime, got.Pages[0].DiscoveredAt)
}

func TestEningeAnalyze_RootURLFailed(t *testing.T) {
	server := tu.SetupTestServer(t, "test.com",
		map[string]tu.TestServerRoute{
			"/": {Status: 500},
		},
		nil,
	)

	rootURL := testURL(t, "http://test.com")

	e, err := New(Options{
		URL:        rootURL.String(),
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	want := Report{
		RootURL: rootURL,
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

	rootURL := testURL(t, "http://"+host)

	server := tu.SetupTestServer(t, host,
		map[string]tu.TestServerRoute{
			"/":        {Status: 200, Body: pageContent},
			"/working": {Status: 200, Body: pageContent},
			"/broken":  {Status: 500},
		},
		nil,
	)

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      5,
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	rootPageReport := PageReport{
		URL:        testURL(t, "http://test.com"),
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
		BrokenLinks: map[string]*BrokenLink{
			"http://test.com/broken": {
				URL:        testURL(t, "http://test.com/broken"),
				StatusCode: 500,
			},
			"https://example.com/home": {
				URL:        testURL(t, "https://example.com/home"),
				StatusCode: 404,
			},
			"https://example.com/page?param1=x&param2=y": {
				URL:        testURL(t, "https://example.com/page?param1=x&param2=y"),
				StatusCode: 404,
			},
		},
		Error: nil,
	}

	nestedPageReport := rootPageReport
	nestedPageReport.URL = testURL(t, "http://test.com/working")
	nestedPageReport.Depth = 1

	want := Report{
		RootURL: testURL(t, "http://test.com"),
		Depth:   5,
		Pages: []PageReport{
			rootPageReport,
			nestedPageReport,
		},
	}

	_, err = e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)

	require.Equal(t, 1, server.CallsCountTo("/"))
	require.Equal(t, 1, server.CallsCountTo("/working"))
	require.Equal(t, 1, server.CallsCountTo("/broken"))
}

func TestEngineAnalyze_RestrictDepth(t *testing.T) {
	pageContent := loadFixture(t, "well-formed-page.html")
	host := "test.com"

	rootURL := testURL(t, "http://"+host)

	server := tu.SetupTestServer(t, host,
		map[string]tu.TestServerRoute{
			"/":        {Status: 200, Body: pageContent},
			"/working": {Status: 200},
			"/broken":  {Status: 500},
		},
		nil,
	)

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      0,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, len(got.Pages), "page count")
}

func TestEngineAnalyze_RecordPageMinDepth(t *testing.T) {
	// we testing two paths to get to /target page:
	// (1) root -> /nested       -> /nested/deep -> /target
	// (2) root -> /nested-block -> /target
	//
	// In path (1) routes do not block requests, /target has depth=3;
	// In path (2) /nested-block route blocks request until target is requested once, /target has depth=2.
	//
	// This way /target page first discovered with depth=3 and then with depth=2
	wantDepth := 2

	rootPageContent := generatePageWithLinks([]string{"/nested", "/nested-block"})
	nestedPageContent := generatePageWithLinks([]string{"/nested/deep"})
	pageWithTargetLink := generatePageWithLinks([]string{"/target"})

	targetVisited := make(chan struct{}, 1)

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/":            {Status: 200, Body: rootPageContent},
		"/nested":      {Status: 200, Body: nestedPageContent},
		"/nested/deep": {Status: 200, Body: pageWithTargetLink},
		"/nested-block": {Fn: func(w http.ResponseWriter, _ *http.Request) {
			<-targetVisited

			time.Sleep(time.Millisecond * 100)

			w.WriteHeader(http.StatusOK)

			_, err := w.Write(pageWithTargetLink)
			require.NoError(t, err)
		}},
		"/target": {Fn: func(w http.ResponseWriter, _ *http.Request) {
			targetVisited <- struct{}{}

			w.WriteHeader(http.StatusOK)
		}},
	}, nil)

	e, err := New(Options{
		URL:        "http://test.com",
		Depth:      5,
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	gotPage, ok := got.findPageByRawURL("http://test.com/target")
	require.True(t, ok, "did not found /target page in report")

	require.Equal(t, wantDepth, gotPage.Depth)
}

func TestEngineAnalyze_Delay(t *testing.T) {
	linkCount := 10
	pageContent := generatePageWithLinksAmount(linkCount)

	host := "test.com"

	rootURL := testURL(t, "http://"+host)

	analyzeDuration := time.Millisecond * 100

	var (
		serverCallsWithoutDelay int
		serverCallsWithDelay    int
	)

	t.Run("no delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		analyzeEnded := make(chan struct{})

		server := tu.SetupTestServer(t, host,
			map[string]tu.TestServerRoute{
				"/": {Status: 200, Body: pageContent},
			},
			nil,
		)

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			UserAgent:  "test",
			HTTPClient: server.Client,
		})
		require.NoError(t, err)

		go func() {
			defer close(analyzeEnded)

			_, err = e.Analyze(ctx)
			require.NoError(t, err, context.Canceled)
		}()

		time.Sleep(analyzeDuration)

		cancel()

		select {
		case <-analyzeEnded:
			break
		case <-time.After(time.Second * 5):
			t.Fatal("waited analyze too long")
		}

		serverCallsWithoutDelay = server.TotalCallsCount()
	})

	t.Run("with delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		delay := time.Millisecond * 30
		slack := time.Millisecond * 5
		maxCalls := int(analyzeDuration/delay) + 1 // NOTE: first call does not have delay

		require.Less(t, maxCalls, linkCount)

		analyzeEnded := make(chan struct{})

		server := tu.SetupTestServer(t, host,
			map[string]tu.TestServerRoute{
				"/": {Status: 200, Body: pageContent},
			},
			nil,
		)

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			Delay:      delay,
			UserAgent:  "test",
			HTTPClient: server.Client,
		})
		require.NoError(t, err)

		go func() {
			defer close(analyzeEnded)

			_, err = e.Analyze(ctx)
			require.ErrorIs(t, err, context.Canceled)
		}()

		time.Sleep(analyzeDuration)

		cancel()

		select {
		case <-analyzeEnded:
			break
		case <-time.After(time.Second * 5):
			t.Fatal("waited analyze too long")
		}

		totalCallsCount := server.TotalCallsCount()
		serverCallsWithDelay = totalCallsCount

		require.Greater(t, totalCallsCount, 0)
		require.LessOrEqual(t, totalCallsCount, maxCalls)

		require.GreaterOrEqual(t, server.MinDelayBetweenRequests(), delay-slack)
	})

	t.Run("more calls without delay", func(t *testing.T) {
		require.Greater(t, serverCallsWithoutDelay, serverCallsWithDelay)
	})

	t.Run("stop delay after cancel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second*20)
		defer cancel()

		delay := time.Hour

		analyzeEnded := make(chan struct{})

		server := tu.SetupTestServer(t, host,
			map[string]tu.TestServerRoute{
				"/": {Status: 200, Body: pageContent},
			},
			nil,
		)

		e, err := New(Options{
			URL:        rootURL.String(),
			Depth:      100,
			Delay:      delay,
			UserAgent:  "test",
			HTTPClient: server.Client,
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

		select {
		case <-analyzeEnded:
			break
		case <-time.After(time.Second * 5):
			t.Fatal("waited analyze too long")
		}

		delayAfterCancel := time.Since(cancelStart)
		require.Less(t, delayAfterCancel, time.Second*5)

		require.Greater(t, server.TotalCallsCount(), 0)
	})
}

func TestEngineAnalyze_RetriesOnRootURL(t *testing.T) {
	cases := []struct {
		name         string
		serverFails  int
		retries      int
		wantCalls    int
		wantStatus   int
		wantError    error
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
			wantError:    io.EOF,
			onServerFail: tu.HijackAndCloseHandler(t),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverFailsLeft := tc.serverFails

			server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
				"/": {Fn: func(w http.ResponseWriter, r *http.Request) {
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
				HTTPClient: server.Client,
				Retries:    tc.retries,
			})
			require.NoError(t, err)

			got, err := e.Analyze(t.Context())
			require.NoError(t, err)

			require.Equal(t, 1, len(got.Pages), "pages amount")

			gotPage := got.Pages[0]
			require.Equal(t, gotPage.HTTPStatus, tc.wantStatus, "status")
			require.ErrorIs(t, gotPage.Error, tc.wantError)
			require.Equal(t, tc.wantCalls, server.TotalCallsCount())
		})
	}
}

func TestEngineAnalyze_RetriesOnNestedURL(t *testing.T) {
	cases := []struct {
		name            string
		serverFails     int
		retries         int
		wantNestedCalls int
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
			wantNestedCalls: 3,
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
			wantNestedCalls: 4,
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
			wantNestedCalls: 4,
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
			wantNestedCalls: 4,
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
			wantNestedCalls: 4,
			wantPages:       1,
			wantBrokenLinks: 1,
			wantError:       io.EOF,
			onServerFail:    tu.HijackAndCloseHandler(t),
		},
	}

	pageContent := loadFixture(t, "page-with-one-link.html")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverFailsLeft := tc.serverFails

			server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
				"/": {Status: 200, Body: pageContent},
				"/link": {Fn: func(w http.ResponseWriter, r *http.Request) {
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
				HTTPClient: server.Client,
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
				gotBrokenLink, ok := gotPage.BrokenLinks["http://test.com/link"]
				require.True(t, ok)

				require.Equal(t, tc.wantStatus, gotBrokenLink.StatusCode, "status")
				require.ErrorIs(t, gotBrokenLink.Error, tc.wantError)
			}

			require.Equal(t, tc.wantNestedCalls, server.CallsCountTo("/link"))
		})
	}
}

func TestEngineAnalyze_Assets(t *testing.T) {
	pageContent := loadFixture(t, "page-with-assets.html")

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/":            {Status: 200, Body: pageContent},
		"/style.css":   {Status: 200},
		"/script.js":   {Fn: tu.HijackAndCloseHandler(t)},
		"/image-1.png": {Status: 200, Body: make([]byte, 121), OmitContentLength: true},
		"/image-2.png": {Status: 200, ContentLength: 121},
	}, nil)

	rootURL := testURL(t, "http://test.com")

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	want := Report{
		RootURL: rootURL,
		Depth:   1,
		Pages: []PageReport{
			{
				URL:        rootURL,
				Depth:      0,
				HTTPStatus: 200,
				Status:     "ok",
				Assets: map[string]*Asset{
					"http://test.com/style.css": {
						URL:        testURL(t, "http://test.com/style.css"),
						Type:       AssetTypeStyle,
						StatusCode: 200,
						SizeBytes:  0,
					},
					"http://test.com/script.js": {
						URL:   testURL(t, "http://test.com/script.js"),
						Type:  AssetTypeScript,
						Error: io.EOF,
					},
					"http://test.com/image-1.png": {
						URL:        testURL(t, "http://test.com/image-1.png"),
						Type:       AssetTypeImage,
						StatusCode: 200,
						SizeBytes:  121,
					},
					"http://test.com/image-2.png": {
						URL:        testURL(t, "http://test.com/image-2.png"),
						Type:       AssetTypeImage,
						StatusCode: 200,
						SizeBytes:  121,
					},
					"http://test.com/image-3.png": {
						URL:        testURL(t, "http://test.com/image-3.png"),
						Type:       AssetTypeImage,
						StatusCode: 404,
						SizeBytes:  0,
					},
				},
			},
		},
	}

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)

	_, err = e.Analyze(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, server.CallsCountTo("/style.css"))
	require.Equal(t, 1, server.CallsCountTo("/script.js"))
	require.Equal(t, 1, server.CallsCountTo("/image-1.png"))
	require.Equal(t, 1, server.CallsCountTo("/image-2.png"))
	require.Equal(t, 1, server.CallsCountTo("/image-3.png"))
}

func TestEngineAnalyze_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")

	rootURL := testURL(t, "http://test.com")

	server := tu.SetupTestServer(t, "test.com",
		map[string]tu.TestServerRoute{
			"/": {Status: 200, Body: pageContent},
		},
		nil,
	)

	want := Report{
		RootURL: rootURL,
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
				Error: nil,
			},
		},
	}

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEngineAnalyze_NetworkError(t *testing.T) {
	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Fn: tu.HijackAndCloseHandler(t)},
	}, nil)

	rootURL := testURL(t, "http://test.com")

	want := Report{
		RootURL: rootURL,
		Depth:   1,
		Pages: []PageReport{
			{
				URL:   rootURL,
				Depth: 0,
				Error: io.EOF,
			},
		},
	}

	e, err := New(Options{
		URL:        rootURL.String(),
		Depth:      1,
		UserAgent:  "test",
		HTTPClient: server.Client,
	})
	require.NoError(t, err)

	got, err := e.Analyze(t.Context())
	require.NoError(t, err)

	requireReportMatches(t, want, got)
}

func TestEngineAnalyze_CancelContextBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := tu.SetupTestServer(t, "test.com", nil, nil)

	e, err := New(Options{
		URL:        "http://test.com",
		HTTPClient: server.Client,
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

	server := tu.SetupTestServer(t, "test.com", map[string]tu.TestServerRoute{
		"/": {Status: 200, Body: pageContent},
		"/working": {Fn: func(w http.ResponseWriter, _ *http.Request) {
			close(reqStarted)
			<-blockReq
			w.WriteHeader(http.StatusOK)
		}},
	}, nil)

	rootURL := testURL(t, "http://test.com")

	e, err := New(Options{
		URL:        rootURL.String(),
		HTTPClient: server.Client,
		Depth:      1,
	})
	require.NoError(t, err)

	want := Report{
		RootURL: rootURL,
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
				BrokenLinks: map[string]*BrokenLink{
					"http://test.com/broken": {
						URL:        testURL(t, "http://test.com/broken"),
						StatusCode: 404,
					},
					"https://example.com/page?param1=x&param2=y": {
						URL:        testURL(t, "https://example.com/page?param1=x&param2=y"),
						StatusCode: 404,
					},
					"https://example.com/home": {
						URL:        testURL(t, "https://example.com/home"),
						StatusCode: 404,
					},
				},
				Error: nil,
			},
		},
	}

	wg.Go(func() {
		got, err := e.Analyze(ctx)
		require.ErrorIs(t, err, context.Canceled)

		requireReportMatches(t, want, got)
	})

	select {
	case <-reqStarted:
		break
	case <-time.After(time.Second * 5):
		t.Fatal("waited for request start too long")
	}

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
