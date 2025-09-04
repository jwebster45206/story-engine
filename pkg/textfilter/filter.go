package textfilter

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Common US English swear words that should be filtered for PG13 and lower content
var swearWords = []string{
	"fuck", "fucking", "shit", "damn", "hell", "ass", "bitch", "bastard", "crap",
	"piss", "cock", "dick", "pussy", "tits", "boobs", "whore", "slut",
	"fag", "retard", "nigger", "nigga", "spic", "chink", "kike",
	"motherfucker", "goddamn", "jesus christ", "christ", "asshole",
	"dumbass", "jackass", "smartass", "badass", "bullshit", "horseshit",
	"dipshit", "shithead", "dickhead", "prick", "douche", "douchebag",
}

// swearWordReplacements maps swear words to family-friendly alternatives
var swearWordReplacements = map[string]string{
	"fuck":         "fudge",
	"fucking":      "fudging",
	"shit":         "shoot",
	"damn":         "dang",
	"hell":         "heck",
	"ass":          "butt",
	"bitch":        "jerk",
	"bastard":      "jerk",
	"crap":         "crud",
	"piss":         "ticked",
	"cock":         "[censored]",
	"dick":         "jerk",
	"pussy":        "[censored]",
	"tits":         "[censored]",
	"boobs":        "[censored]",
	"whore":        "[censored]",
	"slut":         "[censored]",
	"fag":          "[censored]",
	"retard":       "[censored]",
	"nigger":       "[censored]",
	"nigga":        "[censored]",
	"spic":         "[censored]",
	"chink":        "[censored]",
	"kike":         "[censored]",
	"motherfucker": "mother-trucker",
	"goddamn":      "gosh-dang",
	"jesus christ": "jeez",
	"christ":       "crikey",
	"asshole":      "jerk",
	"dumbass":      "dummy",
	"jackass":      "jerk",
	"smartass":     "smarty",
	"badass":       "tough",
	"bullshit":     "baloney",
	"horseshit":    "nonsense",
	"dipshit":      "dummy",
	"shithead":     "jerk",
	"dickhead":     "jerk",
	"prick":        "jerk",
	"douche":       "jerk",
	"douchebag":    "jerk",
}

// ProfanityFilter handles filtering and replacement of profanity
type ProfanityFilter struct {
	regexes map[string]*regexp.Regexp
}

// NewProfanityFilter creates a new profanity filter
func NewProfanityFilter() *ProfanityFilter {
	pf := &ProfanityFilter{
		regexes: make(map[string]*regexp.Regexp),
	}

	// Pre-compile regex patterns for each swear word
	for _, word := range swearWords {
		// Create a regex that matches the word with word boundaries and handles plurals
		pattern := `\b` + regexp.QuoteMeta(word)

		// Add optional 's' for pluralizable words (nouns, not phrases)
		if canBePluralized(word) {
			pattern += `s?` // Optional 's' for plural
		}

		pattern += `\b`
		pf.regexes[word] = regexp.MustCompile(`(?i)` + pattern)
	}

	return pf
}

// canBePluralized determines if a word can have an 's' added for plural form
func canBePluralized(word string) bool {
	// Words that are typically nouns and can be pluralized
	pluralizableWords := map[string]bool{
		"hell":      true,
		"damn":      true,
		"ass":       true,
		"bitch":     true,
		"bastard":   true,
		"dick":      true,
		"prick":     true,
		"douche":    true,
		"douchebag": true,
		"asshole":   true,
		"dumbass":   true,
		"jackass":   true,
		"smartass":  true,
		"badass":    true,
		"dipshit":   true,
		"shithead":  true,
		"dickhead":  true,
	}

	// Don't pluralize phrases, expletives, or words that don't make sense as plurals
	return pluralizableWords[word]
}

// FilterText replaces profanity in the input text based on content rating
func (pf *ProfanityFilter) FilterText(text string, contentRating string) string {
	rating := strings.ToUpper(strings.TrimSpace(contentRating))

	// No filtering for unknown/empty ratings or mature content
	if rating != "G" && rating != "PG" && rating != "PG13" && rating != "PG-13" {
		return text
	}

	result := text

	// Define words that are acceptable in PG13 content
	pg13AllowedWords := map[string]bool{
		"damn":     true,
		"hell":     true,
		"ass":      true,
		"crap":     true,
		"goddamn":  true,
		"christ":   true,
		"asshole":  true,
		"jackass":  true,
		"smartass": true,
		"badass":   true,
		"bastard":  true,
		"bullshit": true,
	}

	// Replace each swear word with its family-friendly alternative
	for _, word := range swearWords {
		// Skip filtering for PG13-allowed words if rating is PG13
		if (rating == "PG13" || rating == "PG-13") && pg13AllowedWords[word] {
			continue
		}

		if regex, exists := pf.regexes[word]; exists {
			if replacement, hasReplacement := swearWordReplacements[word]; hasReplacement {
				result = regex.ReplaceAllStringFunc(result, func(match string) string {
					return preserveCase(match, replacement)
				})
			}
		}
	}

	return result
}

// preserveCase applies the case pattern of the original word to the replacement
func preserveCase(original, replacement string) string {
	if len(original) == 0 {
		return replacement
	}

	// Check if the original word is plural (ends with 's')
	isPlural := strings.HasSuffix(strings.ToLower(original), "s")

	// If original is plural, make replacement plural too
	if isPlural {
		replacement = replacement + "s"
	}

	// All uppercase
	if strings.ToUpper(original) == original {
		return strings.ToUpper(replacement)
	}

	// All lowercase
	if strings.ToLower(original) == original {
		return strings.ToLower(replacement)
	}

	// Title case (first letter uppercase, rest lowercase)
	titleCaser := cases.Title(language.English)
	if titleCaser.String(strings.ToLower(original)) == original {
		return titleCaser.String(replacement)
	}

	// Mixed case - try to preserve the pattern character by character
	result := make([]rune, 0, len(replacement))
	originalRunes := []rune(original)
	replacementRunes := []rune(replacement)

	for i, r := range replacementRunes {
		if i < len(originalRunes) {
			// Apply the case of the corresponding character in the original
			if unicode.IsUpper(originalRunes[i]) {
				result = append(result, unicode.ToUpper(r))
			} else {
				result = append(result, unicode.ToLower(r))
			}
		} else {
			// If replacement is longer, use lowercase for extra characters
			result = append(result, unicode.ToLower(r))
		}
	}

	return string(result)
}

// ContainsProfanity checks if the text contains any profanity
func (pf *ProfanityFilter) ContainsProfanity(text string) bool {
	for _, word := range swearWords {
		if regex, exists := pf.regexes[word]; exists {
			if regex.MatchString(text) {
				return true
			}
		}
	}
	return false
}
