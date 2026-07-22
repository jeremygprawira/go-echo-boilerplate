package middleware

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldAllowCredentials_WildcardDisables(t *testing.T) {
	require.False(t, shouldAllowCredentials([]string{"*"}))
	require.False(t, shouldAllowCredentials([]string{"https://app.example.com", "*"}))
}

func TestShouldAllowCredentials_ExplicitEnables(t *testing.T) {
	require.True(t, shouldAllowCredentials([]string{"https://app.example.com"}))
}
