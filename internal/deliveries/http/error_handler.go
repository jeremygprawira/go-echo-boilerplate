package http

import (
	"errors"
	"go-echo-boilerplate/internal/pkg/apperr"
	"go-echo-boilerplate/internal/pkg/logger"
	"go-echo-boilerplate/internal/pkg/stringc"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jeremygprawira/herr"
	"github.com/labstack/echo/v4"
)

// ErrorHandler is the single rendering point for every error returned by a
// handler or middleware. Registered as e.HTTPErrorHandler in core.Setup.
// It renders herr's safe wire body and enriches the wide-event log once.
func ErrorHandler(err error, ctx echo.Context) {
	if ctx.Response().Committed {
		return
	}

	herrErr := coerce(err)

	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}
	herrErr = herrErr.WithPublic("requestId", requestID)

	record := herr.LogRecord(herrErr)
	errType := "AppError"
	if record.HTTPStatus >= http.StatusInternalServerError {
		errType = "InternalError"
	}
	logger.AddError(ctx.Request().Context(), &logger.ErrorContext{
		Type:    errType,
		Code:    record.Code,
		Message: herrErr.Error(), // code + internal detail + cause; logs are trusted
		Stack:   record.Stack,
	})

	if seconds := herrErr.RetryAfterSeconds(); seconds > 0 {
		ctx.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	}

	if writeErr := ctx.JSON(herrErr.HTTPStatus(), herrErr.Body("")); writeErr != nil && logger.Instance != nil {
		logger.Instance.Error(ctx.Request().Context(), "failed to write error response", logger.Error(writeErr))
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
				Public(herr.Msg(statusText))
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

	return apperr.Internal.New().Wrap(err)
}

// classForStatus maps well-known HTTP statuses to catalog classes so
// framework errors (404 route miss, echo bind failures, ...) reuse stable codes.
func classForStatus(status int) *herr.Class {
	switch status {
	case http.StatusBadRequest:
		return apperr.InvalidInput
	case http.StatusUnauthorized:
		return apperr.Unauthorized
	case http.StatusForbidden:
		return apperr.Forbidden
	case http.StatusNotFound:
		return apperr.DataNotFound
	case http.StatusConflict:
		return apperr.AlreadyExists
	case http.StatusUnprocessableEntity:
		return apperr.Validation
	default:
		return nil
	}
}
