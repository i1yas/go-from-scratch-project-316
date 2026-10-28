package weburl

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrFailedToParse       = errors.New("failed to parse")
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
)

// Parse parses and validates URL
func Parse(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParse, err)
	}

	switch parsed.Scheme {
	case "http", "https":
		break
	default:
		return nil, fmt.Errorf("%w '%s'", ErrUnsupportedProtocol, parsed.Scheme)
	}

	parsed.Host = strings.ToLower(parsed.Host)

	return parsed, nil
}
