package apperr_test

import (
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/apperr"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/jeremygprawira/herr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromValidation(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, apperr.FromValidation(nil))
	})

	t.Run("multierror maps to typed field errors", func(t *testing.T) {
		var merr *multierror.Error
		merr = multierror.Append(merr,
			models.ErrorValidationResponse{Code: "INVALID_EMAIL", Field: "email", Message: "Enter a valid email address."},
			models.ErrorValidationResponse{Code: "REQUIRED", Field: "password", Message: "This field is required."},
		)

		err := apperr.FromValidation(merr)
		require.Error(t, err)
		assert.True(t, apperr.Validation.Is(err))

		var he *herr.Error
		require.True(t, errors.As(err, &he))
		assert.Equal(t, 422, he.HTTPStatus())

		body, jsonErr := json.Marshal(he)
		require.NoError(t, jsonErr)
		var wire struct {
			Code   string `json:"code"`
			Errors []struct {
				Field   string `json:"field"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(body, &wire))
		assert.Equal(t, "VALIDATION_FAILED", wire.Code)
		require.Len(t, wire.Errors, 2)
		assert.Equal(t, "email", wire.Errors[0].Field)
		assert.Equal(t, "INVALID_EMAIL", wire.Errors[0].Code)
		assert.Equal(t, "Enter a valid email address.", wire.Errors[0].Message)
		assert.Equal(t, "password", wire.Errors[1].Field)
	})

	t.Run("non-multierror stays a 422 with internal detail only", func(t *testing.T) {
		err := apperr.FromValidation(errors.New("reflect: nil interface"))
		require.Error(t, err)
		assert.True(t, apperr.Validation.Is(err))
		body, jsonErr := json.Marshal(err)
		require.NoError(t, jsonErr)
		assert.NotContains(t, string(body), "reflect:")
	})
}
