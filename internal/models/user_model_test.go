package models_test

import (
	"encoding/json"
	"testing"

	"go-echo-boilerplate/internal/models"

	"github.com/stretchr/testify/require"
)

func TestUser_JSONOmitsPasswordHash(t *testing.T) {
	body, err := json.Marshal(models.User{ID: 1, Password: "$2a$12$hash"})
	require.NoError(t, err)
	require.NotContains(t, string(body), "hash")
	require.NotContains(t, string(body), "password")
}
