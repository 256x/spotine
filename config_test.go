package main

import "testing"

func TestLocaleToTag(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"ja_JP.UTF-8", "ja-JP"},
		{"ja_JP", "ja-JP"},
		{"ja", "ja"},
		{"en_US.UTF-8", "en-US"},
		{"de_DE@euro", "de-DE"},
		{"  ja_JP.UTF-8  ", "ja-JP"},

		// No language to ask for.
		{"", ""},
		{"C", ""},
		{"POSIX", ""},
		{"C.UTF-8", ""},
	}
	for _, tt := range tests {
		if got := localeToTag(tt.in); got != tt.want {
			t.Errorf("localeToTag(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAcceptLanguage(t *testing.T) {
	saved := cfg
	defer func() { cfg = saved }()

	// An explicit setting wins over the environment.
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_US.UTF-8")
	cfg.UI.Language = "ja-JP"
	if got := acceptLanguage(); got != "ja-JP" {
		t.Errorf("configured value = %q, want ja-JP", got)
	}

	// "none" opts out entirely, even with a locale set.
	for _, v := range []string{"none", "NONE", " none "} {
		cfg.UI.Language = v
		if got := acceptLanguage(); got != "" {
			t.Errorf("language=%q should send no header, got %q", v, got)
		}
	}

	// Unset follows the shell locale, most specific variable first.
	cfg.UI.Language = ""
	if got := acceptLanguage(); got != "en-US" {
		t.Errorf("LANG fallback = %q, want en-US", got)
	}
	t.Setenv("LC_MESSAGES", "fr_FR.UTF-8")
	if got := acceptLanguage(); got != "fr-FR" {
		t.Errorf("LC_MESSAGES should beat LANG, got %q", got)
	}
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	if got := acceptLanguage(); got != "ja-JP" {
		t.Errorf("LC_ALL should beat LC_MESSAGES, got %q", got)
	}

	// A locale-less environment asks for nothing rather than sending garbage.
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_MESSAGES", "C")
	t.Setenv("LANG", "C")
	if got := acceptLanguage(); got != "" {
		t.Errorf("C locale should send no header, got %q", got)
	}
}
