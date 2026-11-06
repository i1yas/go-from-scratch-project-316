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

func analyzeSEO(doc *goquery.Document) PageSEO {
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

	hasH1 := doc.Find(`html > body > h1`).Length() > 0
	if hasH1 {
		seo.HasH1 = true
	}

	return seo
}
