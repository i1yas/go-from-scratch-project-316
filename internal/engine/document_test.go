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
			name:        "html type",
			contentType: "text/html",
			want:        documentTypeHTML,
		},
		{
			name:        "rss type",
			contentType: "application/rss+xml",
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
			name:           "application/xml is not supported",
			isNotSupported: true,
		},
		{
			name:           "text/xml is not supported",
			isNotSupported: true,
		},
		{
			name:           "text/plain is not supported",
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
