package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/urfave/cli/v3"

	"code/crawler"
)

var (
	ErrNoRootURL      = errors.New("no root url provided")
	ErrInvalidDepth   = errors.New("invalid depth value")
	ErrInvalidRetries = errors.New("invalid retries amount")
	ErrInvalidDelay   = errors.New("invalid delay duration")
	ErrInvalidTimeout = errors.New("invalid timeout duration")
	ErrInvalidRPS     = errors.New("invalid RPS value")
	ErrInvalidWorkers = errors.New("invalid amount of workers")
)

// BuildCommand build crawler command
func BuildCommand() *cli.Command {
	flags := processFlags([]cli.Flag{
		&cli.IntFlag{
			Name:  "depth",
			Usage: "crawl depth",
			Value: 10,
		},
		&cli.IntFlag{
			Name:  "retries",
			Usage: "number of retries for failed requests",
			Value: 1,
		},
		&cli.StringFlag{
			Name:  "delay",
			Usage: "delay between requests (example: 200ms, 1s)",
			Value: "0s",
		},
		&cli.StringFlag{
			Name:  "timeout",
			Usage: "per-request timeout",
			Value: "15s",
		},
		&cli.IntFlag{
			Name:  "rps",
			Usage: "limit requests per second (overrides delay)",
			Value: 0,
		},
		&cli.StringFlag{
			Name:  "user-agent",
			Usage: "custom user agent",
		},
		&cli.IntFlag{
			Name:  "workers",
			Usage: "number of concurrent workers",
			Value: 4,
		},
	})

	return &cli.Command{
		Name:      "hexlet-go-crawler",
		Usage:     "analyze a website structure",
		UsageText: "hexlet-go-crawler [global options] command [command options] <url>",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "url",
				UsageText: "<url>",
				Required:  true,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			opts, err := parseOptions(cmd)
			if err != nil {
				return err
			}

			opts.HTTPClient = &http.Client{}

			output, err := crawler.Analyze(ctx, opts)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.Root().Writer, string(output))
			if err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			return nil
		},
		Flags: flags,
	}
}

func processFlags(flags []cli.Flag) []cli.Flag {
	for _, flag := range flags {
		switch v := flag.(type) {
		case *cli.StringFlag:
			// NOTE: do not show quotes for default values
			v.DefaultText = v.Value
		}
	}

	return flags
}

func parseOptions(cmd *cli.Command) (crawler.Options, error) {
	rootURL := cmd.StringArg("url")
	if rootURL == "" {
		return crawler.Options{}, ErrNoRootURL
	}

	depth := cmd.Int("depth")
	if err := validateInt(depth, 0, math.MaxInt, ErrInvalidDepth); err != nil {
		return crawler.Options{}, err
	}

	retries := cmd.Int("retries")
	if err := validateInt(retries, 0, math.MaxInt, ErrInvalidRetries); err != nil {
		return crawler.Options{}, err
	}

	delay, err := time.ParseDuration(cmd.String("delay"))
	if err != nil {
		return crawler.Options{}, fmt.Errorf("%w: %w", ErrInvalidDelay, err)
	}

	timeout, err := time.ParseDuration(cmd.String("timeout"))
	if err != nil {
		return crawler.Options{}, fmt.Errorf("%w: %w", ErrInvalidTimeout, err)
	}

	workers := cmd.Int("workers")
	if err := validateInt(workers, 1, math.MaxInt, ErrInvalidWorkers); err != nil {
		return crawler.Options{}, err
	}

	userAgent := cmd.String("user-agent")

	rps := cmd.Int("rps")
	if err := validateInt(rps, 0, math.MaxInt, ErrInvalidRPS); err != nil {
		return crawler.Options{}, err
	}

	if rps > 0 {
		delay = time.Second / time.Duration(rps)
	}

	opts := crawler.Options{
		URL:         rootURL,
		Depth:       depth,
		Retries:     retries,
		Delay:       delay,
		Timeout:     timeout,
		Concurrency: workers,
		UserAgent:   userAgent,
	}

	return opts, nil
}

func validateInt(value, minValue, maxValue int, baseErr error) error {
	if value < minValue {
		return fmt.Errorf("%w: too small, got %d, min is %d", baseErr, value, minValue)
	}

	if value > maxValue {
		return fmt.Errorf("%w: too big, got %d, max is %d", baseErr, value, maxValue)
	}

	return nil
}
