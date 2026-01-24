package errorc

import (
	"go-echo-boilerplate/internal/models"
	"net/http"
)

func wrap(resp models.ErrorResponse) *HTTPError {
	return &HTTPError{Response: resp, Err: nil}
}

// ==== Predefined ErrorResponses ====
var (
	ErrorUserNotFound   = wrap(models.ErrorResponse{Code: http.StatusNotFound, Status: "DATA_NOT_FOUND", Message: "user not found"})
	ErrorUnauthorized   = wrap(models.ErrorResponse{Code: http.StatusUnauthorized, Status: "UNAUTHORIZED", Message: "unauthorized"})
	ErrorForbidden      = wrap(models.ErrorResponse{Code: http.StatusForbidden, Status: "FORBIDDEN", Message: "forbidden"})
	ErrorEmailExists    = wrap(models.ErrorResponse{Code: http.StatusConflict, Status: "CONFLICT", Message: "email already exists"})
	ErrorTokenExpired   = wrap(models.ErrorResponse{Code: http.StatusUnauthorized, Status: "TOKEN_EXPIRED", Message: "token expired"})
	ErrorAlreadyExist   = wrap(models.ErrorResponse{Code: http.StatusConflict, Status: "ALREADY_EXISTS", Message: "resource already exists"})
	ErrorDataNotFound   = wrap(models.ErrorResponse{Code: http.StatusNotFound, Status: "DATA_NOT_FOUND", Message: "data not found"})
	ErrorInvalidInput   = wrap(models.ErrorResponse{Code: http.StatusBadRequest, Status: "BAD_REQUEST", Message: "invalid input"})
	ErrorInvalidData    = wrap(models.ErrorResponse{Code: http.StatusBadRequest, Status: "INVALID_DATA", Message: "invalid data"})
	ErrorForbiddenRole  = wrap(models.ErrorResponse{Code: http.StatusForbidden, Status: "FORBIDDEN_ROLE", Message: "you are not allowed to access this feature"})
	ErrorInternalServer = wrap(models.ErrorResponse{Code: http.StatusInternalServerError, Status: "INTERNAL_SERVER_ERROR", Message: "Unknown server error occurred."})
	ErrorValidation     = wrap(models.ErrorResponse{Code: http.StatusBadRequest, Status: "VALIDATION_ERROR", Message: "Validation failed for one or more fields."})
	ErrorDatabase       = wrap(models.ErrorResponse{Code: http.StatusInternalServerError, Status: "DATABASE_ERROR", Message: "Database error occurred."})
)
