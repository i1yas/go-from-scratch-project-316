package engine

import (
	"strings"
	"unicode"
)

func isRetriableStatus(statusCode int) bool {
	return statusCode == 492 || statusCode >= 500
}

func isHTMLContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "text/html")
}

func statusText(statusLine string) string {
	for i, r := range statusLine {
		if unicode.IsSpace(r) {
			return strings.ToLower(statusLine[i+1:])
		}
	}

	return ""
}
