package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	srv   *httptest.Server
	calls map[string]int
}

func setupTestServer(
	t *testing.T,
	host string,
	internalLinksMapper map[string]testServerResponse,
	foreignLinksMapper map[string]testServerResponse,
) testServer {
	t.Helper()

	calls := map[string]int{}

	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.String()]++

		if r.Host != host {
			handleTestRequest(t, foreignLinksMapper, w, r)
			return
		}

		handleTestRequest(t, internalLinksMapper, w, r)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return testServer{
		srv:   server,
		calls: calls,
	}
}

func (ts *testServer) totalCallsCount() int {
	total := 0
	for _, count := range ts.calls {
		total += count
	}

	return total
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
