// stringc (stringcustom) is a custom string helper package
package stringc

import "unicode"

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
