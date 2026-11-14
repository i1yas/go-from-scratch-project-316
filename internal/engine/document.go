package engine

import "strings"

type documentType string

const (
	documentTypeHTML documentType = "html"
	documentTypeRSS  documentType = "rss"
)

var contentTypeToDocumentType = map[string]documentType{
	"text/html":           documentTypeHTML,
	"application/rss+xml": documentTypeRSS,
	"application/xml":     documentTypeRSS,
	"text/atom+xml":       documentTypeRSS,
	"text/xml":            documentTypeRSS,
}

func documentTypeFromContentType(contentType string) (documentType, bool) {
	if contentType == "" {
		return "", false
	}

	parts := strings.Split(contentType, ";")
	t := strings.TrimSpace(parts[0])

	docType, ok := contentTypeToDocumentType[t]
	if !ok {
		return "", false
	}

	return docType, true
}
