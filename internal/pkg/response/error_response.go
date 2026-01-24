package response

import (
	"fmt"
	"go-echo-boilerplate/internal/models"
	"go-echo-boilerplate/internal/pkg/errorc"
	"go-echo-boilerplate/internal/pkg/stringc"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/labstack/echo/v4"
)

func Error(ctx echo.Context, err error, message ...string) error {
	var response models.Response

	if err == nil {
		errorcResponse := errorc.GetResponse(errorc.ErrorInternalServer)
		response = models.Response{
			Code:    errorcResponse.Code,
			Status:  errorcResponse.Status,
			Message: errorcResponse.Message,
		}
	} else {
		errorcResponse := errorc.GetResponse(err)

		var finalMessage string
		switch len(message) {
		case 0:
			finalMessage = errorcResponse.Message
		case 1:
			finalMessage = message[0]
		default:
			finalMessage = fmt.Sprintf(message[0], stringc.SlicesToInterfaces(message[1:])...)
		}

		response = models.Response{
			Code:    errorcResponse.Code,
			Status:  errorcResponse.Status,
			Message: finalMessage,
		}
	}

	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	timestamp, _ := ctx.Get("X-Timestamp").(string)
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	response.Metadata = models.Metadata{
		RequestId: requestID,
		Timestamp: timestamp,
	}

	return ctx.JSON(response.Code, response)
}

func ErrorValidation(ctx echo.Context, errors interface{}) error {
	requestID, _ := ctx.Get("X-Request-ID").(string)
	if requestID == "" {
		requestID = uuid.New().String()
	}

	timestamp, _ := ctx.Get("X-Timestamp").(string)
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	errorcResponse := errorc.GetResponse(errorc.ErrorValidation)
	response := models.Response{
		Code:    errorcResponse.Code,
		Status:  errorcResponse.Status,
		Message: errorcResponse.Message,
		Metadata: models.Metadata{
			RequestId: requestID,
			Timestamp: timestamp,
		},
	}

	if data, ok := errors.(*multierror.Error); ok {
		response.Errors = data.Errors
	}

	return ctx.JSON(response.Code, response)
}
