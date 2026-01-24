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

func Success(ctx echo.Context, code int, data interface{}, message ...string) error {
	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	timestamp, _ := ctx.Get("X-Timestamp").(string)
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	var finalMessage string
	switch len(message) {
	case 0:
		finalMessage = "Request has been successfully processed."
	case 1:
		finalMessage = message[0]
	default:
		finalMessage = fmt.Sprintf(message[0], stringc.SlicesToInterfaces(message[1:])...)
	}

	return ctx.JSON(code, models.Response{
		Code:    code,
		Status:  "OK",
		Message: finalMessage,
		Data:    data,
		Metadata: models.Metadata{
			RequestId: requestID,
			Timestamp: timestamp,
		},
	})
}

func SuccessList(ctx echo.Context, code int, message string, data interface{}) error {
	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	timestamp, _ := ctx.Get("X-Timestamp").(string)
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	if message == "" {
		message = "Request has been successfully processed."
	}

	length, err := numberc.LengthOf(data)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, models.Response{
			Code:    http.StatusInternalServerError,
			Status:  "ERROR",
			Message: "Failed to get length of data.",
			Metadata: models.Metadata{
				RequestId: requestID,
				Timestamp: timestamp,
			},
		})
	}

	return ctx.JSON(http.StatusOK, models.Response{
		Code:    code,
		Status:  "OK",
		Message: message,
		Data:    data,
		Metadata: models.Metadata{
			RequestId: requestID,
			Timestamp: timestamp,
			TotalRows: length,
		},
	})
}

func SuccessPagination(ctx echo.Context, code int, message string, pagination models.PaginationOutput, data interface{}) error {
	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	timestamp, _ := ctx.Get("X-Timestamp").(string)
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	if message == "" {
		message = "Request has been successfully processed."
	}

	return ctx.JSON(http.StatusOK, models.Response{
		Code:       code,
		Status:     "OK",
		Message:    message,
		Data:       data,
		Pagination: &pagination,
		Metadata: models.Metadata{
			RequestId: requestID,
			Timestamp: timestamp,
		},
	})
}
