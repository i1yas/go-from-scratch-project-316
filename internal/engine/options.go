package engine

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"
)

var (
	ErrInvalidDepth          = errors.New("invalid depth value")
	ErrInvalidRetries        = errors.New("invalid retries value")
	ErrInvalidDelay          = errors.New("invalid delay")
	ErrInvalidTimeout        = errors.New("invalid timeout")
	ErrInvalidConcurrency    = errors.New("invalid concurrency value")
	ErrHTTPClientNotProvided = errors.New("http client not provided")
)

// Options contains engine options
type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int

	HTTPClient *http.Client
	Clock      func() time.Time
}

func validateOptions(opts Options) error {
	if opts.HTTPClient == nil {
		return ErrHTTPClientNotProvided
	}

	if err := validateMinInt(opts.Depth, 0); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDepth, err)
	}

	if err := validateMinInt(opts.Retries, 0); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRetries, err)
	}

	if opts.Delay < time.Duration(0) {
		return fmt.Errorf("%w: negative duration %s", ErrInvalidDelay, opts.Delay)
	}

	if opts.Timeout < time.Duration(0) {
		return fmt.Errorf("%w: negative duration=%s", ErrInvalidTimeout, opts.Timeout)
	}

	if err := validateMinInt(opts.Concurrency, 1); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConcurrency, err)
	}

	return nil
}

func validateMinInt(value, minValue int) error {
	return validateInt(value, minValue, math.MaxInt)
}

func validateInt(value, minValue, maxValue int) error {
	if value < minValue {
		return fmt.Errorf("too small, got %d, min is %d", value, minValue)
	}

	if value > maxValue {
		return fmt.Errorf("too big, got %d, max is %d", value, maxValue)
	}

	return nil
}
