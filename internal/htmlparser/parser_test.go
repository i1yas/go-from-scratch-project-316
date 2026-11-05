package htmlparser

import (
	"bytes"
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

	gotLinks, err := ExtractLinks(doc)
	require.NoError(t, err)

	wantLinks := []string{
		"/link",
		"/item-1",
		"/item-2",
		"/image",
		"/banner",
		"https://example.com/page?param=value",
	}

	require.ElementsMatch(t, wantLinks, gotLinks)
}

func TestExtractLinks_PageWithoutLinks(t *testing.T) {
	pageContent := loadFixture(t, "page-without-links.html")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks, err := ExtractLinks(doc)
	require.NoError(t, err)

	require.Equal(t, 0, len(gotLinks))
}

func TestExtractLinks_InvalidHTML(t *testing.T) {
	pageContent := loadFixture(t, "not-html.txt")
	pageContentReader := bytes.NewReader(pageContent)

	doc, err := goquery.NewDocumentFromReader(pageContentReader)
	require.NoError(t, err)

	gotLinks, err := ExtractLinks(doc)
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
