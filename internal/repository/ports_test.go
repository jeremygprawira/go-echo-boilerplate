package repository_test

import (
	"testing"

	"go-echo-boilerplate/internal/repository"
	"go-echo-boilerplate/internal/repository/pgsql"

	"github.com/stretchr/testify/require"
)

// The pgsql adapters must satisfy the storage-neutral ports.
func TestPgsqlSatisfiesPorts(t *testing.T) {
	var _ repository.UserRepository = (pgsql.UserRepository)(nil)
	var _ repository.HealthRepository = (pgsql.HealthRepository)(nil)
	require.True(t, true)
}
