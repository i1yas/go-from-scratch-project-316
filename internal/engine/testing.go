package engine

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireReportMatches(t *testing.T, want, got Report) {
	t.Helper()

	require.Equal(t, want.RootURL, got.RootURL, "root url")
	require.Equal(t, want.Depth, got.Depth, "report depth")
	require.NotZero(t, got.GeneratedAt, "report generation time is set")

	requirePagesMatch(t, want.Pages, got.Pages)
}

func requirePagesMatch(t *testing.T, want, got []PageReport) {
	require.Equal(t, len(want), len(got), "amount of pages")

	slices.SortFunc(got, comparePageReports)
	slices.SortFunc(want, comparePageReports)

	for i := range want {
		wantPage := want[i]
		gotPage := got[i]

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
	fn     func(w http.ResponseWriter, r *http.Request)
}

type testServer struct {
	srv      *httptest.Server
	client   *http.Client
	calls    map[string]int
	timeline []time.Time
	mu       sync.Mutex
}

func setupTestServer(
	t *testing.T,
	host string,
	internalLinksMapper map[string]testServerResponse,
	foreignLinksMapper map[string]testServerResponse,
) *testServer {
	t.Helper()

	server := &testServer{
		calls:    map[string]int{},
		timeline: []time.Time{},
		mu:       sync.Mutex{},
	}

	server.srv = httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqT := time.Now()

		defer func() {
			server.mu.Lock()
			server.calls[r.URL.String()]++
			server.timeline = append(server.timeline, reqT)
			server.mu.Unlock()
		}()

		if r.Host != host {
			handleTestRequest(t, foreignLinksMapper, w, r)
			return
		}

		handleTestRequest(t, internalLinksMapper, w, r)
	}))

	client := server.srv.Client()
	client.Transport.(*http.Transport).DisableKeepAlives = true // nolint:forcetypeassert
	server.client = client

	t.Cleanup(func() {
		server.srv.Close()
	})

	return server
}

func (ts *testServer) totalCallsCount() int {
	defer ts.mu.Unlock()

	ts.mu.Lock()

	return len(ts.timeline)
}

func (ts *testServer) minDelayBetweenRequests() (time.Duration, bool) {
	defer ts.mu.Unlock()

	ts.mu.Lock()

	minDelay := time.Duration(math.MaxInt64)

	if len(ts.timeline) == 0 {
		return minDelay, false
	}

	lastT := ts.timeline[0]

	for i := 1; i < len(ts.timeline); i++ {
		t := ts.timeline[i]

		delay := t.Sub(lastT)
		if delay < minDelay {
			minDelay = delay
		}

		lastT = t
	}

	return minDelay, true
}

func handleTestRequest(
	t *testing.T,
	mapper map[string]testServerResponse,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()

	respConf, ok := mapper[r.RequestURI]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if respConf.fn != nil {
		respConf.fn(w, r)
		return
	}

	w.WriteHeader(respConf.status)
	_, err := w.Write(respConf.body)
	require.NoError(t, err)
}

func generateDocumentWithLinks(t *testing.T, linkCount int) []byte {
	t.Helper()

	var pageSb strings.Builder
	fmt.Fprint(&pageSb, "<body>")

	for i := range linkCount {
		n := i + 1
		fmt.Fprintf(&pageSb, `<a href="/link-%d">Link %d</a>`, n, n)
	}

	fmt.Fprint(&pageSb, "</body>")

	return []byte(pageSb.String())
}

func hijackAndCloseHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
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
	}
}
