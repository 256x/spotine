package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) *SpotifyClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prevBase := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = prevBase })

	return NewSpotifyClient(&TokenData{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}, "id", "secret", func(string, ...any) {})
}

// A short Retry-After is ordinary throttling: the client waits it out and
// returns the eventual success.
func TestDoRetriesShortRateLimit(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	resp, err := c.do(context.Background(), "GET", "/v1/me/player/devices", nil)
	if err != nil {
		t.Fatalf("do() = %v", err)
	}
	resp.Body.Close()
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

// A long Retry-After — Spotify's app-wide quota block — must fail fast
// instead of blocking the caller for hours.
func TestDoFailsFastOnLongRateLimit(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "18620")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	done := make(chan error, 1)
	go func() {
		_, err := c.do(context.Background(), "GET", "/v1/me/player/devices", nil)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("do() = nil error, want a rate-limit error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("do() blocked instead of failing fast on a long Retry-After")
	}
}
