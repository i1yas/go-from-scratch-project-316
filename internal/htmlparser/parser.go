package htmlparser

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// LinkType represents link type: anchor, image, style, script or other
type LinkType string

const (
	// LinkTypeAnchor represents link from <a> tag
	LinkTypeAnchor LinkType = "anchor"
	// LinkTypeImage represents link from <img> tag
	LinkTypeImage LinkType = "image"
	// LinkTypeStyle represents link from <link rel="stylesheet" ...> tag
	LinkTypeStyle LinkType = "style"
	// LinkTypeScript represents link from <script> tag
	LinkTypeScript LinkType = "script"
	// LinkTypeOther groups all other links
	LinkTypeOther LinkType = "other"
)

// Link represents link in html document
type Link struct {
	Type LinkType
	URL  *url.URL
}

// ExtractLinks extracts links from html document
func ExtractLinks(doc *goquery.Document, docURL *url.URL) ([]Link, error) {
	var links []Link

	doc.Each(func(i int, s *goquery.Selection) {
		node := s.Nodes[i]
		links = append(links, extractLinksFromNode(node, docURL)...)
	})

	return links, nil
}

func extractLinksFromNode(node *html.Node, docURL *url.URL) []Link {
	var links []Link

	link, ok := extractLinkFromNode(node, docURL)
	if ok {
		links = append(links, link)
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		links = append(links, extractLinksFromNode(c, docURL)...)
	}

	return links
}

func extractLinkFromNode(node *html.Node, docURL *url.URL) (Link, bool) {
	rawLink, typ, ok := extractRawLinkFromNode(node)
	if !ok {
		return Link{}, false
	}

	u, ok := parseURL(rawLink, docURL)
	if !ok {
		return Link{}, false
	}

	return Link{
		Type: typ,
		URL:  u,
	}, true
}

func parseURL(rawLink string, docURL *url.URL) (*url.URL, bool) {
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

func extractRawLinkFromNode(node *html.Node) (string, LinkType, bool) {
	switch node.Data {
	case "a":
		href, ok := attr(node, "href")
		if !ok {
			return "", LinkTypeOther, false
		}

		return href, LinkTypeAnchor, true
	case "link":
		rel, ok := linkFromLinkTag(node)
		if !ok {
			return "", LinkTypeOther, false
		}

		return rel, LinkTypeStyle, true
	case "script":
		src, ok := attr(node, "src")
		if !ok {
			return "", LinkTypeOther, false
		}

		return src, LinkTypeScript, true
	case "img":
		src, ok := attr(node, "src")
		if !ok {
			return "", LinkTypeOther, false
		}

		return src, LinkTypeImage, true
		// TODO: video, audio, source, etc
	}

	return "", LinkTypeOther, false
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
