package engine

import "strings"

type documentType string

const (
	documentTypeHTML documentType = "html"
	documentTypeRSS  documentType = "rss"
)

var supportedDocumentTypes = map[string]documentType{
	"text/html":           documentTypeHTML,
	"application/rss+xml": documentTypeRSS,
}

func documentTypeFromContentType(contentType string) (documentType, bool) {
	if contentType == "" {
		return "", false
	}

	parts := strings.Split(contentType, ";")
	t := strings.TrimSpace(parts[0])

	docType, ok := supportedDocumentTypes[t]
	if !ok {
		return "", false
	}

	return docType, true
}
