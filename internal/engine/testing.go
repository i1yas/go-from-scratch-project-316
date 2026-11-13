package engine

import (
	"fmt"
	"net/url"
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
	t.Helper()

	require.Equal(t, len(want), len(got), "amount of pages")

	slices.SortFunc(got, comparePageReports)
	slices.SortFunc(want, comparePageReports)

	for i := range want {
		wantPage := want[i]
		gotPage := got[i]

		assert.Equal(t, wantPage.URL, gotPage.URL, "url")
		assert.Equal(t, wantPage.Depth, gotPage.Depth, "depth")
		assert.ErrorIs(t, gotPage.Error, wantPage.Error)
		assert.Equal(t, wantPage.HTTPStatus, gotPage.HTTPStatus, "http status")
		assert.Equal(t, wantPage.Status, gotPage.Status, "status")
		assert.Equal(t, wantPage.SEO, gotPage.SEO, "seo")

		requireAssetsMatch(t, wantPage.Assets, gotPage.Assets)
		requireBrokenLinksMatch(t, wantPage.BrokenLinks, gotPage.BrokenLinks)

		require.NotZero(t, gotPage.DiscoveredAt, "discovered time is set")
	}
}

func requireAssetsMatch(t *testing.T, want, got map[string]*Asset) {
	t.Helper()

	require.Equal(t, len(want), len(got), "amount of assets")

	for key := range want {
		wantAsset := want[key]
		gotAsset := got[key]

		assert.Equal(t, wantAsset.Type, gotAsset.Type, "asset type")
		assert.Equal(t, wantAsset.StatusCode, gotAsset.StatusCode, "status")
		assert.Equal(t, wantAsset.SizeBytes, gotAsset.SizeBytes, "size bytes")
		assert.ErrorIs(t, gotAsset.Error, wantAsset.Error, "error")
		require.Equal(t, wantAsset.URL.String(), gotAsset.URL.String(), "url")
	}
}

func requireBrokenLinksMatch(t *testing.T, want, got map[string]*BrokenLink) {
	t.Helper()

	require.Equal(t, len(want), len(got), "amount of broken links")

	for key := range want {
		wantLink := want[key]
		gotLink := got[key]

		assert.Equal(t, wantLink.StatusCode, gotLink.StatusCode)
		assert.ErrorIs(t, gotLink.Error, wantLink.Error)
		require.Equal(t, wantLink.URL, gotLink.URL)
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

func generatePageWithLinks(links []string) []byte {
	var sb strings.Builder

	sb.WriteString("<body>")

	for _, link := range links {
		fmt.Fprintf(&sb, `<a href="%s">link</a>`, link)
	}

	sb.WriteString("</body>")

	return []byte(sb.String())
}

func generatePageWithLinksAmount(linkCount int) []byte {
	links := make([]string, linkCount)

	for i := range linkCount {
		n := i + 1
		links[i] = fmt.Sprintf(`/link-%d`, n)
	}

	return generatePageWithLinks(links)
}

func testURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	return u
}
