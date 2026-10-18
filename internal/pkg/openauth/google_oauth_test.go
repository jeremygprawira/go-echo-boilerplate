package openauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go-echo-boilerplate/internal/config"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type fakeGoogle struct {
	*httptest.Server
	tokenCalls    int
	userInfoBody  string
	userInfoCode  int
	gotAuthHeader string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{
		userInfoCode: http.StatusOK,
		userInfoBody: `{"id":"42","email":"ann@example.com","verified_email":true,"name":"Ann Lee"}`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenCalls++
		if err := r.ParseForm(); err != nil || r.Form.Get("code") != "good-code" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-123","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		f.gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(f.userInfoCode)
		_, _ = w.Write([]byte(f.userInfoBody))
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeGoogle) client(t *testing.T) *GoogleOAuth {
	t.Helper()
	g, err := newGoogleOAuth(config.Google{
		ClientID:     "client-1",
		ClientSecret: "secret-1",
		RedirectURI:  "https://app.example.com/callback",
		UserInfoURL:  f.URL + "/userinfo",
	}, oauth2.Endpoint{AuthURL: f.URL + "/auth", TokenURL: f.URL + "/token"})
	require.NoError(t, err)
	return g
}

func TestNewGoogleOAuth_RequiresCredentials(t *testing.T) {
	_, err := NewGoogleOAuth(config.Google{ClientID: "client-1"})
	require.Error(t, err)
}

func TestGoogleOAuth_AuthURL(t *testing.T) {
	g := newFakeGoogle(t).client(t)

	u, err := url.Parse(g.AuthURL("state-abc"))
	require.NoError(t, err)
	require.Equal(t, "/auth", u.Path)
	q := u.Query()
	require.Equal(t, "state-abc", q.Get("state"))
	require.Equal(t, "client-1", q.Get("client_id"))
	require.Equal(t, "https://app.example.com/callback", q.Get("redirect_uri"))
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, "profile email", q.Get("scope"))
}

func TestGoogleOAuth_AuthURLUsesGoogleByDefault(t *testing.T) {
	g, err := NewGoogleOAuth(config.Google{ClientID: "c", ClientSecret: "s", RedirectURI: "https://app.example.com/cb", UserInfoURL: "https://example.com/u"})
	require.NoError(t, err)

	u, err := url.Parse(g.AuthURL("s1"))
	require.NoError(t, err)
	require.Equal(t, "accounts.google.com", u.Host)
}

func TestGoogleOAuth_Callback(t *testing.T) {
	f := newFakeGoogle(t)

	user, err := f.client(t).Callback(context.Background(), "state-abc", "state-abc", "good-code")
	require.NoError(t, err)
	require.Equal(t, &GoogleUser{Id: "42", Email: "ann@example.com", Verified: true, Name: "Ann Lee"}, user)
	require.Equal(t, "Bearer access-123", f.gotAuthHeader)
}

func TestGoogleOAuth_CallbackRejectsStateMismatch(t *testing.T) {
	for _, tc := range []struct{ name, want, got string }{
		{"different", "state-abc", "state-xyz"},
		{"empty", "", ""},
		{"got empty", "state-abc", ""},
		{"got has extra characters", "state-abc", "state-abcd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGoogle(t)
			_, err := f.client(t).Callback(context.Background(), tc.want, tc.got, "good-code")
			require.Error(t, err)
			require.Zero(t, f.tokenCalls, "a bad state must not reach the token endpoint")
		})
	}
}

func TestGoogleOAuth_CallbackFails(t *testing.T) {
	cases := []struct {
		name string
		code string
		body string
		resp int
	}{
		{"code exchange rejected", "bad-code", "", http.StatusOK},
		{"userinfo error status with a valid profile", "good-code", "", http.StatusInternalServerError},
		{"userinfo not json", "good-code", `<html>`, http.StatusOK},
		{"userinfo wrong types", "good-code", `{"email":5,"name":true}`, http.StatusOK},
		{"userinfo without email", "good-code", `{"id":"42","name":"Ann Lee"}`, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGoogle(t)
			if tc.body != "" {
				f.userInfoBody = tc.body
			}
			f.userInfoCode = tc.resp

			user, err := f.client(t).Callback(context.Background(), "s", "s", tc.code)
			require.Error(t, err)
			require.Nil(t, user)
		})
	}
}
