package models

import "errors"

// The repository returns these, and services match them with errors.Is.
var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
)
