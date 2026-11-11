package htmlparser

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractLinks_PageWithLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-with-links.html")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks := ExtractLinks(doc, docURL(t))

	wantLinks := []Link{
		{
			Type: LinkTypeStyle,
			URL:  testURL(t, "http://test.com/style.css"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/link"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/inner-page/relative-with-dot"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/inner-page/relative-no-dot"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/relative-5-dir-up"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/item-1"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/item-2"),
		},
		{
			Type: LinkTypeImage,
			URL:  testURL(t, "http://test.com/image"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://test.com/banner"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "https://example.com/page?param=value"),
		},
		{
			Type: LinkTypeAnchor,
			URL:  testURL(t, "http://example.com/page?param=value"),
		},
		{
			Type: LinkTypeScript,
			URL:  testURL(t, "http://test.com/script.js"),
		},
	}

	requireLinksMatch(t, wantLinks, gotLinks)
}

func TestExtractLinks_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks := ExtractLinks(doc, docURL(t))

	require.Equal(t, 0, len(gotLinks))
}

func TestExtractLinks_InvalidHTML(t *testing.T) {
	pageContent := loadFixture(t, "not-html.txt")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks := ExtractLinks(doc, docURL(t))

	require.Equal(t, 0, len(gotLinks))
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", "fixture", name)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}

func docURL(t *testing.T) *url.URL {
	t.Helper()

	u, err := url.Parse("http://test.com/inner-page")
	require.NoError(t, err)

	return u
}

func testURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	return u
}

func requireLinksMatch(t *testing.T, wantLinks, gotLinks []Link) {
	slices.SortFunc(wantLinks, compareLinks)
	slices.SortFunc(gotLinks, compareLinks)

	for i := range wantLinks {
		wantLink := wantLinks[i]
		gotLink := gotLinks[i]

		assert.Equal(t, wantLink.Type, gotLink.Type, wantLink.URL.String())
		require.Equal(t, wantLink.URL.String(), gotLink.URL.String())
	}
}

func compareLinks(l1, l2 Link) int {
	a := l1.URL.String() + string(l1.Type)
	b := l2.URL.String() + string(l2.Type)

	return strings.Compare(a, b)
}
