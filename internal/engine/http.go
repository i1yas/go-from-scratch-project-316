package engine

import (
	"bytes"
	"io"
	"net/http"
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

func responseBodySize(resp *http.Response) (int64, error) {
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}

	if resp.Body == nil {
		return 0, nil
	}

	data, err := io.ReadAll(resp.Body)
	resp.Body.Close() // nolint: errcheck

	if err != nil {
		return 0, err
	}

	// NOTE: restore body
	resp.Body = io.NopCloser(bytes.NewReader(data))

	return int64(len(data)), nil
}
