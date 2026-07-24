package apperr

import (
	"errors"
	"go-echo-boilerplate/internal/models"

	"github.com/hashicorp/go-multierror"
)

// FromValidation converts the *multierror.Error produced by validator.Input
// into a single 422 herr error carrying typed field errors. Non-multierror
// input keeps its detail on the internal (log-only) surface.
func FromValidation(err error) error {
	if err == nil {
		return nil
	}

	e := Validation.New()

	var merr *multierror.Error
	if !errors.As(err, &merr) {
		return e.Internal(err.Error())
	}

	for _, entry := range merr.Errors {
		var ve models.ErrorValidationResponse
		if errors.As(entry, &ve) {
			e = e.FieldError(ve.Field, ve.Code, ve.Message)
		} else {
			e = e.FieldError("", "INVALID", entry.Error())
		}
	}

	return e
}
