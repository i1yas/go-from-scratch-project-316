package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
			Pages: []pageReport{
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

		var got reportDTO

		err = json.Unmarshal(gotRaw, &got)
		require.NoError(t, err)

		require.Equal(t, want.RootURL, got.RootURL)
		require.Equal(t, want.Depth, got.Depth)
		require.Equal(t, want.Pages, got.Pages)
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
		}

		gotRaw, err := Analyze(t.Context(), Options{
			URL:        url,
			Depth:      1,
			UserAgent:  "test",
			HTTPClient: client,
		})
		require.NoError(t, err)

		var got reportDTO

		err = json.Unmarshal(gotRaw, &got)
		require.NoError(t, err)

		require.Equal(t, want.RootURL, got.RootURL)
		require.Equal(t, want.Depth, got.Depth)
		require.Equal(t, 1, len(got.Pages))
		require.Contains(t, got.Pages[0].Error, "EOF")
	})

	t.Run("error on canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := Analyze(ctx, Options{})
		require.ErrorIs(t, err, context.Canceled)
	})
}
