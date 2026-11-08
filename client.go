package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// maxRetryAfter is the longest Retry-After this client will wait out inline.
// Anything beyond it is a block worth surfacing, not sleeping through.
const maxRetryAfter = 30

// apiBase is a var, not a const, so tests can point it at an httptest server.
var apiBase = "https://api.spotify.com"

type SpotifyClient struct {
	mu           sync.Mutex
	token        *TokenData
	clientID     string
	clientSecret string
	acceptLang   string
	httpClient   *http.Client
	debugLog     func(string, ...any)
}

func NewSpotifyClient(token *TokenData, clientID, clientSecret string, debug func(string, ...any)) *SpotifyClient {
	return &SpotifyClient{
		token:        token,
		clientID:     clientID,
		clientSecret: clientSecret,
		acceptLang:   acceptLanguage(),
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		debugLog:     debug,
	}
}

func (c *SpotifyClient) Token() *TokenData {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *SpotifyClient) ensureFreshToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.token.Expiry.Add(-30 * time.Second)) {
		return nil
	}
	c.debugLog("refreshing token")
	newToken, err := RefreshToken(c.clientID, c.clientSecret, c.token)
	if err != nil {
		return fmt.Errorf("token refresh: %w", err)
	}
	c.token = newToken
	return SaveToken(newToken)
}

func (c *SpotifyClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.ensureFreshToken(); err != nil {
			return nil, err
		}

		var bodyReader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(data)
		}

		req, err := http.NewRequestWithContext(ctx, method, apiBase+path, bodyReader)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		req.Header.Set("Authorization", "Bearer "+c.token.AccessToken)
		c.mu.Unlock()
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		// Spotify returns names in the requested language where it has them,
		// so a Japanese act reads as サカナクション rather than "sakanaction".
		if c.acceptLang != "" {
			req.Header.Set("Accept-Language", c.acceptLang)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		retryAfter := 1
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if n, err := strconv.Atoi(ra); err == nil {
				retryAfter = n
			}
		}
		resp.Body.Close()

		// A short Retry-After is normal per-request throttling; wait it out.
		// Spotify's app-wide quota block returns one in the hours, and
		// sleeping through that would hang the popup for the rest of the
		// day with no way out. Fail fast instead so the UI stays usable.
		if retryAfter > maxRetryAfter {
			return nil, fmt.Errorf("rate limited by Spotify, retry after %s", time.Duration(retryAfter)*time.Second)
		}

		c.debugLog("rate limited, sleeping %ds", retryAfter)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(retryAfter) * time.Second):
		}
	}
	return nil, fmt.Errorf("rate limited: max retries exceeded")
}

// getJSON performs one GET and decodes the body into out.
func (c *SpotifyClient) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %d %s", path, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// pager is the shape every paginated Spotify response shares.
type pager[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next"`
}

// paginate walks a paginated endpoint, calling collect for each page, until
// the results run out or maxPages is reached. Spotify returns "next" as a full
// URL, so it is reduced back to a path before the next request.
//
// Every list endpoint used to repeat this loop; it lives here once instead.
func paginate[T any](ctx context.Context, c *SpotifyClient, path string, maxPages int, collect func([]T)) error {
	for page := 0; path != "" && page < maxPages; page++ {
		var p pager[T]
		if err := c.getJSON(ctx, path, &p); err != nil {
			return err
		}
		collect(p.Items)

		if p.Next == "" {
			return nil
		}
		u, err := url.Parse(p.Next)
		if err != nil || u.Path == "" {
			return nil
		}
		path = u.Path
		if u.RawQuery != "" {
			path += "?" + u.RawQuery
		}
	}
	return nil
}

func deviceQuery(deviceID string) string {
	if deviceID == "" {
		return ""
	}
	return "?device_id=" + url.QueryEscape(deviceID)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("unexpected status %d", resp.StatusCode)
}
