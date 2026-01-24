package models

import "fmt"

type (
	PaginationOutput struct {
		Prev  string
		Next  string
		Total int
		Limit int
	}
	ErrorResponse struct {
		Code     int         `json:"code"`
		Status   string      `json:"status"`
		Message  string      `json:"message"`
		Errors   interface{} `json:"errors,omitempty"`
		Metadata Metadata    `json:"metadata"`
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
