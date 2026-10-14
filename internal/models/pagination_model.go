package models

import "strconv"

const (
	defaultLimit = 20
	maxLimit     = 100
)

// PaginationInput is the normalized, validated pagination request. Cursor
// pagination is used instead of offset/limit so listing large tables doesn't
// degrade as the page number grows — the repository layer is expected to
// query "WHERE id > cursor ORDER BY id LIMIT limit" (or the equivalent for
// its sort key) rather than skipping rows.
type PaginationInput struct {
	// Cursor is the opaque value of the last item's sort key from the
	// previous page (e.g. its ID), or "" for the first page. It is passed
	// through as-is; the repository is responsible for parsing/validating it
	// against its actual key type.
	Cursor string
	Limit  int
}

// ParsePagination converts raw query strings into a safe PaginationInput,
// applying a default and an upper bound on limit so clients cannot request
// unbounded pages.
func ParsePagination(cursor, limitStr string) PaginationInput {
	limit := defaultLimit
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return PaginationInput{Cursor: cursor, Limit: limit}
}
