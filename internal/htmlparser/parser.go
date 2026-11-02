package htmlparser

import (
	"io"

	"golang.org/x/net/html"
)

// ExtractLinks extracts links from html document
func ExtractLinks(r io.Reader) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	links := extractLinksFromNode(doc)

	return links, nil
}

func extractLinksFromNode(node *html.Node) []string {
	var links []string

	switch node.Data {
	case "a":
		href, ok := attr(node, "href")
		if ok {
			links = append(links, href)
		}
	case "link":
		l, ok := linkFromLinkTag(node)
		if ok {
			links = append(links, l)
		}
	case "script":
		src, ok := attr(node, "src")
		if ok {
			links = append(links, src)
		}
	case "img":
		src, ok := attr(node, "src")
		if ok {
			links = append(links, src)
		}
		// TODO: video, audio, source, etc
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		links = append(links, extractLinksFromNode(c)...)
	}

	return links
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
