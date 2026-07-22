package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validCfg() *Configuration {
	c := &Configuration{}
	c.Authorization.Access = TokenConfiguration{Secret: "access-secret", Duration: "15m"}
	c.Authorization.Refresh = TokenConfiguration{Secret: "refresh-secret", Duration: "168h"}
	c.Authorization.APIKey = "an-api-key"
	return c
}

func TestValidate_OK(t *testing.T) {
	require.NoError(t, validCfg().Validate())
}

func TestValidate_EmptyAccessSecret(t *testing.T) {
	c := validCfg()
	c.Authorization.Access.Secret = ""
	require.Error(t, c.Validate())
}

func TestValidate_DuplicateSecrets(t *testing.T) {
	c := validCfg()
	c.Authorization.Refresh.Secret = c.Authorization.Access.Secret
	require.Error(t, c.Validate())
}

func TestValidate_EmptyAPIKey(t *testing.T) {
	c := validCfg()
	c.Authorization.APIKey = ""
	require.Error(t, c.Validate())
}

func TestValidate_BadDuration(t *testing.T) {
	c := validCfg()
	c.Authorization.Access.Duration = "not-a-duration"
	require.Error(t, c.Validate())
}
