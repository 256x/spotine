package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope"`
}

func tokenPath() string {
	return filepath.Join(configDir(), "token.json")
}

func LoadToken() (*TokenData, error) {
	return readToken(tokenPath())
}

func readToken(path string) (*TokenData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t TokenData
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func SaveToken(t *TokenData) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenPath(), data, 0o600)
}

func DeleteToken() error {
	return os.Remove(tokenPath())
}

// migrationMarker records that the one-time import from the superseded app has
// already been considered, so it is never repeated.
func migrationMarker() string {
	return filepath.Join(configDir(), ".migrated")
}

// AdoptLegacyToken copies slp's token on first run so upgrading does not force
// a fresh login. The token is copied rather than shared: slp is frozen, and two
// processes refreshing the same file would race.
//
// It runs at most once, ever. Without the marker, `--logout` would delete the
// token and the next launch would immediately import it again, so logging out
// would appear to do nothing.
func AdoptLegacyToken() bool {
	if _, err := os.Stat(migrationMarker()); err == nil {
		return false
	}
	if _, err := os.Stat(tokenPath()); err == nil {
		markMigrated()
		return false
	}
	t, err := readToken(filepath.Join(legacyConfigDir(), "token.json"))
	if err != nil {
		// Nothing to import; don't mark, so a later slp install still migrates.
		return false
	}
	if SaveToken(t) != nil {
		return false
	}
	markMigrated()
	return true
}

func markMigrated() {
	_ = os.MkdirAll(configDir(), 0o755)
	_ = os.WriteFile(migrationMarker(), nil, 0o644)
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// Authenticate runs the full OAuth flow and returns a token.
func Authenticate(clientID, clientSecret, redirectURI string) (*TokenData, error) {
	state := randomState()

	// Parse redirect URI to get callback path and port
	u, err := url.Parse(redirectURI)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect URI: %w", err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback")
			http.Error(w, "no code", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "<html><body><h2>Authentication successful. You can close this tab.</h2></body></html>")
		codeCh <- code
	})

	host := u.Host
	if u.Port() == "" {
		host = net.JoinHostPort(u.Hostname(), "8888")
	}
	ln, err := net.Listen("tcp", host)
	if err != nil {
		return nil, fmt.Errorf("cannot start callback server: %w", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authURL := buildAuthURL(clientID, redirectURI, state)
	fmt.Fprintf(os.Stderr, "Opening browser for Spotify login...\n%s\n", authURL)
	openBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}

	token, err := exchangeCode(clientID, clientSecret, redirectURI, code)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// The scope set matches slp's, so a token adopted from slp already carries
// everything spotine needs. Catalogue search and album browsing need no scope.
func buildAuthURL(clientID, redirectURI, state string) string {
	params := url.Values{
		"client_id":     {clientID},
		"response_type": {"code"},
		"redirect_uri":  {redirectURI},
		"state":         {state},
		"scope": {"user-read-playback-state user-modify-playback-state " +
			"playlist-read-private playlist-read-collaborative"},
	}
	return "https://accounts.spotify.com/authorize?" + params.Encode()
}

func exchangeCode(clientID, clientSecret, redirectURI, code string) (*TokenData, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	return postToken(clientID, clientSecret, form)
}

func RefreshToken(clientID, clientSecret string, t *TokenData) (*TokenData, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
	}
	newToken, err := postToken(clientID, clientSecret, form)
	if err != nil {
		return nil, err
	}
	// Spotify may not return a new refresh token; keep old one
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = t.RefreshToken
	}
	return newToken, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
}

var tokenHTTPClient = &http.Client{Timeout: 10 * time.Second}

func postToken(clientID, clientSecret string, form url.Values) (*TokenData, error) {
	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tokenHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	if tr.Error != "" {
		return nil, fmt.Errorf("token error: %s", tr.Error)
	}
	return &TokenData{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Expiry:       time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
		Scope:        tr.Scope,
	}, nil
}
