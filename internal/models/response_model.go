package models

import "fmt"

type (
	PaginationOutput struct {
		Prev  string
		Next  string
		Total int
		Limit int
	}

	Response struct {
		Code       int               `json:"code" example:"200"`
		Status     string            `json:"status" example:"OK"`
		Message    string            `json:"message" example:"Request has been successfully processed."`
		Data       interface{}       `json:"data,omitempty"`
		Pagination *PaginationOutput `json:"pagination,omitempty"`
		Errors     interface{}       `json:"errors,omitempty"`
		Metadata   Metadata          `json:"metadata"`
	}
	Metadata struct {
		RequestId string `json:"requestId"`
		Timestamp string `json:"timestamp"`
		TotalRows int    `json:"totalRows,omitempty"`
	}
)

type ErrorValidationResponse struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ErrorValidationResponse) Error() string {
	return fmt.Sprintf("code: %s, field: %s, message: %s", e.Code, e.Field, e.Message)
}

// ErrorWireResponse documents herr's error wire body for Swagger only.
// The actual body is rendered by herr (internal/deliveries/http/error_handler.go);
// this struct must mirror it, never be constructed at runtime.
type ErrorWireResponse struct {
	Code     string                    `json:"code" example:"USER_NOT_FOUND"`
	Message  string                    `json:"message" example:"user not found"`
	Errors   []ErrorValidationResponse `json:"errors,omitempty"`
	Metadata map[string]interface{}    `json:"metadata,omitempty"`
}
