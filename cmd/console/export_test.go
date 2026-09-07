package main

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Deck One", "deck_one"},
		{`bad/name:file*?`, "badnamefile"},
		{`path\with:chars`, "pathwithchars"},
		{`Quote"Name`, "quotename"},
		{"Already_ok", "already_ok"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := sanitizeFilename(tt.in); got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
