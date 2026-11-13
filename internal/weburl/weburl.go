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

// Parse parses, validates and noramlizes URL
func Parse(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParse, err)
	}

	if err := ValidateURL(parsed); err != nil {
		return nil, err
	}

	NormalizeURL(parsed)

	return parsed, nil
}

// NormalizeURL noramlizes given URL
func NormalizeURL(u *url.URL) {
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
}

// ValidateURL validates given URL
func ValidateURL(u *url.URL) error {
	switch u.Scheme {
	case "http", "https":
		break
	default:
		return fmt.Errorf("%w '%s'", ErrUnsupportedProtocol, u.Scheme)
	}

	return nil
}
