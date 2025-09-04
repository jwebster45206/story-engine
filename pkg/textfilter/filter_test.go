package textfilter

import (
	"testing"
)

func TestProfanityFilter_FilterText(t *testing.T) {
	filter := NewProfanityFilter()

	tests := []struct {
		name          string
		input         string
		contentRating string
		expected      string
	}{
		{
			name:          "G rating - simple profanity replacement",
			input:         "What the hell is going on?",
			contentRating: "G",
			expected:      "What the heck is going on?",
		},
		{
			name:          "PG rating - multiple profanities",
			input:         "This is damn crap!",
			contentRating: "PG",
			expected:      "This is dang crud!",
		},
		{
			name:          "PG13 rating - allows mild profanity",
			input:         "What the hell is this damn thing?",
			contentRating: "PG13",
			expected:      "What the hell is this damn thing?", // hell and damn allowed in PG13
		},
		{
			name:          "PG13 rating - still filters strong profanity",
			input:         "This shit is fucking crazy!",
			contentRating: "PG13",
			expected:      "This shoot is fudging crazy!",
		},
		{
			name:          "PG13 rating - mixed mild and strong profanity",
			input:         "Hell yeah, that's some badass shit right there!",
			contentRating: "PG13",
			expected:      "Hell yeah, that's some badass shoot right there!",
		},
		{
			name:          "R rating - no filtering",
			input:         "What the hell is this damn shit?",
			contentRating: "R",
			expected:      "What the hell is this damn shit?",
		},
		{
			name:          "Empty rating - no filtering",
			input:         "What the hell is this damn shit?",
			contentRating: "",
			expected:      "What the hell is this damn shit?",
		},
		{
			name:          "Unknown rating - no filtering",
			input:         "What the hell is this damn shit?",
			contentRating: "UNKNOWN",
			expected:      "What the hell is this damn shit?",
		},
		{
			name:          "case preservation - uppercase",
			input:         "DAMN that's annoying!",
			contentRating: "PG",
			expected:      "DANG that's annoying!",
		},
		{
			name:          "case preservation - title case",
			input:         "Hell no, that's not right",
			contentRating: "PG",
			expected:      "Heck no, that's not right",
		},
		{
			name:          "word boundaries - partial matches should not be replaced",
			input:         "I love classical music",
			contentRating: "G",
			expected:      "I love classical music", // "ass" in "classical" should not be replaced
		},
		{
			name:          "mild profanity replacement",
			input:         "You're such a bastard!",
			contentRating: "PG",
			expected:      "You're such a jerk!",
		},
		{
			name:          "no profanity",
			input:         "This is a perfectly clean sentence.",
			contentRating: "G",
			expected:      "This is a perfectly clean sentence.",
		},
		{
			name:          "empty string",
			input:         "",
			contentRating: "G",
			expected:      "",
		},
		{
			name:          "profanity with punctuation",
			input:         "What the hell?! That's damn crazy.",
			contentRating: "PG",
			expected:      "What the heck?! That's dang crazy.",
		},
		{
			name:          "mixed case profanity",
			input:         "HeLl yeah, that's DaMn good!",
			contentRating: "PG",
			expected:      "HeCk yeah, that's DaNg good!",
		},
		{
			name:          "plural profanity",
			input:         "There are too many assholes and bastards here!",
			contentRating: "PG",
			expected:      "There are too many jerks and jerks here!",
		},
		{
			name:          "PG13 allows asshole",
			input:         "That guy is such an asshole!",
			contentRating: "PG13",
			expected:      "That guy is such an asshole!", // asshole allowed in PG13
		},
		{
			name:          "non-pluralizable words should not match extra s",
			input:         "I need to process this data",
			contentRating: "G",
			expected:      "I need to process this data", // "ass" in "process" should not match, even with 's'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.FilterText(tt.input, tt.contentRating)
			if result != tt.expected {
				t.Errorf("FilterText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProfanityFilter_ContainsProfanity(t *testing.T) {
	filter := NewProfanityFilter()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "contains mild profanity",
			input:    "What the hell is this?",
			expected: true,
		},
		{
			name:     "contains multiple profanities",
			input:    "This damn crap is annoying",
			expected: true,
		},
		{
			name:     "no profanity",
			input:    "This is a clean sentence",
			expected: false,
		},
		{
			name:     "partial word match should not trigger",
			input:    "I love classical music",
			expected: false, // "ass" in "classical" should not trigger
		},
		{
			name:     "case insensitive detection",
			input:    "HELL no!",
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "contains plural profanity",
			input:    "There are multiple hells on earth",
			expected: true,
		},
		{
			name:     "plural mixed case detection",
			input:    "These DAMNS are everywhere!",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.ContainsProfanity(tt.input)
			if result != tt.expected {
				t.Errorf("ContainsProfanity() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProfanityFilter_Integration(t *testing.T) {
	filter := NewProfanityFilter()

	// Test a realistic user input scenario with plurals for PG rating
	userInput := "That boss fight was damn hard! What the hells were the developers thinking? There are too many assholes in this game."
	filtered := filter.FilterText(userInput, "PG")
	expected := "That boss fight was dang hard! What the hecks were the developers thinking? There are too many jerks in this game."

	if filtered != expected {
		t.Errorf("PG Integration test failed:\nInput:    %q\nExpected: %q\nGot:      %q", userInput, expected, filtered)
	}

	// Test PG13 rating allows some mild profanity
	pg13Filtered := filter.FilterText(userInput, "PG13")
	pg13Expected := "That boss fight was damn hard! What the hells were the developers thinking? There are too many assholes in this game."

	if pg13Filtered != pg13Expected {
		t.Errorf("PG13 Integration test failed:\nInput:    %q\nExpected: %q\nGot:      %q", userInput, pg13Expected, pg13Filtered)
	}

	// Test no filtering for mature content
	noFilterFiltered := filter.FilterText(userInput, "R")
	if noFilterFiltered != userInput {
		t.Errorf("R rating should not filter:\nInput:    %q\nGot:      %q", userInput, noFilterFiltered)
	}

	// Verify the original contained profanity
	if !filter.ContainsProfanity(userInput) {
		t.Errorf("Original input should contain profanity")
	}

	// Verify the PG filtered version does not contain profanity
	if filter.ContainsProfanity(filtered) {
		t.Errorf("PG filtered input should not contain profanity")
	}
}
