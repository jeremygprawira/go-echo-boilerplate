package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAccountNumber(t *testing.T) {
	t.Run("Valid Account Numbers", func(t *testing.T) {
		validNumbers := []string{
			"4532015112830366", // Valid Luhn
			"6011514433546201", // Valid Luhn
			"5425233430109903", // Valid Luhn
		}

		for _, number := range validNumbers {
			assert.True(t, AccountNumber(number), "Should validate: %s", number)
		}
	})

	t.Run("Invalid Account Numbers - Wrong Check Digit", func(t *testing.T) {
		invalidNumbers := []string{
			"4532015112830367", // Wrong check digit
			"6011514433546202", // Wrong check digit
			"3714496353984313", // Wrong check digit
		}

		for _, number := range invalidNumbers {
			assert.False(t, AccountNumber(number), "Should reject: %s", number)
		}
	})

	t.Run("Invalid Account Numbers - Wrong Length", func(t *testing.T) {
		assert.False(t, AccountNumber("123"))
		assert.False(t, AccountNumber("12345678901234567")) // 17 digits
		assert.False(t, AccountNumber(""))
	})

	t.Run("Invalid Account Numbers - Contains Non-Digits", func(t *testing.T) {
		assert.False(t, AccountNumber("453201511283036a"))
		assert.False(t, AccountNumber("4532-0151-1283-0366"))
		assert.False(t, AccountNumber("4532 0151 1283 0366"))
	})
}

func BenchmarkValidateAccountNumber(b *testing.B) {
	accountNumber := "4532015112830366"
	for i := 0; i < b.N; i++ {
		_ = AccountNumber(accountNumber)
	}
}
