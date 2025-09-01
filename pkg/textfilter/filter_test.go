package textfilter

import (
	"testing"
)

func TestProfanityFilter_FilterText(t *testing.T) {
	filter := NewProfanityFilter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple profanity replacement",
			input:    "What the hell is going on?",
			expected: "What the heck is going on?",
		},
		{
			name:     "multiple profanities",
			input:    "This is damn crap!",
			expected: "This is dang crud!",
		},
		{
			name:     "case preservation - uppercase",
			input:    "DAMN that's annoying!",
			expected: "DANG that's annoying!",
		},
		{
			name:     "case preservation - title case",
			input:    "Hell no, that's not right",
			expected: "Heck no, that's not right",
		},
		{
			name:     "word boundaries - partial matches should not be replaced",
			input:    "I love classical music",
			expected: "I love classical music", // "ass" in "classical" should not be replaced
		},
		{
			name:     "mild profanity replacement",
			input:    "You're such a bastard!",
			expected: "You're such a jerk!",
		},
		{
			name:     "no profanity",
			input:    "This is a perfectly clean sentence.",
			expected: "This is a perfectly clean sentence.",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "profanity with punctuation",
			input:    "What the hell?! That's damn crazy.",
			expected: "What the heck?! That's dang crazy.",
		},
		{
			name:     "mixed case profanity",
			input:    "HeLl yeah, that's DaMn good!",
			expected: "HeCk yeah, that's DaNg good!",
		},
		{
			name:     "plural profanity",
			input:    "There are too many assholes and bastards here!",
			expected: "There are too many jerks and jerks here!",
		},
		{
			name:     "non-pluralizable words should not match extra s",
			input:    "I need to process this data",
			expected: "I need to process this data", // "ass" in "process" should not match, even with 's'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.FilterText(tt.input)
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

func TestShouldFilterContent(t *testing.T) {
	tests := []struct {
		name     string
		rating   string
		expected bool
	}{
		{
			name:     "G rating should filter",
			rating:   "G",
			expected: true,
		},
		{
			name:     "PG rating should filter",
			rating:   "PG",
			expected: true,
		},
		{
			name:     "PG13 rating should filter",
			rating:   "PG13",
			expected: true,
		},
		{
			name:     "PG-13 rating should filter",
			rating:   "PG-13",
			expected: true,
		},
		{
			name:     "R rating should not filter",
			rating:   "R",
			expected: false,
		},
		{
			name:     "lowercase ratings should work",
			rating:   "pg",
			expected: true,
		},
		{
			name:     "rating with whitespace",
			rating:   " PG13 ",
			expected: true,
		},
		{
			name:     "unknown rating should not filter",
			rating:   "NC-17",
			expected: false,
		},
		{
			name:     "empty rating should not filter",
			rating:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldFilterContent(tt.rating)
			if result != tt.expected {
				t.Errorf("ShouldFilterContent() = %v, want %v for rating %q", result, tt.expected, tt.rating)
			}
		})
	}
}

func TestProfanityFilter_Integration(t *testing.T) {
	filter := NewProfanityFilter()

	// Test a realistic user input scenario with plurals
	userInput := "That boss fight was damn hard! What the hells were the developers thinking? There are too many assholes in this game."
	filtered := filter.FilterText(userInput)
	expected := "That boss fight was dang hard! What the hecks were the developers thinking? There are too many jerks in this game."

	if filtered != expected {
		t.Errorf("Integration test failed:\nInput:    %q\nExpected: %q\nGot:      %q", userInput, expected, filtered)
	}

	// Verify the original contained profanity
	if !filter.ContainsProfanity(userInput) {
		t.Errorf("Original input should contain profanity")
	}

	// Verify the filtered version does not contain profanity
	if filter.ContainsProfanity(filtered) {
		t.Errorf("Filtered input should not contain profanity")
	}
}
