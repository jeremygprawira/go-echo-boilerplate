package models

import "fmt"

type (
	// PaginationOutput is the cursor-pagination response shape: the server
	// hands back the exact cursor to request the next page with, so clients
	// never compute pagination state themselves.
	PaginationOutput struct {
		NextCursor string `json:"nextCursor,omitempty"`
		HasMore    bool   `json:"hasMore"`
		Limit      int    `json:"limit"`
	}

	// Response is the single envelope shape for every response, success or
	// error. Code is always the HTTP status (int); a failed request is
	// distinguished by Status == "ERROR" and carries the stable catalog
	// error identifier in ErrorCode (e.g. "VALIDATION_FAILED") — Code never
	// changes type depending on outcome.
	Response struct {
		Code       int               `json:"code" example:"200"`
		Status     string            `json:"status" example:"OK"`
		Message    string            `json:"message" example:"Request has been successfully processed."`
		ErrorCode  string            `json:"errorCode,omitempty" example:"VALIDATION_FAILED"`
		Data       interface{}       `json:"data,omitempty"`
		Pagination *PaginationOutput `json:"pagination,omitempty"`
		Errors     interface{}       `json:"errors,omitempty"`
		Metadata   Metadata          `json:"metadata"`
	}
	Metadata struct {
		RequestId string `json:"requestId"`
		Timestamp string `json:"timestamp"`
		TotalRows int    `json:"totalRows,omitempty"`
		// Retryable/RetryAfter/TraceId are only ever populated on error
		// responses, carried over from herr's wire body.
		Retryable  *bool  `json:"retryable,omitempty"`
		RetryAfter int    `json:"retryAfter,omitempty"`
		TraceId    string `json:"traceId,omitempty"`
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
