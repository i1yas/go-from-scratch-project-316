package crawler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyze(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		client := server.Client()

		url := "http://test.com"

		want := reportDTO{
			RootURL: url,
			Depth:   1,
			Pages: []pageReportDTO{
				{
					URL:        url,
					Depth:      0,
					HTTPStatus: 200,
					Status:     "ok",
					Error:      "",
				},
			},
		}

		gotRaw, err := Analyze(t.Context(), Options{
			URL:        url,
			Depth:      1,
			UserAgent:  "test",
			HTTPClient: client,
		})
		require.NoError(t, err)

		requireReportEquals(t, want, gotRaw)
	})

	t.Run("network error", func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("hijacking not supported")
			}

			conn, _, err := hj.Hijack()
			if err != nil {
				t.Error(err)
				return
			}

			require.NoError(t, conn.Close())
		}))

		client := server.Client()

		url := "http://test.com"

		want := reportDTO{
			RootURL: url,
			Depth:   1,
			Pages: []pageReportDTO{
				{
					URL:   url,
					Depth: 0,
					Error: io.EOF.Error(),
				},
			},
		}

		gotRaw, err := Analyze(t.Context(), Options{
			URL:        url,
			Depth:      1,
			UserAgent:  "test",
			HTTPClient: client,
		})
		require.NoError(t, err)

		requireReportEquals(t, want, gotRaw)
	})

	t.Run("error on canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		server := httptest.NewTestServer(
			t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		)

		_, err := Analyze(ctx, Options{
			URL:        "http://test.com",
			HTTPClient: server.Client(),
		})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestAnalyze_InvalidOptions(t *testing.T) {
	t.Run("invalid root url", func(t *testing.T) {
		server := httptest.NewTestServer(
			t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		)

		_, err := Analyze(t.Context(), Options{
			URL:        "invalid",
			HTTPClient: server.Client(),
		})
		require.ErrorIs(t, err, ErrInvalidRootURL)
	})

	t.Run("missing http client", func(t *testing.T) {
		_, err := Analyze(t.Context(), Options{
			URL: "http://test.com",
		})
		require.ErrorIs(t, err, ErrHTTPClientNotProvided)
	})
}

func requireReportEquals(t *testing.T, want reportDTO, gotRaw []byte) {
	t.Helper()

	var got reportDTO

	err := json.Unmarshal(gotRaw, &got)
	require.NoError(t, err)

	require.Equal(t, want.RootURL, got.RootURL)
	require.Equal(t, want.Depth, got.Depth)
	requireTimeIsValid(t, got.GeneratedAt)

	require.Equal(t, len(want.Pages), len(got.Pages))
	for i := range want.Pages {
		wantPage := want.Pages[i]
		gotPage := got.Pages[i]

		assert.Equal(t, wantPage.URL, gotPage.URL)
		assert.Equal(t, wantPage.Depth, gotPage.Depth)
		assert.Contains(t, gotPage.Error, wantPage.Error)
		assert.Equal(t, wantPage.HTTPStatus, gotPage.HTTPStatus)
		assert.Equal(t, wantPage.Status, gotPage.Status)

		requireTimeIsValid(t, gotPage.DiscoveredAt)
	}
}

func requireTimeIsValid(t *testing.T, value string) {
	t.Helper()

	_, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
}
