package jwtc_test

import (
	"go-echo-boilerplate/internal/constant"
	"go-echo-boilerplate/internal/pkg/jwtc"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveRefreshToken(t *testing.T) {
	withCookie := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.AddCookie(&http.Cookie{Name: constant.RefreshTokenCookieName, Value: "from-cookie"})
		return r
	}

	assert.Equal(t, "from-body", jwtc.ResolveRefreshToken(withCookie(), "from-body"))
	assert.Equal(t, "from-cookie", jwtc.ResolveRefreshToken(withCookie(), ""))
	assert.Equal(t, "", jwtc.ResolveRefreshToken(httptest.NewRequest(http.MethodPost, "/", nil), ""))
}
