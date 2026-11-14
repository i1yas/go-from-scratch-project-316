package engine

import "github.com/PuerkitoBio/goquery"

// PageSEO contains page's SEO report
type PageSEO struct {
	HasTitle       bool
	HasDescription bool
	HasH1          bool
	Title          string
	Description    string
}

func analyzeSEO(doc *goquery.Document, docType documentType) PageSEO {
	switch docType {
	case documentTypeHTML:
		return analyzeSEOFromHTML(doc)
	case documentTypeRSS:
		return analyzeSEOFromRSS(doc)
	default:
		return PageSEO{}
	}
}

func analyzeSEOFromHTML(doc *goquery.Document) PageSEO {
	seo := PageSEO{}

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

	seo.HasH1 = doc.Find(`h1`).Length() > 0

	return seo
}

func analyzeSEOFromRSS(doc *goquery.Document) PageSEO {
	seo := PageSEO{}

	title := doc.
		Find("title").
		First().
		Text()
	if title != "" {
		seo.HasTitle = true
		seo.Title = title
	}

	return seo
}
