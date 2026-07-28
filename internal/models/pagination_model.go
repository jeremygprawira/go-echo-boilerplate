package models

import "strconv"

const (
	defaultLimit = 20
	maxLimit     = 100
)

// PaginationInput is the normalized, validated pagination request.
type PaginationInput struct {
	Limit  int
	Offset int
}

// ParsePagination converts raw query strings into a safe PaginationInput,
// applying defaults and an upper bound so clients cannot request unbounded pages.
func ParsePagination(limitStr, offsetStr string) PaginationInput {
	limit := defaultLimit
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if v, err := strconv.Atoi(offsetStr); err == nil && v > 0 {
		offset = v
	}

	return PaginationInput{Limit: limit, Offset: offset}
}
