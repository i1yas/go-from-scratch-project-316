package app

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"code/crawler"
)

func TestParseOptions(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want crawler.Options
		err  error
	}{
		{
			name: "default flag values",
			args: []string{"http://test.com"},
			want: crawler.Options{
				URL:         "http://test.com",
				Depth:       10,
				Retries:     1,
				Concurrency: 4,
				Timeout:     time.Second * 15,
			},
		},
		{
			name: "main flags set",
			args: []string{
				"http://test.com",
				"--depth", "11",
				"--retries", "3",
				"--delay", "17s",
				"--timeout", "5s",
				"--user-agent", "test-user-agent",
				"--workers", "8",
			},
			want: crawler.Options{
				URL:         "http://test.com",
				Depth:       11,
				Retries:     3,
				Delay:       time.Second * 17,
				Timeout:     time.Second * 5,
				Concurrency: 8,
				UserAgent:   "test-user-agent",
			},
		},
		{
			name: "rps overrides delay",
			args: []string{
				"http://test.com",
				"--delay", "1m",
				"--rps", "5",
			},
			want: crawler.Options{
				URL:         "http://test.com",
				Depth:       10,
				Retries:     1,
				Concurrency: 4,
				Delay:       time.Millisecond * 200,
				Timeout:     time.Second * 15,
			},
		},
		{
			name: "invalid url",
			args: []string{""},
			err:  ErrNoRootURL,
		},
		{
			name: "invalid depth",
			args: []string{
				"http://test.com",
				"--depth", "-5",
			},
			err: ErrInvalidDepth,
		},
		{
			name: "invalid retries",
			args: []string{
				"http://test.com",
				"--retries", "-5",
			},
			err: ErrInvalidRetries,
		},
		{
			name: "invalid delay",
			args: []string{
				"http://test.com",
				"--delay", "invalid",
			},
			err: ErrInvalidDelay,
		},
		{
			name: "invalid timeout",
			args: []string{
				"http://test.com",
				"--timeout", "invalid",
			},
			err: ErrInvalidTimeout,
		},
		{
			name: "invalid RPS",
			args: []string{
				"http://test.com",
				"--rps", "-5",
			},
			err: ErrInvalidRPS,
		},
		{
			name: "invalid workers",
			args: []string{
				"http://test.com",
				"--workers", "-5",
			},
			err: ErrInvalidWorkers,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotOptions crawler.Options
				gotErr     error
			)

			cmd := BuildCommand()
			cmd.Action = func(_ context.Context, c *cli.Command) error {
				gotOptions, gotErr = parseOptions(c)
				return nil
			}

			cmd.Writer = io.Discard
			cmd.ErrWriter = io.Discard

			args := append([]string{"crawler"}, tc.args...)

			require.NoError(t, cmd.Run(t.Context(), args))

			require.ErrorIs(t, gotErr, tc.err)
			require.Equal(t, tc.want, gotOptions)
		})
	}
}
