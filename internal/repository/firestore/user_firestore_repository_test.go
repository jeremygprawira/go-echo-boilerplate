package firestore_test

import (
	"testing"

	"go-echo-boilerplate/internal/repository"
	fsrepo "go-echo-boilerplate/internal/repository/firestore"

	"github.com/stretchr/testify/require"
)

func TestFirestoreSatisfiesUserPort(t *testing.T) {
	var _ repository.UserRepository = (*fsrepo.UserRepository)(nil)
	require.True(t, true)
}
