package testutils

import (
	"context"
	"crypto/tls"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestServerRoute contains route config for test server
type TestServerRoute struct {
	Status            int
	Body              []byte
	ContentLength     int64
	OmitContentLength bool
	Fn                func(w http.ResponseWriter, r *http.Request)
}

// TestServer is test server that uses [httptest.Server] under the hood and collects metrics
type TestServer struct {
	Client   *http.Client
	calls    map[string]int
	timeline []time.Time
	mu       sync.Mutex
}

// SetupTestServer creates and sets up [TestServer]
func SetupTestServer(
	t *testing.T,
	host string,
	internalRoutesMapper map[string]TestServerRoute,
	externalRoutesMapper map[string]TestServerRoute,
) *TestServer {
	t.Helper()

	server := &TestServer{
		calls:    map[string]int{},
		timeline: []time.Time{},
		mu:       sync.Mutex{},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqT := time.Now()

		defer func() {
			server.mu.Lock()
			server.calls[r.URL.String()]++
			server.timeline = append(server.timeline, reqT)
			server.mu.Unlock()
		}()

		if r.Host != host {
			handleTestRequest(t, externalRoutesMapper, w, r)
			return
		}

		handleTestRequest(t, internalRoutesMapper, w, r)
	})

	plainServer := httptest.NewServer(handler)
	t.Cleanup(plainServer.Close)

	secureServer := httptest.NewTLSServer(handler)
	t.Cleanup(secureServer.Close)

	var d net.Dialer

	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			target := plainServer.Listener.Addr().String()
			if _, port, _ := net.SplitHostPort(addr); port == "443" { // nolint: errcheck
				target = secureServer.Listener.Addr().String()
			}

			return d.DialContext(ctx, network, target)
		},
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
	}

	client := &http.Client{Transport: tr}

	server.Client = client

	return server
}

// TotalCallsCount returns total calls count to server
func (ts *TestServer) TotalCallsCount() int {
	defer ts.mu.Unlock()

	ts.mu.Lock()

	return len(ts.timeline)
}

// CallsCountTo returns calls count to specified route
func (ts *TestServer) CallsCountTo(uri string) int {
	defer ts.mu.Unlock()

	ts.mu.Lock()

	return ts.calls[uri]
}

// MinDelayBetweenRequests calculates min delay between requests
func (ts *TestServer) MinDelayBetweenRequests() (time.Duration, bool) {
	defer ts.mu.Unlock()

	ts.mu.Lock()

	minDelay := time.Duration(math.MaxInt64)

	if len(ts.timeline) == 0 {
		return minDelay, false
	}

	lastT := ts.timeline[0]

	for i := 1; i < len(ts.timeline); i++ {
		t := ts.timeline[i]

		delay := t.Sub(lastT)
		if delay < minDelay {
			minDelay = delay
		}

		lastT = t
	}

	return minDelay, true
}

func handleTestRequest(
	t *testing.T,
	mapper map[string]TestServerRoute,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()

	respConf, ok := mapper[r.RequestURI]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if respConf.Fn != nil {
		respConf.Fn(w, r)
		return
	}

	if respConf.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(respConf.ContentLength, 10))
	}

	w.WriteHeader(respConf.Status)
	_, err := w.Write(respConf.Body)
	require.NoError(t, err)

	if respConf.OmitContentLength {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		flusher.Flush()
	}
}

// HijackAndCloseHandler is a handler that hijacks response and closes connection.
// This handler triggers [io.EOF] error on [http.Client]
func HijackAndCloseHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, _ *http.Request) {
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
	}
}
