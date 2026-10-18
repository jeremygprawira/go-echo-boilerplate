// Package openauth is optional Google OAuth sign-in. Nothing wires it by
// default. To use it, build a GoogleOAuth in core.Setup and call it from a
// handler pair: one that redirects, and one that serves redirect_uri.
package openauth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"go-echo-boilerplate/internal/config"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

// Google is the sign-in flow. The caller makes a random state for each login
// (generator.SessionToken works), keeps it in a cookie, and hands both that
// state and the one on the callback to Fetch.
type Google interface {
	Redirect(state string) string
	Fetch(ctx context.Context, wantState, gotState, code string) (*GoogleUser, error)
}

var _ Google = (*GoogleOAuth)(nil)

type GoogleOAuth struct {
	config      oauth2.Config
	userInfoURL string
}

func NewGoogleOAuth(cfg config.Google) (*GoogleOAuth, error) {
	return newGoogleOAuth(cfg, endpoints.Google)
}

func newGoogleOAuth(cfg config.Google, endpoint oauth2.Endpoint) (*GoogleOAuth, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" || cfg.UserInfoURL == "" {
		return nil, errors.New("google oauth: client_id, client_secret, redirect_uri and user_info_url are required")
	}

	return &GoogleOAuth{
		config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURI,
			Endpoint:     endpoint,
			Scopes:       []string{"profile", "email"},
		},
		userInfoURL: cfg.UserInfoURL,
	}, nil
}

// Redirect returns the Google consent URL that carries state.
func (g *GoogleOAuth) Redirect(state string) string {
	return g.config.AuthCodeURL(state)
}

// Fetch checks the state, trades code for a token, and reads the user profile.
func (g *GoogleOAuth) Fetch(ctx context.Context, wantState, gotState, code string) (*GoogleUser, error) {
	if wantState == "" || subtle.ConstantTimeCompare([]byte(wantState), []byte(gotState)) != 1 {
		return nil, errors.New("google oauth: state mismatch")
	}

	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google oauth: exchange code: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("google oauth: build userinfo request: %w", err)
	}
	resp, err := g.config.Client(ctx, token).Do(req)
	if err != nil {
		return nil, fmt.Errorf("google oauth: userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google oauth: userinfo status %d", resp.StatusCode)
	}

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("google oauth: decode userinfo: %w", err)
	}
	if user.Email == "" {
		return nil, errors.New("google oauth: userinfo has no email")
	}
	return &user, nil
}
