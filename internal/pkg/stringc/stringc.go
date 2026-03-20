// stringc (stringcustom) is a custom string helper package
package stringc

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ContainsAlphabet checks if a string contains any alphabet characters
// Example:
//
//	stringc.ContainsAlphabet("abc123") // returns true
//	stringc.ContainsAlphabet("123")    // returns false
func ContainsAlphabet(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// SlicesToInterfaces converts a slice of strings to a slice of interfaces
// Example:
//
//	stringc.SlicesToInterfaces([]string{"abc", "def"}) // returns []interface{}{"abc", "def"}
func SlicesToInterfaces(args []string) []interface{} {
	result := make([]interface{}, len(args))
	for i, v := range args {
		result[i] = v
	}
	return result
}

// SnakeCase converts a string (CamelCase, PascalCase, or space-separated) to snake_case.
func SnakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 5) // Pre-allocate with some extra space for underscores

	var lastWasUpper, lastWasUnderscore bool

	for i, r := range s {
		// Treat spaces, dashes, and underscores as delimiters
		if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !lastWasUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastWasUnderscore = true
			}
			continue
		}

		isUpper := unicode.IsUpper(r)
		if isUpper {
			// Add underscore if we are transitioning from lower to upper,
			// or if we are in a block of uppers but the next character is lower (e.g. HTTPRequest -> http_request)
			if b.Len() > 0 && !lastWasUnderscore {
				// Use utf8.DecodeRuneInString to safely peek at the next rune;
				// s[i+1] would index bytes and break on multi-byte UTF-8 sequences.
				nextR, _ := utf8.DecodeRuneInString(s[i+utf8.RuneLen(r):])
				if !lastWasUpper || unicode.IsLower(nextR) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			lastWasUpper = true
			lastWasUnderscore = false
		} else {
			b.WriteRune(r)
			lastWasUpper = false
			lastWasUnderscore = false
		}
	}

	// Trim trailing underscore if present
	return strings.TrimSuffix(b.String(), "_")
}

// TrimAndUpperCase trims surrounding whitespace and converts s to UPPER CASE.
func TrimAndUpperCase(s string) string {
	return strings.TrimSpace(strings.ToUpper(s))
}

// TrimAndLowerCase trims surrounding whitespace and converts s to lower case.
func TrimAndLowerCase(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
