package validator_test

import (
	"strings"
	"testing"

	"go-echo-boilerplate/internal/pkg/validator"

	"github.com/stretchr/testify/require"
)

func TestPasswordWithinBcryptLimit(t *testing.T) {
	require.NoError(t, validator.PasswordWithinBcryptLimit(strings.Repeat("a", 72)))
	require.Error(t, validator.PasswordWithinBcryptLimit(strings.Repeat("a", 73)))
}
