// Package errorc (error customized) is the application's error catalog: one
// immutable herr.Class per stable error code. Services and middleware stamp
// instances with errorc.X.New() and never construct ad-hoc error responses.
package errorc

import "github.com/jeremygprawira/herr"

var (
	UserNotFound  = herr.Define(herr.Class{Code: "USER_NOT_FOUND", Kind: herr.KindNotFound, Public: herr.Message("user not found")})
	DataNotFound  = herr.Define(herr.Class{Code: "DATA_NOT_FOUND", Kind: herr.KindNotFound, Public: herr.Message("data not found")})
	InvalidInput  = herr.Define(herr.Class{Code: "INVALID_INPUT", Kind: herr.KindInvalid, Public: herr.Message("invalid input")})
	InvalidData   = herr.Define(herr.Class{Code: "INVALID_DATA", Kind: herr.KindInvalid, Public: herr.Message("invalid data")})
	Unauthorized  = herr.Define(herr.Class{Code: "UNAUTHORIZED", Kind: herr.KindUnauthorized, Public: herr.Message("unauthorized")})
	TokenExpired  = herr.Define(herr.Class{Code: "TOKEN_EXPIRED", Kind: herr.KindUnauthorized, Public: herr.Message("token expired")})
	Forbidden     = herr.Define(herr.Class{Code: "FORBIDDEN", Kind: herr.KindForbidden, Public: herr.Message("forbidden")})
	ForbiddenRole = herr.Define(herr.Class{Code: "FORBIDDEN_ROLE", Kind: herr.KindForbidden, Public: herr.Message("you are not allowed to access this feature")})
	EmailExists   = herr.Define(herr.Class{Code: "EMAIL_EXISTS", Kind: herr.KindConflict, Public: herr.Message("email already exists")})
	AlreadyExists = herr.Define(herr.Class{Code: "ALREADY_EXISTS", Kind: herr.KindConflict, Public: herr.Message("resource already exists")})
	Validation    = herr.Define(herr.Class{Code: "VALIDATION_FAILED", Kind: herr.KindUnprocessable, Public: herr.Message("Validation failed for one or more fields.")})
	Internal      = herr.Define(herr.Class{Code: "INTERNAL_SERVER_ERROR", Kind: herr.KindInternal, Public: herr.Message("Unknown server error occurred.")})
	Database      = herr.Define(herr.Class{Code: "DATABASE_ERROR", Kind: herr.KindInternal, Public: herr.Message("Database error occurred.")})
)
