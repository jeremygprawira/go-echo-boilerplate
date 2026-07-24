// Package apperr is the application's error catalog: one immutable herr.Class
// per stable error code. Services and middleware stamp instances with
// apperr.X.New() and never construct ad-hoc error responses.
package apperr

import "github.com/jeremygprawira/herr"

var (
	UserNotFound  = herr.Define(herr.Class{Code: "USER_NOT_FOUND", Kind: herr.KindNotFound, Public: herr.Msg("user not found")})
	DataNotFound  = herr.Define(herr.Class{Code: "DATA_NOT_FOUND", Kind: herr.KindNotFound, Public: herr.Msg("data not found")})
	InvalidInput  = herr.Define(herr.Class{Code: "INVALID_INPUT", Kind: herr.KindInvalid, Public: herr.Msg("invalid input")})
	InvalidData   = herr.Define(herr.Class{Code: "INVALID_DATA", Kind: herr.KindInvalid, Public: herr.Msg("invalid data")})
	Unauthorized  = herr.Define(herr.Class{Code: "UNAUTHORIZED", Kind: herr.KindUnauthorized, Public: herr.Msg("unauthorized")})
	TokenExpired  = herr.Define(herr.Class{Code: "TOKEN_EXPIRED", Kind: herr.KindUnauthorized, Public: herr.Msg("token expired")})
	Forbidden     = herr.Define(herr.Class{Code: "FORBIDDEN", Kind: herr.KindForbidden, Public: herr.Msg("forbidden")})
	ForbiddenRole = herr.Define(herr.Class{Code: "FORBIDDEN_ROLE", Kind: herr.KindForbidden, Public: herr.Msg("you are not allowed to access this feature")})
	EmailExists   = herr.Define(herr.Class{Code: "EMAIL_EXISTS", Kind: herr.KindConflict, Public: herr.Msg("email already exists")})
	AlreadyExists = herr.Define(herr.Class{Code: "ALREADY_EXISTS", Kind: herr.KindConflict, Public: herr.Msg("resource already exists")})
	Validation    = herr.Define(herr.Class{Code: "VALIDATION_FAILED", Kind: herr.KindUnprocessable, Public: herr.Msg("Validation failed for one or more fields.")})
	Internal      = herr.Define(herr.Class{Code: "INTERNAL_SERVER_ERROR", Kind: herr.KindInternal, Public: herr.Msg("Unknown server error occurred.")})
	Database      = herr.Define(herr.Class{Code: "DATABASE_ERROR", Kind: herr.KindInternal, Public: herr.Msg("Database error occurred.")})
)
