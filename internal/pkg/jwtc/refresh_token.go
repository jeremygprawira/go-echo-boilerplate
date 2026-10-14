package jwtc

import (
	"go-echo-boilerplate/internal/constant"
	"net/http"
)

// ResolveRefreshToken applies one precedence rule wherever a refresh token may
// arrive: an explicit body value wins, otherwise the refresh token cookie set
// at login. Every endpoint that accepts a refresh token uses it, so a client
// gets the same behavior from each.
func ResolveRefreshToken(r *http.Request, bodyToken string) string {
	if bodyToken != "" {
		return bodyToken
	}
	if cookie, err := r.Cookie(constant.RefreshTokenCookieName); err == nil {
		return cookie.Value
	}
	return ""
}
