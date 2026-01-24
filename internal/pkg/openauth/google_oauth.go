package openauth

import (
	"context"
	"encoding/json"
	"errors"
	"go-echo-boilerplate/internal/config"
	"io"

	"golang.org/x/oauth2"
)

type Google interface {
	Redirect() string
	Fetch(state, code string) (*GoogleUser, error)
}

type GoogleOAuth struct {
	authState    string
	clientId     string
	clientSecret string
	redirectUrl  string
}

var oauth = &oauth2.Config{}

func InitializeGoogleOAuth(config *config.Configuration) (*GoogleOAuth, error) {
	if config.Google.State == "" {
		return nil, errors.New("empty signing key")
	}

	return &GoogleOAuth{
		authState:    config.Google.State,
		clientId:     config.Google.ClientID,
		clientSecret: config.Google.ClientSecret,
		redirectUrl:  config.Google.RedirectURI,
	}, nil
}

func (g *GoogleOAuth) Redirect() string {
	oauth = &oauth2.Config{
		ClientID:     g.clientId,
		ClientSecret: g.clientSecret,
		RedirectURL:  g.redirectUrl,
		Scopes:       []string{"profile", "email"},
	}

	url := oauth.AuthCodeURL(g.authState)
	return url
}

func (g *GoogleOAuth) Fetch(config *config.Configuration, state string, code string) (*GoogleUser, error) {
	if state != g.authState {
		return nil, errors.New("state are not the same")
	}

	token, err := oauth.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}

	client := oauth.Client(context.Background(), token)
	response, err := client.Get(config.Google.UserInfoURL)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	byteData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	err = json.Unmarshal(byteData, &data)
	if err != nil {
		return nil, err
	}

	user := &GoogleUser{
		Email: data["email"].(string),
		Name:  data["name"].(string),
	}

	return user, nil
}
