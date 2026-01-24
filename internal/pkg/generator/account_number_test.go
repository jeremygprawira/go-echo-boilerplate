package generator

import (
	"go-echo-boilerplate/internal/pkg/validator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAccountNumber(t *testing.T) {
	t.Run("Generate Valid Account Number", func(t *testing.T) {
		accountNumber, err := AccountNumber()
		assert.NoError(t, err)
		assert.NotEmpty(t, accountNumber)
		assert.Len(t, accountNumber, AccountNumberLength)

		// Validate the generated account number
		assert.True(t, validator.AccountNumber(accountNumber), "Generated account number should be valid")
	})

	t.Run("Generate Multiple Unique Account Numbers", func(t *testing.T) {
		numbers := make(map[string]bool)

		// Generate 100 account numbers
		for i := 0; i < 100; i++ {
			accountNumber, err := AccountNumber()
			assert.NoError(t, err)

			// Check uniqueness (very high probability)
			assert.False(t, numbers[accountNumber], "Account numbers should be unique")
			numbers[accountNumber] = true

			// Validate each number
			assert.True(t, validator.AccountNumber(accountNumber))
		}
	})
}

func TestGenerateAccountNumberWithPrefix(t *testing.T) {
	t.Run("Generate With Valid Prefix", func(t *testing.T) {
		prefix := "45"
		accountNumber, err := AccountNumber(prefix)
		assert.NoError(t, err)
		assert.NotEmpty(t, accountNumber)
		assert.Len(t, accountNumber, AccountNumberLength)

		// Check prefix
		assert.Equal(t, prefix, accountNumber[:len(prefix)])

		// Validate the generated account number
		assert.True(t, validator.AccountNumber(accountNumber))
	})

	t.Run("Generate With Longer Prefix", func(t *testing.T) {
		prefix := "453201"
		accountNumber, err := AccountNumber(prefix)
		assert.NoError(t, err)
		assert.Equal(t, prefix, accountNumber[:len(prefix)])
		assert.True(t, validator.AccountNumber(accountNumber))
	})

	t.Run("Generate With Suffix", func(t *testing.T) {
		suffix := "88"
		accountNumber, err := AccountNumber("", suffix)
		assert.NoError(t, err)
		assert.Len(t, accountNumber, AccountNumberLength)

		// Check suffix (before check digit, which is the last digit)
		suffixStart := AccountNumberLength - len(suffix) - 1
		assert.Equal(t, suffix, accountNumber[suffixStart:AccountNumberLength-1])

		// Validate the generated account number
		assert.True(t, validator.AccountNumber(accountNumber))
	})

	t.Run("Generate With Prefix And Suffix", func(t *testing.T) {
		prefix := "99"
		suffix := "88"
		accountNumber, err := AccountNumber(prefix, suffix)
		assert.NoError(t, err)
		assert.Len(t, accountNumber, AccountNumberLength)

		// Check prefix
		assert.Equal(t, prefix, accountNumber[:len(prefix)])

		// Check suffix (before check digit)
		suffixStart := AccountNumberLength - len(suffix) - 1
		assert.Equal(t, suffix, accountNumber[suffixStart:AccountNumberLength-1])

		// Validate
		assert.True(t, validator.AccountNumber(accountNumber))
	})

	t.Run("Invalid Prefix - Too Long", func(t *testing.T) {
		prefix := "12345678901234567" // 17 digits
		accountNumber, err := AccountNumber(prefix)
		assert.Error(t, err)
		assert.Empty(t, accountNumber)
		assert.Contains(t, err.Error(), "combined prefix and suffix length must be less than")
	})

	t.Run("Invalid Prefix - Contains Non-Digits", func(t *testing.T) {
		prefix := "45a2"
		accountNumber, err := AccountNumber(prefix)
		assert.Error(t, err)
		assert.Empty(t, accountNumber)
		assert.Contains(t, err.Error(), "prefix must contain only digits")
	})

	t.Run("Invalid Suffix - Contains Non-Digits", func(t *testing.T) {
		suffix := "88x"
		accountNumber, err := AccountNumber("", suffix)
		assert.Error(t, err)
		assert.Empty(t, accountNumber)
		assert.Contains(t, err.Error(), "suffix must contain only digits")
	})

	t.Run("Invalid Prefix And Suffix - Too Long Combined", func(t *testing.T) {
		prefix := "123456789"
		suffix := "123456789"
		accountNumber, err := AccountNumber(prefix, suffix)
		assert.Error(t, err)
		assert.Empty(t, accountNumber)
		assert.Contains(t, err.Error(), "combined prefix and suffix length must be less than")
	})

	t.Run("Generate Multiple With Same Prefix", func(t *testing.T) {
		prefix := "99"
		numbers := make(map[string]bool)

		for i := 0; i < 50; i++ {
			accountNumber, err := AccountNumber(prefix)
			assert.NoError(t, err)
			assert.Equal(t, prefix, accountNumber[:len(prefix)])

			// Check uniqueness
			assert.False(t, numbers[accountNumber])
			numbers[accountNumber] = true

			// Validate
			assert.True(t, validator.AccountNumber(accountNumber))
		}
	})
}

func TestCalculateLuhnCheckDigit(t *testing.T) {
	t.Run("Calculate Check Digit", func(t *testing.T) {
		testCases := []struct {
			digits      []int
			expectedCD  int
			description string
		}{
			{
				digits:      []int{4, 5, 3, 2, 0, 1, 5, 1, 1, 2, 8, 3, 0, 3, 6},
				expectedCD:  6,
				description: "Credit card example 1",
			},
			{
				digits:      []int{6, 0, 1, 1, 5, 1, 4, 4, 3, 3, 5, 4, 6, 2, 0},
				expectedCD:  1,
				description: "Credit card example 2",
			},
		}

		for _, tc := range testCases {
			checkDigit := calculateLuhnCheckDigit(tc.digits)
			assert.Equal(t, tc.expectedCD, checkDigit, tc.description)
		}
	})
}

func TestValidateLuhn(t *testing.T) {
	t.Run("Validate Complete Numbers", func(t *testing.T) {
		testCases := []struct {
			digits   []int
			expected bool
			name     string
		}{
			{
				digits:   []int{4, 5, 3, 2, 0, 1, 5, 1, 1, 2, 8, 3, 0, 3, 6, 6},
				expected: true,
				name:     "Valid number 1",
			},
			{
				digits:   []int{6, 0, 1, 1, 5, 1, 4, 4, 3, 3, 5, 4, 6, 2, 0, 1},
				expected: true,
				name:     "Valid number 2",
			},
			{
				digits:   []int{4, 5, 3, 2, 0, 1, 5, 1, 1, 2, 8, 3, 0, 3, 6, 7},
				expected: false,
				name:     "Invalid check digit",
			},
		}

		for _, tc := range testCases {
			result := validator.Luhn(tc.digits)
			assert.Equal(t, tc.expected, result, tc.name)
		}
	})
}

// Benchmark tests
func BenchmarkGenerateAccountNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = AccountNumber()
	}
}

func BenchmarkGenerateAccountNumberWithPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = AccountNumber("45")
	}
}
