package generator_test

import (
	"encoding/base64"
	"testing"

	"go-echo-boilerplate/internal/pkg/generator"

	"github.com/stretchr/testify/assert"
)

func TestToken(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		wantErr   bool
		errString string
	}{
		{
			name:    "Valid length 16",
			length:  16,
			wantErr: false,
		},
		{
			name:    "Valid length 32",
			length:  32,
			wantErr: false,
		},
		{
			name:    "Valid length 64",
			length:  64,
			wantErr: false,
		},
		{
			name:    "Valid length 256",
			length:  256,
			wantErr: false,
		},
		{
			name:      "Invalid length 15",
			length:    15,
			wantErr:   true,
			errString: "token length must be at least 16 bytes",
		},
		{
			name:      "Invalid length 257",
			length:    257,
			wantErr:   true,
			errString: "token length must be at most 256 bytes",
		},
		{
			name:      "Invalid length 0",
			length:    0,
			wantErr:   true,
			errString: "token length must be at least 16 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generator.Token(tt.length)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, got)

				// Check that it is valid base64 URL encoded
				decoded, err := base64.URLEncoding.DecodeString(got)
				assert.NoError(t, err, "generated token should be valid base64 url encoded")
				assert.Equal(t, tt.length, len(decoded), "decoded length should match requested byte length")
			}
		})
	}
}

func TestTokenUniqueness(t *testing.T) {
	// Generate a few tokens and make sure they are unique
	count := 100
	tokens := make(map[string]bool)
	for i := 0; i < count; i++ {
		token, err := generator.Token(16)
		assert.NoError(t, err)
		assert.False(t, tokens[token], "token collision detected")
		tokens[token] = true
	}
}

func TestSessionToken(t *testing.T) {
	token, err := generator.SessionToken()
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Session token is 32 bytes
	decoded, err := base64.URLEncoding.DecodeString(token)
	assert.NoError(t, err)
	assert.Equal(t, 32, len(decoded))
}

func TestAPIKey(t *testing.T) {
	key, err := generator.APIKey()
	assert.NoError(t, err)
	assert.NotEmpty(t, key)

	// API key is 64 bytes
	decoded, err := base64.URLEncoding.DecodeString(key)
	assert.NoError(t, err)
	assert.Equal(t, 64, len(decoded))
}
