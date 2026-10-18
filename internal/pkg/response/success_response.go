package response

import (
	"fmt"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/numberc"
	"go-echo-boilerplate/internal/pkg/stringc"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const defaultMessage = "Request has been successfully processed."

// Metadata is the envelope's metadata block: the X-Request-Id response header
// (a fresh UUID when it is unset) and the current time in RFC3339.
func Metadata(ctx echo.Context) models.Metadata {
	requestID := ctx.Response().Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		requestID = uuid.New().String()
	}
	return models.Metadata{RequestId: requestID, Timestamp: time.Now().Format(time.RFC3339)}
}

func Success(ctx echo.Context, code int, data interface{}, message ...string) error {
	var finalMessage string
	switch len(message) {
	case 0:
		finalMessage = defaultMessage
	case 1:
		finalMessage = message[0]
	default:
		finalMessage = fmt.Sprintf(message[0], stringc.SlicesToInterfaces(message[1:])...)
	}

	return ctx.JSON(code, models.Response{
		Code:     code,
		Status:   "OK",
		Message:  finalMessage,
		Data:     data,
		Metadata: Metadata(ctx),
	})
}

func SuccessList(ctx echo.Context, code int, message string, data interface{}) error {
	if message == "" {
		message = defaultMessage
	}

	metadata := Metadata(ctx)
	length, err := numberc.LengthOf(data)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, models.Response{
			Code:     http.StatusInternalServerError,
			Status:   "ERROR",
			Message:  "Failed to get length of data.",
			Metadata: metadata,
		})
	}

	status := "OK"
	if code >= http.StatusBadRequest {
		status = "ERROR"
	}
	metadata.TotalRows = length
	return ctx.JSON(code, models.Response{
		Code:     code,
		Status:   status,
		Message:  message,
		Data:     data,
		Metadata: metadata,
	})
}

// SuccessPagination writes a list response with cursor pagination.
func SuccessPagination(ctx echo.Context, code int, message string, pagination models.PaginationOutput, data interface{}) error {
	if message == "" {
		message = defaultMessage
	}

	return ctx.JSON(code, models.Response{
		Code:       code,
		Status:     "OK",
		Message:    message,
		Data:       data,
		Pagination: &pagination,
		Metadata:   Metadata(ctx),
	})
}
