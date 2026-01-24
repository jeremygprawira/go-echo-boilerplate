package generator

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
)

// AccountNumberLength defines the total length of the account number including check digit
const AccountNumberLength = 16

// GenerateAccountNumber generates a random account number with Luhn check digit
// The format is similar to credit card numbers (16 digits with Luhn validation)
//
// Parameters:
//   - affix: Optional variadic parameters for prefix and/or suffix
//   - affix[0]: Prefix - digits to prepend to the account number
//   - affix[1]: Suffix - digits to append before the check digit
//
// Examples:
//   - GenerateAccountNumber() -> "4532015112830366" (fully random)
//   - GenerateAccountNumber("99") -> "9932015112830366" (with prefix "99")
//   - GenerateAccountNumber("99", "88") -> "9932015112883066" (with prefix "99" and suffix "88")
//   - GenerateAccountNumber("", "88") -> "4532015112883088" (with suffix "88" only)
//
// Returns:
//   - string: A valid account number with Luhn check digit
//   - error: If affix is invalid or random number generation fails
func AccountNumber(affix ...string) (string, error) {
	var prefix, suffix string

	// Extract prefix and suffix from affix
	if len(affix) > 0 {
		prefix = affix[0]
	}
	if len(affix) > 1 {
		suffix = affix[1]
	}

	// Validate total length
	if len(prefix)+len(suffix) >= AccountNumberLength {
		return "", fmt.Errorf("combined prefix and suffix length must be less than %d", AccountNumberLength)
	}

	// Validate prefix contains only digits
	for _, char := range prefix {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("prefix must contain only digits")
		}
	}

	// Validate suffix contains only digits
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("suffix must contain only digits")
		}
	}

	// Convert prefix to digits
	digits := make([]int, 0, AccountNumberLength)
	for _, char := range prefix {
		digits = append(digits, int(char-'0'))
	}

	// Calculate how many random digits we need (excluding check digit and suffix)
	randomLength := AccountNumberLength - len(prefix) - len(suffix) - 1

	// Generate random digits
	for i := 0; i < randomLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("failed to generate random digit: %w", err)
		}
		digits = append(digits, int(n.Int64()))
	}

	// Append suffix digits (before check digit)
	for _, char := range suffix {
		digits = append(digits, int(char-'0'))
	}

	// Calculate and append Luhn check digit
	checkDigit := calculateLuhnCheckDigit(digits)
	digits = append(digits, checkDigit)

	// Convert to string
	accountNumber := ""
	for _, digit := range digits {
		accountNumber += strconv.Itoa(digit)
	}

	return accountNumber, nil
}

// calculateLuhnCheckDigit calculates the Luhn check digit for a sequence of digits
// The input should NOT include the check digit
func calculateLuhnCheckDigit(digits []int) int {
	sum := 0

	// Process digits from right to left (excluding the check digit position)
	// Since we're calculating the check digit, we start from the rightmost position
	for i := len(digits) - 1; i >= 0; i-- {
		digit := digits[i]

		// Double every second digit from the right
		// Position 0 from right will be the check digit (not included in input)
		// So position 1 from right (index len-1) should be doubled
		position := len(digits) - i
		if position%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
	}

	// Calculate check digit: (10 - (sum mod 10)) mod 10
	checkDigit := (10 - (sum % 10)) % 10
	return checkDigit
}
