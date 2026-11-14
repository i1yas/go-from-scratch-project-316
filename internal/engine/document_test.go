package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocumentTypeFromContentType(t *testing.T) {
	cases := []struct {
		name           string
		contentType    string
		want           documentType
		isNotSupported bool
	}{
		{
			name:        "text.html is html",
			contentType: "text/html",
			want:        documentTypeHTML,
		},
		{
			name:        "application.rss+xml is rss",
			contentType: "application/rss+xml",
			want:        documentTypeRSS,
		},
		{
			name:        "application.xml is rss",
			contentType: "application/xml",
			want:        documentTypeRSS,
		},
		{
			name:        "text.xml is rss",
			contentType: "text/xml",
			want:        documentTypeRSS,
		},
		{
			name:        "html type with charset",
			contentType: "text/html; charset=utf-8",
			want:        documentTypeHTML,
		},
		{
			name:        "rss type with charset",
			contentType: "application/rss+xml; charset=utf-8",
			want:        documentTypeRSS,
		},
		{
			name:        "html type from long string",
			contentType: " text/html  ; charset=utf-8;  boundary=ExampleBoundaryString",
			want:        documentTypeHTML,
		},
		{
			name:        "rss type from long string",
			contentType: " application/rss+xml ; charset=utf-8;  boundary=ExampleBoundaryString",
			want:        documentTypeRSS,
		},

		{
			name:           "text.plain is not supported",
			contentType:    "text/plain",
			isNotSupported: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, isSupported := documentTypeFromContentType(tc.contentType)

			require.Equal(t, tc.isNotSupported, !isSupported)
			require.Equal(t, tc.want, got)
		})
	}
}
