package http

import (
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/stringc"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jeremygprawira/herr"
	"github.com/jeremygprawira/wlog"
	"github.com/labstack/echo/v4"
)

// wireBody mirrors herr's unexported wireError shape so its safe, allow-listed
// fields can be read out of the `any` that Body() returns. herr guarantees
// nothing beyond these fields ever reaches the wire, so this struct is the
// complete set of data available to fold into models.Response.
type wireBody struct {
	Code       string                           `json:"code"`
	Title      string                           `json:"title,omitempty"`
	Message    string                           `json:"message,omitempty"`
	Retryable  *bool                            `json:"retryable,omitempty"`
	RetryAfter int                              `json:"retryAfter,omitempty"`
	TraceID    string                           `json:"traceId,omitempty"`
	Errors     []models.ErrorValidationResponse `json:"errors,omitempty"`
}

// decodeWireBody round-trips herr's safe wire body through JSON since its
// concrete type is unexported — this is the only way to read it from outside
// the herr package without losing the library's field allow-listing.
func decodeWireBody(herrErr *herr.Error) wireBody {
	var w wireBody
	if b, err := json.Marshal(herrErr.Body("")); err == nil {
		_ = json.Unmarshal(b, &w)
	}
	return w
}

// ErrorHandler is the single rendering point for every error returned by a
// handler or middleware. Registered as e.HTTPErrorHandler in core.Setup. It
// records the error on the wide event once, then folds herr's safe wire body
// into the same models.Response envelope success responses use, so Code is
// always the HTTP status (int) regardless of outcome and the catalog error
// identifier lives in ErrorCode instead.
func ErrorHandler(err error, ctx echo.Context) {
	herrErr := coerce(err)
	wlog.Error(ctx.Request().Context(), herrErr)
	if ctx.Response().Committed {
		return
	}

	requestID := ctx.Response().Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	body := decodeWireBody(herrErr)

	if body.RetryAfter > 0 {
		ctx.Response().Header().Set("Retry-After", strconv.Itoa(body.RetryAfter))
	}

	message := body.Message
	if message == "" {
		message = body.Title
	}

	// A nil []ErrorValidationResponse assigned directly into the Errors
	// interface{} field would still marshal as "errors":null (a typed-nil
	// interface is non-nil), so only assign when there's something to show.
	var responseErrors interface{}
	if len(body.Errors) > 0 {
		responseErrors = body.Errors
	}

	resp := models.Response{
		Code:      herrErr.HTTPStatus(),
		Status:    "ERROR",
		Message:   message,
		ErrorCode: body.Code,
		Errors:    responseErrors,
		Metadata: models.Metadata{
			RequestId:  requestID,
			Timestamp:  time.Now().Format(time.RFC3339),
			Retryable:  body.Retryable,
			RetryAfter: body.RetryAfter,
			TraceId:    body.TraceID,
		},
	}

	if writeErr := ctx.JSON(herrErr.HTTPStatus(), resp); writeErr != nil {
		wlog.Set(ctx.Request().Context(), "error_response_write_failed", writeErr.Error())
	}
}

// coerce normalizes any error into a *herr.Error without losing internal detail.
func coerce(err error) *herr.Error {
	var he *herr.Error
	if errors.As(err, &he) {
		return he
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		var e *herr.Error
		if class := classForStatus(echoErr.Code); class != nil {
			e = class.New()
		} else {
			statusText := http.StatusText(echoErr.Code)
			e = herr.New(stringc.TrimAndUpperCase(stringc.SnakeCase(statusText))).
				Status(echoErr.Code).
				Public(herr.Message(statusText))
			if echoErr.Code < http.StatusInternalServerError {
				e = e.Kind(herr.KindInvalid)
			}
		}
		e = e.Internalf("framework: %v", echoErr.Message)
		if echoErr.Internal != nil {
			e = e.Wrap(echoErr.Internal)
		}
		return e
	}

	return errorc.Internal.New().Wrap(err)
}

// classForStatus maps well-known HTTP statuses to catalog classes so
// framework errors (404 route miss, echo bind failures, ...) reuse stable codes.
func classForStatus(status int) *herr.Class {
	switch status {
	case http.StatusBadRequest:
		return errorc.InvalidInput
	case http.StatusUnauthorized:
		return errorc.Unauthorized
	case http.StatusForbidden:
		return errorc.Forbidden
	case http.StatusNotFound:
		return errorc.DataNotFound
	case http.StatusConflict:
		return errorc.AlreadyExists
	case http.StatusUnprocessableEntity:
		return errorc.Validation
	case http.StatusTooManyRequests:
		return errorc.TooManyRequests
	default:
		return nil
	}
}
