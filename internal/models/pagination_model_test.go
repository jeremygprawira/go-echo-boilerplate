package models_test

import (
	"testing"

	"go-echo-boilerplate/internal/models"

	"github.com/stretchr/testify/require"
)

func TestParsePagination_Defaults(t *testing.T) {
	p := models.ParsePagination("", "")
	require.Equal(t, 20, p.Limit)
	require.Equal(t, 0, p.Offset)
}

func TestParsePagination_CapsLimit(t *testing.T) {
	p := models.ParsePagination("500", "40")
	require.Equal(t, 100, p.Limit)
	require.Equal(t, 40, p.Offset)
}

func TestParsePagination_IgnoresGarbage(t *testing.T) {
	p := models.ParsePagination("abc", "-5")
	require.Equal(t, 20, p.Limit)
	require.Equal(t, 0, p.Offset)
}
