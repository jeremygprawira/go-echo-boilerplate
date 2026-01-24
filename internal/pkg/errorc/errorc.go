// errorc (errorcustom) is a custom error helper package
package errorc

import (
	"errors"
	"fmt"
	"go-echo-boilerplate/internal/models"
	"net/http"
)

// ==== HTTPError Wrapper ====
type HTTPError struct {
	Response models.ErrorResponse
	Err      error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Response.Message
}

// ==== Main Constructor ====

// Error wraps an error into an HTTPError.
//   - If `err` is a predefined models.ErrorResponse, it uses its values.
//   - If `err` is a regular error, it creates a default 500 Internal Server Error response.
//   - `message` is optional; if provided, it overrides the default response message.
//     You can pass a string or a formatted string with args (like fmt.Sprintf).
func Error(err error, message ...any) *HTTPError {
	var resp models.ErrorResponse
	var internalErr = err

	// Case 1: predefined models.ErrorResponse
	if predefined, ok := any(err).(models.ErrorResponse); ok {
		resp = predefined
	} else if httpErr, ok := err.(*HTTPError); ok {
		// Case 1.5: already an HTTPError, use its response
		resp = httpErr.Response
		internalErr = httpErr.Err
	} else {
		// Case 2: standard error
		resp = models.ErrorResponse{
			Code:    http.StatusInternalServerError,
			Status:  "INTERNAL_SERVER_ERROR",
			Message: err.Error(),
		}
	}

	// Case 3: custom override or format
	if len(message) > 0 {
		switch msg := message[0].(type) {
		case string:
			if len(message) == 1 {
				resp.Message = msg
			} else {
				resp.Message = fmt.Sprintf(msg, message[1:]...)
			}
		default:
			resp.Message = fmt.Sprint(message...)
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

	return models.ErrorResponse{
		Code:    http.StatusInternalServerError,
		Status:  "INTERNAL_SERVER_ERROR",
		Message: err.Error(),
	}
}
