package models_test

import (
	"testing"

	"go-echo-boilerplate/internal/models"

	"github.com/stretchr/testify/require"
)

func TestParsePagination_Defaults(t *testing.T) {
	p := models.ParsePagination("", "")
	require.Equal(t, 20, p.Limit)
	require.Equal(t, "", p.Cursor)
}

func TestParsePagination_CapsLimit(t *testing.T) {
	p := models.ParsePagination("42", "500")
	require.Equal(t, 100, p.Limit)
	require.Equal(t, "42", p.Cursor)
}

func TestParsePagination_IgnoresGarbageLimit(t *testing.T) {
	p := models.ParsePagination("42", "abc")
	require.Equal(t, 20, p.Limit)
	require.Equal(t, "42", p.Cursor)
}

func TestParsePagination_IgnoresNonPositiveLimit(t *testing.T) {
	require.Equal(t, 20, models.ParsePagination("", "0").Limit)
	require.Equal(t, 20, models.ParsePagination("", "-5").Limit)
}

func TestParsePagination_KeepsLimitWithinBounds(t *testing.T) {
	require.Equal(t, 1, models.ParsePagination("", "1").Limit)
	require.Equal(t, 100, models.ParsePagination("", "100").Limit)
}
