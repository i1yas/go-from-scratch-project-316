package htmlparser

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func TestExtractLinks_PageWithLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-with-links.html")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks, err := ExtractLinks(doc, docURL(t))
	require.NoError(t, err)

	wantLinks := []string{
		"http://test.com/link",
		"http://test.com/inner-page/relative-with-dot",
		"http://test.com/inner-page/relative-no-dot",
		"http://test.com/relative-5-dir-up",
		"http://test.com/item-1",
		"http://test.com/item-2",
		"http://test.com/image",
		"http://test.com/banner",
		"https://example.com/page?param=value",
		"http://example.com/page?param=value",
	}

	require.ElementsMatch(t, wantLinks, urlsAsStrings(t, gotLinks))
}

func TestExtractLinks_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks, err := ExtractLinks(doc, docURL(t))
	require.NoError(t, err)

	require.Equal(t, 0, len(gotLinks))
}

func TestExtractLinks_InvalidHTML(t *testing.T) {
	pageContent := loadFixture(t, "not-html.txt")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks, err := ExtractLinks(doc, docURL(t))
	require.NoError(t, err)

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

func urlsAsStrings(t *testing.T, urls []*url.URL) []string {
	t.Helper()

	result := make([]string, len(urls))

	for i, u := range urls {
		result[i] = u.String()
	}

	return result
}
