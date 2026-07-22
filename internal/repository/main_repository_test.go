package repository_test

import (
	"testing"

	"go-echo-boilerplate/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestRepositoryExposesPorts(t *testing.T) {
	// Zero-value struct: fields must be the port types, not a nested Postgre struct.
	var r repository.Repository
	require.Nil(t, r.User)
	require.Nil(t, r.Health)
}
