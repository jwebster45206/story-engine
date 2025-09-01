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
	"fuck", "shit", "damn", "hell", "ass", "bitch", "bastard", "crap",
	"piss", "cock", "dick", "pussy", "tits", "boobs", "whore", "slut",
	"fag", "retard", "nigger", "nigga", "spic", "chink", "kike",
	"motherfucker", "goddamn", "jesus christ", "christ", "asshole",
	"dumbass", "jackass", "smartass", "badass", "bullshit", "horseshit",
	"dipshit", "shithead", "dickhead", "prick", "douche", "douchebag",
}

// swearWordReplacements maps swear words to family-friendly alternatives
var swearWordReplacements = map[string]string{
	"fuck":         "fudge",
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
		// Create a regex that matches the word with word boundaries and handles basic variations
		pattern := `\b` + regexp.QuoteMeta(word) + `\b`
		pf.regexes[word] = regexp.MustCompile(`(?i)` + pattern)
	}

	return pf
}

// FilterText replaces profanity in the input text with family-friendly alternatives
func (pf *ProfanityFilter) FilterText(text string) string {
	result := text

	// Replace each swear word with its family-friendly alternative
	for _, word := range swearWords {
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

// ShouldFilterContent determines if content should be filtered based on rating
func ShouldFilterContent(rating string) bool {
	rating = strings.ToUpper(strings.TrimSpace(rating))
	switch rating {
	case "G", "PG", "PG13", "PG-13":
		return true
	default:
		return false
	}
}
