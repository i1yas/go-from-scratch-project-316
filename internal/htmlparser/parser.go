package htmlparser

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// ExtractLinks extracts links from html document
func ExtractLinks(doc *goquery.Document, docURL *url.URL) ([]*url.URL, error) {
	var links []*url.URL

	doc.Each(func(i int, s *goquery.Selection) {
		node := s.Nodes[i]
		links = append(links, extractLinksFromNode(node, docURL)...)
	})

	return links, nil
}

// SEO contains document's SEO report
type SEO struct {
	HasTitle       bool
	HasDescription bool
	HasH1          bool
	Title          string
	Description    string
}

// AnalyzeSEO checks if document follows basic SEO recommendations
func AnalyzeSEO(doc *goquery.Document) SEO {
	seo := SEO{}

	title := doc.
		Find("html > head > title").
		First().
		Text()
	if title != "" {
		seo.HasTitle = true
		seo.Title = title
	}

	desc, ok := doc.
		Find(`html > head > meta[name="description"]`).
		First().
		Attr("content")
	if ok {
		seo.HasDescription = true
		seo.Description = desc
	}

	hasH1 := doc.Find(`html > body > h1`).Length() > 0
	if hasH1 {
		seo.HasH1 = true
	}

	return seo
}

func extractLinksFromNode(node *html.Node, docURL *url.URL) []*url.URL {
	var links []*url.URL

	link, ok := extractLinkFromNode(node, docURL)
	if ok {
		links = append(links, link)
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		links = append(links, extractLinksFromNode(c, docURL)...)
	}

	return links
}

func extractLinkFromNode(node *html.Node, docURL *url.URL) (*url.URL, bool) {
	rawLink, ok := extractRawLinkFromNode(node)
	if !ok {
		return nil, false
	}

	link, ok := parseLink(rawLink, docURL)
	if !ok {
		return nil, false
	}

	return link, true
}

func parseLink(rawLink string, docURL *url.URL) (*url.URL, bool) {
	parsedURL, err := url.Parse(rawLink)
	if err != nil {
		return nil, false
	}

	if strings.HasPrefix(rawLink, "//") {
		parsedURL.Scheme = docURL.Scheme
	}

	if parsedURL.Host == "" && parsedURL.Path == "" {
		return nil, false
	}

	if parsedURL.Scheme == "" && parsedURL.Host == "" {
		path, ok := joinPath(parsedURL, docURL)
		if !ok {
			return nil, false
		}

		parsedURL.Scheme = docURL.Scheme
		parsedURL.Host = docURL.Host
		parsedURL.Path = path
	}

	return parsedURL, true
}

func joinPath(link, docURL *url.URL) (string, bool) {
	if strings.HasPrefix(link.Path, "/") {
		return link.Path, true
	}

	path, err := url.JoinPath(docURL.Path, link.Path)
	if err != nil {
		return "", false
	}

	return path, true
}

func extractRawLinkFromNode(node *html.Node) (string, bool) {
	switch node.Data {
	case "a":
		return attr(node, "href")
	case "link":
		return linkFromLinkTag(node)
	case "script":
		return attr(node, "src")
	case "img":
		return attr(node, "src")
		// TODO: video, audio, source, etc
	}

	return "", false
}

func linkFromLinkTag(node *html.Node) (string, bool) {
	href, ok := attr(node, "href")
	if !ok {
		return "", false
	}

	rel, ok := attr(node, "rel")
	if !ok || rel != "stylesheet" {
		return "", false
	}

	return href, true
}

func attr(node *html.Node, key string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}

	return "", false
}
