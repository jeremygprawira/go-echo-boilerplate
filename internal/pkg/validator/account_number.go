package validator

// AccountNumberLength defines the total length of the account number including check digit
const AccountNumberLength = 16

// ValidateAccountNumber validates an account number using the Luhn algorithm
//
// Parameters:
//   - accountNumber: The account number to validate
//
// Returns:
//   - bool: true if valid, false otherwise
func AccountNumber(accountNumber string) bool {
	if len(accountNumber) != AccountNumberLength {
		return false
	}

	// Convert to digits
	digits := make([]int, len(accountNumber))
	for i, char := range accountNumber {
		if char < '0' || char > '9' {
			return false
		}
		digits[i] = int(char - '0')
	}

	// Validate using Luhn algorithm
	return Luhn(digits)
}

// Luhn validates a complete number (including check digit) using Luhn algorithm
func Luhn(digits []int) bool {
	sum := 0

	// Process all digits from right to left
	for i := len(digits) - 1; i >= 0; i-- {
		digit := digits[i]

		// Double every second digit from the right (starting from the second-to-last)
		position := len(digits) - i
		if position%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
	}

	// Valid if sum is divisible by 10
	return sum%10 == 0
}
