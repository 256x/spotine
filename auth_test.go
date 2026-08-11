package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolateConfig points both the current and the legacy config dirs at a temp
// tree, so these tests never touch the real ones.
func isolateConfig(t *testing.T) (spotineDir, legacyDir string) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	return filepath.Join(base, "spotine"), filepath.Join(base, legacyApp)
}

func writeToken(t *testing.T, dir, refresh string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(TokenData{
		AccessToken:  "access",
		RefreshToken: refresh,
		Expiry:       time.Now().Add(time.Hour),
	})
	if err := os.WriteFile(filepath.Join(dir, "token.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptLegacyTokenImportsOnce(t *testing.T) {
	_, legacy := isolateConfig(t)
	writeToken(t, legacy, "legacy-refresh")

	if !AdoptLegacyToken() {
		t.Fatal("first run should import the old token")
	}
	got, err := LoadToken()
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshToken != "legacy-refresh" {
		t.Errorf("imported %q, want legacy-refresh", got.RefreshToken)
	}

	// Logging out must stick. Without the marker the next launch would import
	// the same token again and logout would appear to do nothing.
	if err := DeleteToken(); err != nil {
		t.Fatal(err)
	}
	if AdoptLegacyToken() {
		t.Error("import repeated after logout")
	}
	if _, err := LoadToken(); err == nil {
		t.Error("token came back after logout")
	}
}

func TestAdoptLegacyTokenSkipsWhenAlreadySignedIn(t *testing.T) {
	spotine, legacy := isolateConfig(t)
	writeToken(t, legacy, "legacy-refresh")
	writeToken(t, spotine, "own-refresh")

	if AdoptLegacyToken() {
		t.Error("should not overwrite an existing token")
	}
	got, err := LoadToken()
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshToken != "own-refresh" {
		t.Errorf("token was replaced with %q", got.RefreshToken)
	}

	// Having been signed in already counts as migrated, so a later logout is
	// not undone either.
	if err := DeleteToken(); err != nil {
		t.Fatal(err)
	}
	if AdoptLegacyToken() {
		t.Error("import ran after logout on an already-signed-in install")
	}
}

// Nothing to import must not be recorded as migrated, or installing the old
// app afterwards would never hand its token over.
func TestAdoptLegacyTokenWithNothingToImport(t *testing.T) {
	_, legacy := isolateConfig(t)

	if AdoptLegacyToken() {
		t.Error("nothing to import, yet it reported success")
	}
	writeToken(t, legacy, "arrived-later")
	if !AdoptLegacyToken() {
		t.Error("a token that appeared later should still be imported")
	}
}
