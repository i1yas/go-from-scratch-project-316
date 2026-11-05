package htmlparser

import (
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// ExtractLinks extracts links from html document
func ExtractLinks(doc *goquery.Document) ([]string, error) {
	var links []string

	doc.Each(func(i int, s *goquery.Selection) {
		node := s.Nodes[i]
		links = append(links, extractLinksFromNode(node)...)
	})

	return links, nil
}

func extractLinkFromNode(node *html.Node) (string, bool) {
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

func extractLinksFromNode(node *html.Node) []string {
	var links []string

	link, ok := extractLinkFromNode(node)
	if ok {
		links = append(links, link)
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
