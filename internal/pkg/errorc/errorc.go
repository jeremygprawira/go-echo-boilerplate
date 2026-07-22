// errorc (errorcustom) is a custom error helper package
package errorc

import (
	"errors"
	"fmt"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/stringc"
	"net/http"

	"github.com/labstack/echo/v4"
)

// httpStatusToCode converts an HTTP status code into a normalized status string.
// e.g. 404 → "NOT_FOUND", 500 → "INTERNAL_SERVER_ERROR".
// This is the single source of truth for status formatting used across the package.
func httpStatusToCode(code int) string {
	return stringc.TrimAndUpperCase(stringc.SnakeCase(http.StatusText(code)))
}

// ==== HTTPError Wrapper ====
type HTTPError struct {
	Response models.ErrorResponse
	Err      error // The raw, unsafe internal error
}

// Error strictly returns the SAFE public message. It acts as the firewall.
func (e *HTTPError) Error() string {
	return e.Response.Message
}

// Internal returns the raw, unsafe internal error for logging purposes.
func (e *HTTPError) Internal() error {
	return e.Err
}

// ==== Main Constructor ====

// Error wraps an error into an HTTPError.
// It accepts options to customize the behavior.
//
// Examples:
//  1. errorc.Error(errorc.ErrorDatabase, err) // Wrap an unsafe error with a safe public error
//  2. errorc.Error(errorc.ErrorDatabase, "Custom message") // Override public message
//  3. errorc.Error(errorc.ErrorDatabase, err, "Custom message %s", id) // Wrap unsafe AND override message
func Error(err error, args ...any) *HTTPError {
	var resp models.ErrorResponse
	var internalErr = err

	// Case 1: predefined models.ErrorResponse
	// models.ErrorResponse does not implement the error interface, so we must box
	// through any to perform the type assertion from the error parameter.
	if predefined, ok := any(err).(models.ErrorResponse); ok {
		resp = predefined
		internalErr = nil // Will be overridden if an error is also passed in args
	} else if predefinedPtr, ok := any(err).(*HTTPError); ok {
		// Case 1.5: already an HTTPError
		resp = predefinedPtr.Response
		internalErr = predefinedPtr.Err
	} else if echoErr, ok := err.(*echo.HTTPError); ok {
		// Case 2: echo.HTTPError
		resp = models.ErrorResponse{
			Code:    echoErr.Code,
			Status:  httpStatusToCode(echoErr.Code),
			Message: fmt.Sprintf("%v", echoErr.Message),
		}
		if echoErr.Internal != nil {
			internalErr = echoErr.Internal
		}
	} else {
		// Case 3: standard error (unsafe fallback)
		resp = models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Status:  "INTERNAL_SERVER_ERROR",
			Message: "Unknown server error occurred.",
		}
	}

	// Process variadic args
	var messageArgs []any
	for _, arg := range args {
		if e, ok := arg.(error); ok {
			internalErr = e // Found an unsafe internal error to attach
		} else {
			messageArgs = append(messageArgs, arg) // Found string/formatting args
		}
	}

	// Override message if formatting args were provided
	if len(messageArgs) > 0 {
		switch msg := messageArgs[0].(type) {
		case string:
			if len(messageArgs) == 1 {
				resp.Message = msg
			} else {
				resp.Message = fmt.Sprintf(msg, messageArgs[1:]...)
			}
		default:
			resp.Message = fmt.Sprint(messageArgs...)
		}
	}

	return &HTTPError{
		Response: resp,
		Err:      internalErr,
	}
}

// ==== Fallback Extractor ====

// GetResponse extracts the models.ErrorResponse from an error.
// If it's not an HTTPError, returns a default 500 INTERNAL_SERVER_ERROR.
func GetResponse(err error) models.ErrorResponse {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Response
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		return models.ErrorResponse{
			Code:    echoErr.Code,
			Status:  httpStatusToCode(echoErr.Code),
			Message: fmt.Sprintf("%v", echoErr.Message),
		}
	}

	return models.ErrorResponse{
		Code:    http.StatusInternalServerError,
		Status:  "INTERNAL_SERVER_ERROR",
		Message: "Unknown server error occurred.",
	}
}
