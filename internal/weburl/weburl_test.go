package weburl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		err   error
	}{
		{
			name:  "valid http url",
			input: "http://example.com",
			want:  "http://example.com",
		},
		{
			name:  "valid https url",
			input: "https://example.com",
			want:  "https://example.com",
		},
		{
			name:  "valid url without tld",
			input: "http://example",
			want:  "http://example",
		},
		{
			name:  "valid url with path",
			input: "http://example.com/nested/page",
			want:  "http://example.com/nested/page",
		},
		{
			name:  "valid url with query params",
			input: "http://example.com?param=value",
			want:  "http://example.com?param=value",
		},
		{
			name:  "host normalized",
			input: "http://EXAMPLE.COM",
			want:  "http://example.com",
		},
		{
			name:  "failed to parse",
			input: "://example.com",
			err:   ErrFailedToParse,
		},
		{
			name:  "invalid url with unsupported protocol",
			input: "ws://example.com",
			err:   ErrUnsupportedProtocol,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotParsed, err := Parse(tc.input)
			require.ErrorIs(t, err, tc.err)

			if tc.err == nil {
				require.Equal(t, gotParsed.String(), tc.want)
			}
		})
	}
}
